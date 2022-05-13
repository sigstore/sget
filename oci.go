package main

import (
	"fmt"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func fetchImage(ref string) error {
	r, err := name.NewDigest(ref)
	if err != nil {
		return fmt.Errorf("parsing as OCI ref by digest: %w", err)
	}
	// TODO: look for a signature in Rekor?
	img, err := remote.Image(r, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return fmt.Errorf("fetching image: %w", err)
	}
	ls, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting layers: %w", err)
	}
	if len(ls) != 1 {
		return fmt.Errorf("image had %d layers, expected one", len(ls))
	}
	rc, err := ls[0].Compressed()
	if err != nil {
		return fmt.Errorf("getting layer data: %w", err)
	}
	defer rc.Close()
	if _, err := io.Copy(os.Stdout, rc); err != nil {
		return fmt.Errorf("reading layer data: %w", err)
	}
	return nil
}
