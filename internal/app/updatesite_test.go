// SPDX-License-Identifier: Apache-2.0

package app

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/signature"
	"github.com/gopherium/gophenberg/internal/update"
)

// updateSite is a loopback update site whose key list and stable index the test signs with keys it made.
type updateSite struct {
	t        *testing.T
	roots    map[string]*ecdsa.PrivateKey
	release  ed25519.PrivateKey
	stranger ed25519.PrivateKey
	offered  string
	mu       sync.Mutex
	files    map[string][]byte
	address  string
}

// newUpdateSite starts an update site on loopback, closed when t ends, offering nothing yet.
func newUpdateSite(t *testing.T) *updateSite {
	t.Helper()
	site := &updateSite{t: t, roots: map[string]*ecdsa.PrivateKey{}, files: map[string][]byte{}}
	for _, name := range []string{"the first root", "the second root", "a stranger"} {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("making the key of %s: %v", name, err)
		}
		site.roots[name] = key
	}
	site.release = newEd25519(t)
	site.stranger = newEd25519(t)
	server := httptest.NewServer(http.HandlerFunc(site.serve))
	t.Cleanup(server.Close)
	site.address = server.URL
	return site
}

// newEd25519 returns a fresh Ed25519 signing key.
func newEd25519(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("making an Ed25519 key: %v", err)
	}
	return key
}

// serve answers the file the request names, or 404.
func (s *updateSite) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	body, found := s.files[strings.TrimPrefix(r.URL.Path, "/")]
	s.mu.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(body)
}

// put serves body at path.
func (s *updateSite) put(path string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[path] = body
}

// trustedRoots returns the Multikeys of the two roots the site's settings trust.
func (s *updateSite) trustedRoots() string {
	return s.multikey(&s.roots["the first root"].PublicKey) + "," + s.multikey(&s.roots["the second root"].PublicKey)
}

// multikey returns key as a Multikey, failing the test when it cannot.
func (s *updateSite) multikey(key any) string {
	s.t.Helper()
	text, err := signature.Multikey(key)
	if err != nil {
		s.t.Fatalf("writing a Multikey: %v", err)
	}
	return text
}

// publishKeyList serves a key list naming the release key, signed by each root signers names.
func (s *updateSite) publishKeyList(signers ...string) {
	s.t.Helper()
	body := s.marshal(update.KeyList{
		Schema: 1, Serial: 1, Expires: time.Now().AddDate(1, 0, 0).UTC().Truncate(time.Second),
		Keys: []update.ListedKey{{Role: update.RoleRelease, Multikey: s.multikey(s.release.Public())}},
	})
	lines := make([]string, 0, len(signers))
	for _, name := range signers {
		line, err := update.SignKeyList(body, s.roots[name])
		if err != nil {
			s.t.Fatalf("signing the key list as %s: %v", name, err)
		}
		lines = append(lines, line)
	}
	s.put(update.KeyListPath, body)
	s.put(update.KeyListPath+update.SignatureSuffix, []byte(strings.Join(lines, "\n")+"\n"))
}

// publishIndex serves the index changed by edit, signed by key.
func (s *updateSite) publishIndex(key ed25519.PrivateKey, edit func(*update.Index)) {
	s.t.Helper()
	index := update.Index{
		Schema: 1, Channel: "stable", Serial: 2, Expires: time.Now().AddDate(0, 0, 30).UTC().Truncate(time.Second),
		Offers: []update.Offer{{
			Rollout: 100, MinimumVersion: s.offered, PostgreSQL: 15, Kits: []string{"0.9.0"},
			Notes:   "https://example.com/releases/" + s.offered,
			Release: json.RawMessage(fmt.Sprintf(`{"version":%q,"artifacts":{"package":[]}}`, s.offered)),
		}},
	}
	edit(&index)
	body := s.marshal(index)
	s.put(update.IndexPath("stable"), body)
	s.put(update.IndexPath("stable")+update.SignatureSuffix, []byte(update.SignIndex(body, key)+"\n"))
}

// marshal returns v as JSON, failing the test when it cannot.
func (s *updateSite) marshal(v any) []byte {
	s.t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		s.t.Fatalf("writing %T: %v", v, err)
	}
	return body
}

// unchanged leaves an index as it is.
func unchanged(*update.Index) {}
