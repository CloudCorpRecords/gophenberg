// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/signature"
	"github.com/gopherium/gophenberg/internal/update"
)

// now is the moment every test reads the files at.
var now = time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)

// keys holds the keys one test signs with: three roots, one a stranger, and four Ed25519 keys, one a stranger.
type keys struct {
	root1, root2, strangerRoot            *ecdsa.PrivateKey
	release, standby, timestamp, stranger ed25519.PrivateKey
}

// newKeys returns fresh keys for one test.
func newKeys(t *testing.T) keys {
	t.Helper()
	return keys{
		root1: newRoot(t), root2: newRoot(t), strangerRoot: newRoot(t),
		release: newSigner(t), standby: newSigner(t), timestamp: newSigner(t), stranger: newSigner(t),
	}
}

// newRoot returns a fresh P-256 root key.
func newRoot(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("making a root key: %v", err)
	}
	return key
}

// newSigner returns a fresh Ed25519 release key.
func newSigner(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("making a release key: %v", err)
	}
	return key
}

// multikey returns key as a Multikey, failing the test when it cannot.
func multikey(t *testing.T, key any) string {
	t.Helper()
	text, err := signature.Multikey(key)
	if err != nil {
		t.Fatalf("writing a Multikey: %v", err)
	}
	return text
}

// trusted returns the public keys of the two roots a site trusts.
func (k keys) trusted() []*ecdsa.PublicKey {
	return []*ecdsa.PublicKey{&k.root1.PublicKey, &k.root2.PublicKey}
}

// keyList returns a key list naming the release key, the standby and the timestamp key, valid for a year from now.
func (k keys) keyList(t *testing.T) update.KeyList {
	t.Helper()
	return update.KeyList{
		Schema: 1, Serial: 5, Expires: now.AddDate(1, 0, 0),
		Keys: []update.ListedKey{
			{Role: update.RoleRelease, Multikey: multikey(t, k.release.Public())},
			{Role: update.RoleStandby, Multikey: multikey(t, k.standby.Public())},
			{Role: update.RoleTimestamp, Multikey: multikey(t, k.timestamp.Public())},
		},
	}
}

// marshal returns v as JSON, failing the test when it cannot.
func marshal(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("writing %T: %v", v, err)
	}
	return body
}

// signLines returns the signature file of body signed by each root.
func signLines(t *testing.T, body []byte, roots ...*ecdsa.PrivateKey) []byte {
	t.Helper()
	lines := make([]string, 0, len(roots))
	for _, root := range roots {
		line, err := update.SignKeyList(body, root)
		if err != nil {
			t.Fatalf("signing the key list: %v", err)
		}
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// release returns the release record of version, as the release workflow writes it.
func release(version string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"version":%q,"artifacts":{"package":[]}}`, version))
}

// offer returns an offer of version to every site, minimum version minimum, on PostgreSQL 15.
func offer(version, minimum string) update.Offer {
	return update.Offer{
		Rollout: 100, MinimumVersion: minimum, PostgreSQL: 15, Kits: []string{"0.9.0"},
		Notes: "https://example.com/releases/" + version, Release: release(version),
	}
}

// index returns a stable index of the offers, valid for thirty days from now.
func index(offers ...update.Offer) update.Index {
	return update.Index{Schema: 1, Channel: "stable", Serial: 9, Expires: now.AddDate(0, 0, 30), Offers: offers}
}

// signIndex returns the signature file of body signed by key.
func signIndex(body []byte, key ed25519.PrivateKey) []byte {
	return []byte(update.SignIndex(body, key) + "\n")
}

// timestamp returns a timestamp file valid for seven days from now, naming the key list 5 and the stable index 9.
func timestamp() update.Timestamp {
	return update.Timestamp{Schema: 1, Expires: now.AddDate(0, 0, 7), Keys: 5, Indexes: map[string]int64{"stable": 9}}
}

// signedTimestamp returns stamp as JSON and its signature file under key.
func signedTimestamp(t *testing.T, stamp update.Timestamp, key ed25519.PrivateKey) ([]byte, []byte) {
	t.Helper()
	body := marshal(t, stamp)
	return body, signIndex(body, key)
}

// wantRefused fails t unless err refuses with text.
func wantRefused(t *testing.T, err error, text string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Errorf("error = %v, want it refused saying %q", err, text)
	}
}
