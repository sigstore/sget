package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	neturl "net/url"
	"os"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/in-toto/in-toto-golang/in_toto"
	fapi "github.com/sigstore/fulcio/pkg/api"
	rekor "github.com/sigstore/rekor/pkg/generated/client"
	rentries "github.com/sigstore/rekor/pkg/generated/client/entries"
	rmodels "github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/sigstore/pkg/oauthflow"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/dsse"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func addSign(cmd *cobra.Command) {
	var flags signFlags
	sign := &cobra.Command{
		Use:           "sign URL",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			url := args[0]
			u, err := neturl.Parse(url)
			if err != nil {
				return fmt.Errorf("parsing URL: %w", err)
			}
			if u.Scheme != "https" {
				return fmt.Errorf("URL must be HTTPS")
			}

			// Validate digest if specified.
			tmp, err := fetch(ctx, url, true)
			if err != nil {
				return fmt.Errorf("error getting digest for %q: %w", url, err)
			}
			gotDigest := tmp.digest
			if flags.wantDigest != "" && flags.wantDigest != gotDigest {
				return fmt.Errorf("digest mismatch; got %q, want %q", gotDigest, flags.wantDigest)
			}
			log.Println("Fetched URL with digest:", gotDigest)

			// Do the OAuth dance and get an ID token.
			var flow oauthflow.TokenGetter
			switch {
			case flags.idtoken != "":
				flow = &oauthflow.StaticTokenGetter{RawToken: flags.idtoken}
			case !term.IsTerminal(0):
				fmt.Fprintln(os.Stderr, "Non-interactive mode detected, using device flow.")
				flow = oauthflow.NewDeviceFlowTokenGetter(
					flags.oidcIssuer, oauthflow.SigstoreDeviceURL, oauthflow.SigstoreTokenURL)
			default:
				flow = oauthflow.DefaultIDTokenGetter
			}
			idt, err := oauthflow.OIDConnect(flags.oidcIssuer, flags.oidcClientID, flags.oidcClientSecret, flags.oidcRedirectURL, flow)
			if err != nil {
				return fmt.Errorf("getting ID token: %w", err)
			}
			idtoken := idt.RawString
			log.Println("Got ID token! Signing as", idt.Subject)

			// Get signing cert from ephemeral private key and idtoken.
			priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return fmt.Errorf("generating ephemeral private key: %w", err)
			}
			pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
			if err != nil {
				return err
			}
			h := sha256.Sum256([]byte(idt.Subject))
			proof, err := ecdsa.SignASN1(rand.Reader, priv, h[:])
			if err != nil {
				return err
			}
			fulcioServer, err := neturl.Parse(flags.fulcioURL)
			if err != nil {
				return fmt.Errorf("creating Fulcio client: %w", err)
			}
			fclient := fapi.NewClient(fulcioServer)
			fresp, err := fclient.SigningCert(fapi.CertificateRequest{
				PublicKey: fapi.Key{
					Algorithm: "ecdsa",
					Content:   pubBytes,
				},
				SignedEmailAddress: proof,
			}, idtoken)
			if err != nil {
				return fmt.Errorf("getting signing cert: %w", err)
			}
			// TODO: Verify SCT. Do something with chain?
			log.Println("Got signing cert!")

			// Sign the message.
			s, err := signature.LoadECDSASigner(priv, crypto.SHA256)
			if err != nil {
				return fmt.Errorf("loading signer: %w", err)
			}
			ds := dsse.WrapSigner(s, "application/vnd.in-toto+json")
			msg, err := json.Marshal(in_toto.Statement{
				StatementHeader: in_toto.StatementHeader{
					Type: "sget-fetched",
					Subject: []in_toto.Subject{{
						Name:   url,
						Digest: map[string]string{"sha256": gotDigest},
					}},
				},
			})
			if err != nil {
				return fmt.Errorf("encoding message: %w", err)
			}
			signed, err := ds.SignMessage(bytes.NewReader(msg))
			if err != nil {
				return fmt.Errorf("signing: %w", err)
			}

			// Record url + digest.
			certPEMBase64 := strfmt.Base64(fresp.CertPEM)
			params := rentries.NewCreateLogEntryParams()
			params.SetTimeout(flags.fulcioTimeout)
			params.SetProposedEntry(&rmodels.Intoto{
				APIVersion: swag.String("0.0.1"),
				Spec: rmodels.IntotoV001Schema{
					Content: &rmodels.IntotoV001SchemaContent{
						Envelope: string(signed),
					},
					PublicKey: &certPEMBase64,
				},
			})
			created, err := rekor.NewHTTPClient(nil).Entries.CreateLogEntry(params)
			if err != nil {
				return fmt.Errorf("adding Rekor entry: %w", err)
			}
			log.Println("Rekor entry created!")

			le := created.Payload[created.ETag]
			log.Println("UUID:", created.ETag)
			log.Println("Integrated Time:", time.Unix(*le.IntegratedTime, 0).Format(time.RFC3339))
			log.Println("Log Index:", *le.LogIndex)
			leb, err := base64.StdEncoding.DecodeString(le.Body.(string))
			if err != nil {
				return fmt.Errorf("decoding Rekor LogEntry body: %w", err)
			}
			log.Println("Entry:", string(leb))
			return nil
		},
	}
	flags.addFlags(sign)

	cmd.AddCommand(sign)
}

type rekorFlags struct {
	rekorURL     string
	rekorTimeout time.Duration
}

func (f *rekorFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.rekorURL, "rekor-url", "https://rekor.sigstore.dev", "URL of the Rekor transparency log")
	cmd.Flags().DurationVar(&f.rekorTimeout, "rekor-timeout", 30*time.Second, "Timeout for requests to Rekor")
}

type fulcioFlags struct {
	fulcioURL     string
	fulcioTimeout time.Duration
}

func (f *fulcioFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.fulcioURL, "fulcio-url", "https://fulcio.sigstore.dev", "URL of the Fulcio CA")
	cmd.Flags().DurationVar(&f.fulcioTimeout, "fulcio-timeout", 30*time.Second, "Timeout for requests to Fulcio")
}

type oidcFlags struct {
	oidcIssuer       string
	oidcClientID     string
	oidcClientSecret string
	oidcRedirectURL  string
}

func (f *oidcFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.oidcIssuer, "oidc-issuer", "https://oauth2.sigstore.dev/auth", "OIDC issuer")
	cmd.Flags().StringVar(&f.oidcClientID, "oidc-client-id", "sigstore", "OIDC client ID")
	cmd.Flags().StringVar(&f.oidcClientSecret, "oidc-client-secret", "", "OIDC client secret")
	cmd.Flags().StringVar(&f.oidcRedirectURL, "oidc-redirect-url", "localhost:0/auth/callback", "OIDC redirect URL")

}

type signFlags struct {
	rekorFlags
	fulcioFlags
	oidcFlags
	wantDigest, idtoken string
}

func (f *signFlags) addFlags(cmd *cobra.Command) {
	f.rekorFlags.addFlags(cmd)
	f.fulcioFlags.addFlags(cmd)
	f.oidcFlags.addFlags(cmd)
	cmd.Flags().StringVar(&f.wantDigest, "digest", "", "If set, URL must have the given digest")
	cmd.Flags().StringVar(&f.idtoken, "idtoken", "", "If set, skip OIDC flow and use this ID token")
}
