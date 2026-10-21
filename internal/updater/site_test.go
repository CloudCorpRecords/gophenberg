// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"archive/tar"
	"context"
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

// updateSite is a loopback update site whose key list, index and archives one test signs with keys it made.
type updateSite struct {
	t         *testing.T
	root      *ecdsa.PrivateKey
	release   ed25519.PrivateKey
	timestamp ed25519.PrivateKey
	stranger  ed25519.PrivateKey
	address   string
	mu        sync.Mutex
	files     map[string][]byte
	served    func(path string)
}

// newUpdateSite starts an update site on loopback whose key list the root signed, closed when t ends.
func newUpdateSite(t *testing.T) *updateSite {
	t.Helper()
	root, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("making the root key: %v", err)
	}
	site := &updateSite{
		t: t, root: root, release: newSigner(t), timestamp: newSigner(t), stranger: newSigner(t),
		files: map[string][]byte{}, served: func(string) {},
	}
	server := httptest.NewServer(http.HandlerFunc(site.serve))
	t.Cleanup(server.Close)
	site.address = server.URL
	site.publishKeyList()
	return site
}

// serve answers the file the request names, or 404, once the hook of the site saw its path.
func (s *updateSite) serve(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	s.mu.Lock()
	body, found := s.files[path]
	served := s.served
	s.mu.Unlock()
	served(path)
	if !found {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(body)
}

// onServe makes the site run hook with the path of each request before it answers it.
func (s *updateSite) onServe(hook func(path string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.served = hook
}

// put serves body at path.
func (s *updateSite) put(path string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[path] = body
}

// marshalled returns v as JSON, failing the test when it cannot.
func marshalled(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("writing %T: %v", v, err)
	}
	return body
}

// multikey returns the Multikey of key, failing the test when it cannot.
func (s *updateSite) multikey(key ed25519.PrivateKey) string {
	s.t.Helper()
	text, err := signature.Multikey(key.Public())
	if err != nil {
		s.t.Fatalf("writing a Multikey: %v", err)
	}
	return text
}

// publishKeyList serves a key list naming the release key and the timestamp key, signed by the root.
func (s *updateSite) publishKeyList() {
	s.t.Helper()
	body := marshalled(s.t, update.KeyList{
		Schema: 1, Serial: 1, Expires: time.Now().AddDate(1, 0, 0).UTC().Truncate(time.Second),
		Keys: []update.ListedKey{
			{Role: update.RoleRelease, Multikey: s.multikey(s.release)},
			{Role: update.RoleTimestamp, Multikey: s.multikey(s.timestamp)},
		},
	})
	line, err := update.SignKeyList(body, s.root)
	if err != nil {
		s.t.Fatalf("signing the key list: %v", err)
	}
	s.put(update.KeyListPath, body)
	s.put(update.KeyListPath+update.SignatureSuffix, []byte(line+"\n"))
}

// publish serves the stable index offering offers, signed by key.
func (s *updateSite) publish(key ed25519.PrivateKey, offers ...update.Offer) {
	s.t.Helper()
	body := marshalled(s.t, update.Index{
		Schema: 1, Channel: "stable", Serial: 1, Expires: time.Now().AddDate(0, 0, 30).UTC().Truncate(time.Second),
		Offers: offers,
	})
	s.put(update.IndexPath("stable"), body)
	s.put(update.IndexPath("stable")+update.SignatureSuffix, []byte(update.SignIndex(body, key)+"\n"))
}

// archive serves body as the archive of 0.21.0 for this platform and returns its record entry, signed by key.
func (s *updateSite) archive(body []byte, key ed25519.PrivateKey) update.Artifact {
	s.t.Helper()
	path := "archives/gophenberg_" + offered + "_" + update.Platform() + ".tar.gz"
	s.put(path, body)
	artifact := artifactFor(s.t, body, s.address+"/"+path, key)
	artifact.ID = update.Platform()
	return artifact
}

// replace serves body in place of the archive the record entry names.
func (s *updateSite) replace(artifact update.Artifact, body []byte) {
	s.put(strings.TrimPrefix(artifact.URL, s.address+"/"), body)
}

// config returns the check of the site for an install running running, reading the files at now.
func (s *updateSite) config(running string, now time.Time) update.Config {
	address, err := update.ParseAddress(s.address)
	if err != nil {
		s.t.Fatalf("reading the address of the update site: %v", err)
	}
	return update.Config{
		Address: address, Channel: "stable", Roots: []*ecdsa.PublicKey{&s.root.PublicKey},
		Client: update.NewClient(5 * time.Second), Site: update.Site{Running: running},
		Now: func() time.Time { return now },
	}
}

// offerOf returns the offer of release to every site under minimum, on PostgreSQL 15 and kit 0.9.0, with its archives.
func offerOf(t *testing.T, release, minimum string, archives ...update.Artifact) update.Offer {
	t.Helper()
	return update.Offer{
		Rollout: 100, MinimumVersion: minimum, PostgreSQL: 15, Kits: []string{"0.9.0"},
		Notes:   "https://example.com/releases/" + release,
		Release: marshalled(t, update.Release{Version: release, Artifacts: update.Artifacts{Package: archives}}),
	}
}

// aboutScript returns a stand-in gophenberg whose about -json names release and stamp, then exits with exit.
func aboutScript(release, stamp string, exit int) string {
	return fmt.Sprintf("#!/bin/sh\necho '{\"schema\":1,\"name\":\"gophenberg\",\"version\":%q,\"stamp\":%q}'\nexit %d\n",
		release, stamp, exit)
}

// releaseArchive returns the tar.gz of a release whose gophenberg is script.
func releaseArchive(t *testing.T, script string) []byte {
	t.Helper()
	return packed(t, file("LICENSE", "the Apache License 2.0"), file("NOTICE", "the notice of Gophenberg"),
		entry{name: "gophenberg", body: script, mode: 0o755, typeflag: tar.TypeReg},
		file("release.json", `{"version":"0.21.0"}`))
}

// memoryStore is a site setting store held in memory.
type memoryStore struct {
	mu     sync.Mutex
	values map[string]string
}

// Lookup returns the value of key and whether the store holds one.
func (m *memoryStore) Lookup(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, found := m.values[key]
	return value, found, nil
}

// Save stores values.
func (m *memoryStore) Save(_ context.Context, values map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, value := range values {
		m.values[key] = value
	}
	return nil
}

// Update stores what change returns for the values keys hold.
func (m *memoryStore) Update(
	_ context.Context, keys []string, change func(map[string]string) (map[string]string, error),
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	held := map[string]string{}
	for _, key := range keys {
		held[key] = m.values[key]
	}
	changed, err := change(held)
	for key, value := range changed {
		m.values[key] = value
	}
	return err
}
