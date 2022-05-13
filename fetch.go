package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
)

type tempFile struct {
	f      *os.File
	digest string
}

func (f tempFile) out() io.ReadCloser {
	f.f.Seek(0, 0)
	return f.f
}

func fetch(url string, discard bool) (*tempFile, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %d %s", url, resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()

	tf := &tempFile{}

	var tmp io.Writer
	if discard {
		tmp = io.Discard
	} else {
		tmp, err = os.CreateTemp("", "sget-*")
		if err != nil {
			return nil, fmt.Errorf("making temp file: %w", err)
		}
		tf.f = tmp.(*os.File)
	}

	h := sha256.New()
	if _, err := io.Copy(tmp, io.TeeReader(resp.Body, h)); err != nil {
		return nil, fmt.Errorf("reading response: %v", err)
	}
	tf.digest = fmt.Sprintf("%x", h.Sum(nil))
	return tf, nil
}
