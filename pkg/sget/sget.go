//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sget

import (
	"context"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
)

func New(image, key string, out io.Writer) *SecureGet {
	return &SecureGet{
		ImageRef: image,
		KeyRef:   key,
		Out:      out,
	}
}

type SecureGet struct {
	ImageRef string
	KeyRef   string
	Out      io.Writer
}

func (sg *SecureGet) Do(ctx context.Context) error {
	// Ref must be specified by digest.
	ref, err := name.NewDigest(sg.ImageRef)
	if err != nil {
		return err
	}

	// TODO: Discover any signatures and verify them.

	img, err := remote.Image(ref,
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithContext(ctx))
	if err != nil {
		return err
	}
	layers, err := img.Layers()
	if err != nil {
		return err
	}
	if len(layers) != 1 {
		return errors.New("invalid artifact")
	}
	rc, err := layers[0].Compressed()
	if err != nil {
		return err
	}

	_, err = io.Copy(sg.Out, rc)
	return err
}
