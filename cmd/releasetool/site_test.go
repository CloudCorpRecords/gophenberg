// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/signature"
	"github.com/gopherium/gophenberg/internal/update"
)

// testSite is a clone of an update site in a test folder, with the root and the three keys that sign its files.
type testSite struct {
	t                           *testing.T
	dir                         string
	root                        *ecdsa.PrivateKey
	release, standby, timestamp ed25519.PrivateKey
}

// newTestSite writes a clone holding a key list a fresh root signed, naming three fresh keys.
func newTestSite(t *testing.T) *testSite {
	t.Helper()
	site := &testSite{
		t: t, dir: t.TempDir(), root: newTestP256(t), release: newTestEd25519(t), standby: newTestEd25519(t),
		timestamp: newTestEd25519(t),
	}
	site.putKeyList(site.keyList(), site.root)
	return site
}

// newTestEd25519 makes an Ed25519 key for a test.
func newTestEd25519(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating an Ed25519 key: %v", err)
	}
	return key
}

// keyList returns a key list of serial 5 naming the release key, the standby and the timestamp key, valid for a year.
func (s *testSite) keyList() update.KeyList {
	return update.KeyList{
		Schema: 1, Serial: 5, Expires: time.Now().AddDate(1, 0, 0).UTC().Truncate(time.Second),
		Keys: []update.ListedKey{
			{Role: update.RoleRelease, Multikey: multikeyOf(s.t, s.release)},
			{Role: update.RoleStandby, Multikey: multikeyOf(s.t, s.standby)},
			{Role: update.RoleTimestamp, Multikey: multikeyOf(s.t, s.timestamp)},
		},
	}
}

// putKeyList writes list to the clone with the signature line of root.
func (s *testSite) putKeyList(list update.KeyList, root *ecdsa.PrivateKey) {
	s.t.Helper()
	body := jsonOf(s.t, list)
	line, err := update.SignKeyList(body, root)
	if err != nil {
		s.t.Fatalf("signing the key list: %v", err)
	}
	s.put(update.KeyListPath, body, line)
}

// putIndex writes index to the clone as the index of channel, signed by key.
func (s *testSite) putIndex(channel string, index update.Index, key ed25519.PrivateKey) {
	s.t.Helper()
	body := jsonOf(s.t, index)
	s.put(update.IndexPath(channel), body, update.SignIndex(body, key))
}

// put writes body to path in the clone and line to the .sig beside it.
func (s *testSite) put(path string, body []byte, line string) {
	s.t.Helper()
	file := s.path(path)
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		s.t.Fatalf("making the folder of %s: %v", path, err)
	}
	writeFile(s.t, filepath.Dir(file), filepath.Base(file), string(body))
	writeFile(s.t, filepath.Dir(file), filepath.Base(file)+".sig", line+"\n")
}

// path returns where path of the update site sits in the clone.
func (s *testSite) path(path string) string {
	return filepath.Join(s.dir, filepath.FromSlash(path))
}

// flags returns the flags that name the clone, its three keys and its root.
func (s *testSite) flags() []string {
	return []string{
		"-site", s.dir, "-release", multikeyOf(s.t, s.release), "-standby", multikeyOf(s.t, s.standby),
		"-timestamp", multikeyOf(s.t, s.timestamp), "-root", rootMultikey(s.t, s.root),
	}
}

// run runs the command on the clone with its flags and then more.
func (s *testSite) run(command string, more ...string) runResult {
	return runTool(slices.Concat([]string{command}, s.flags(), more)...)
}

// without returns args with the flag named and the value after it left out.
func without(args []string, flag string) []string {
	at := slices.Index(args, flag)
	if at < 0 {
		return args
	}
	return slices.Concat(args[:at], args[at+2:])
}

// rootMultikey returns the Multikey of a root key's public half.
func rootMultikey(t *testing.T, root *ecdsa.PrivateKey) string {
	t.Helper()
	text, err := signature.Multikey(&root.PublicKey)
	if err != nil {
		t.Fatalf("writing a Multikey: %v", err)
	}
	return text
}

// jsonOf returns v as JSON, failing the test when it cannot.
func jsonOf(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("writing %T: %v", v, err)
	}
	return body
}

// indexOf returns an index of channel and serial offering the versions in order, valid for three more days.
func indexOf(channel string, serial int64, versions ...string) update.Index {
	index := update.Index{Schema: 1, Channel: channel, Serial: serial, Expires: time.Now().AddDate(0, 0, 3)}
	for _, version := range versions {
		index.Offers = append(index.Offers, update.Offer{
			Rollout: 100, MinimumVersion: version, PostgreSQL: 15, Kits: []string{"0.9.0"},
			Notes: "https://example.com/releases/v" + version, Release: json.RawMessage(fmt.Sprintf(`{"version":%q}`, version)),
		})
	}
	return index
}
