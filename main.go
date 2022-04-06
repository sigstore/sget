package main

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	neturl "net/url"
	"os"
	"strings"

	fapi "github.com/sigstore/fulcio/pkg/api"
	rekor "github.com/sigstore/rekor/pkg/generated/client"
	rentries "github.com/sigstore/rekor/pkg/generated/client/entries"
	rindex "github.com/sigstore/rekor/pkg/generated/client/index"
	rmodels "github.com/sigstore/rekor/pkg/generated/models"
	"github.com/spf13/cobra"
)

func main() {
	var flags rootFlags
	root := &cobra.Command{
		Use:           "sget URL",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			u, err := neturl.Parse(url)
			if err != nil {
				return fmt.Errorf("parsing URL: %w", err)
			}
			if u.Scheme != "https" {
				log.Println("URL is not HTTPS, assuming it's an OCI image reference by digest")
				return fetchImage(url)
			}

			// Validate digest if specified.
			tmp, err := fetch(url, false)
			if err != nil {
				return fmt.Errorf("error getting digest for %q: %w", url, err)
			}
			defer os.Remove(tmp.f.Name())
			gotDigest := tmp.digest
			if flags.wantDigest != "" && flags.wantDigest != gotDigest {
				return fmt.Errorf("digest mismatch; got %q, want %q", gotDigest, flags.wantDigest)
			}

			// Get Fulcio root cert.
			fulcioServer, err := neturl.Parse(flags.fulcioURL)
			if err != nil {
				return fmt.Errorf("creating Fulcio client: %w", err)
			}
			fclient := fapi.NewClient(fulcioServer, fapi.WithTimeout(flags.fulcioTimeout))
			fresp, err := fclient.RootCert()
			if err != nil {
				return fmt.Errorf("getting signing cert: %w", err)
			}
			fulcioRoot := x509.NewCertPool()
			if !fulcioRoot.AppendCertsFromPEM(fresp.ChainPEM) {
				return fmt.Errorf("failed appending Fulcio root cert")
			}

			// Find entries for url + digest
			rclient := rekor.NewHTTPClient(nil)
			iparams := rindex.NewSearchIndexParams()
			iparams.SetTimeout(flags.rekorTimeout)
			iparams.SetQuery(&rmodels.SearchIndex{Hash: "sha256:" + tmp.digest})
			iresp, err := rclient.Index.SearchIndex(iparams)
			if err != nil {
				return fmt.Errorf("querying Rekor entries: %w", err)
			}
			if len(iresp.Payload) == 0 {
				return fmt.Errorf("found no Rekor entries for URL: %s", url)
			}
			identities := set{}
			for _, e := range iresp.Payload {
				gparams := rentries.NewGetLogEntryByUUIDParams()
				gparams.SetTimeout(flags.rekorTimeout)
				gparams.SetEntryUUID(e)
				gresp, err := rclient.Entries.GetLogEntryByUUID(gparams)
				if err != nil {
					return fmt.Errorf("getting Rekor entry %q: %w", e, err)
				}
				le := gresp.Payload[e]
				leb, err := base64.StdEncoding.DecodeString(le.Body.(string))
				if err != nil {
					return fmt.Errorf("decoding Rekor LogEntry body: %w", err)
				}
				var ent struct {
					Spec struct {
						PublicKey []byte
					}
				}
				if err := json.Unmarshal(leb, &ent); err != nil {
					return fmt.Errorf("unmarshaling Rekor LogEntry body: %w", err)
				}

				// TODO: Check that the URL matches, not just the digest.

				block, _ := pem.Decode([]byte(ent.Spec.PublicKey))
				if block == nil {
					return fmt.Errorf("parsing certificate PEM; block is nil")
				}
				cert, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					return fmt.Errorf("parsing certificat: %w", err)
				}

				// Verify cert is from Fulcio.
				if _, err := cert.Verify(x509.VerifyOptions{
					// THIS IS IMPORTANT: WE DO NOT CHECK TIMES HERE
					// THE CERTIFICATE IS TREATED AS TRUSTED FOREVER
					// WE CHECK THAT THE SIGNATURES WERE CREATED DURING THIS WINDOW
					CurrentTime: cert.NotBefore,
					Roots:       fulcioRoot,
					KeyUsages: []x509.ExtKeyUsage{
						x509.ExtKeyUsageCodeSigning,
					},
				}); err != nil {
					return fmt.Errorf("checking cert against Fulcio root: %w", err)
				}

				if len(cert.EmailAddresses) != 1 {
					log.Printf("saw unexpected number of identities for %q: %s", e, cert.EmailAddresses)
				}
				for _, email := range cert.EmailAddresses {
					identities.add(email)
				}
			}

			// Collect trusted identities.
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("loading config file: %w", err)
			}
			trust := set{}
			for _, i := range cfg.Identities {
				trust.add(i)
			}
			if h, ok := cfg.Hosts[u.Host]; ok {
				for _, i := range h.Identities {
					trust.add(i)
				}
			}

			log.Printf("Found %d identities who have signed for %s", len(identities), url)
			log.Println("Signing identities:", identities) // TODO: remove
			match := trust.intersect(identities)
			if len(match) == 0 {
				return fmt.Errorf("found no trusted identities for %s", url)
			}

			log.Println("Found trusted identities:", match)

			var outw io.WriteCloser
			switch flags.out {
			case "-":
				outw = os.Stdout
			default:
				outw, err = os.Create(flags.out)
				if err != nil {
					return fmt.Errorf("creating %q: %w", flags.out, err)
				}
			}
			defer outw.Close()

			// Write the contents of the temp file to stdout.
			_, err = io.Copy(outw, tmp.out())
			return err
		},
	}
	flags.addFlags(root)

	addSign(root)
	addTrust(root)

	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

type set map[string]struct{}

func (s set) add(n string) { s[n] = struct{}{} }

func (s set) intersect(o set) set {
	out := set{}
	for k := range s {
		if _, ok := o[k]; ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func (s set) String() string {
	var sb strings.Builder
	first := true
	for k := range s {
		if !first {
			sb.WriteRune(' ')
		}
		sb.WriteString(k)
		first = false
	}
	return sb.String()
}

type rootFlags struct {
	rekorFlags
	fulcioFlags
	wantDigest string
	out        string
}

func (f *rootFlags) addFlags(cmd *cobra.Command) {
	f.rekorFlags.addFlags(cmd)
	f.fulcioFlags.addFlags(cmd)
	cmd.Flags().StringVar(&f.wantDigest, "digest", "", "If set, URL must have the given digest")
	cmd.Flags().StringVarP(&f.out, "out", "o", "-", `File path to write to (for stdout pass "-")`)
}
