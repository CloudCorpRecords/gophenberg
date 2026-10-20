// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/signature"
	"github.com/gopherium/gophenberg/internal/update"
)

// newSigner returns a fresh Ed25519 key, like the release key or the standby.
func newSigner(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("making an Ed25519 key: %v", err)
	}
	return key
}

// artifactFor returns the release record entry of body served at address, signed by key.
func artifactFor(t *testing.T, body []byte, address string, key ed25519.PrivateKey) update.Artifact {
	t.Helper()
	sum, err := signature.Checksum(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("taking the checksum: %v", err)
	}
	digest, err := signature.Digest(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("taking the digest: %v", err)
	}
	return update.Artifact{
		ID: "linux-amd64", Platform: "linux/amd64", ContentType: "application/gzip", URL: address,
		Checksum: sum, Signature: signature.Sign(key, digest),
	}
}

// wantRefused fails t unless err is an error carrying text.
func wantRefused(t *testing.T, err error, text string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Errorf("error = %v, want it refused saying %q", err, text)
	}
}

// wantMissing fails t unless nothing sits at path.
func wantMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s is still there (%v), want it deleted", path, err)
	}
}
