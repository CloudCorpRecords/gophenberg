// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/signature"
	"github.com/gopherium/gophenberg/internal/update"
)

// signerMultikey returns the Multikey of a fresh Ed25519 key.
func signerMultikey(t *testing.T) string {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("making an Ed25519 key: %v", err)
	}
	text, err := signature.Multikey(public)
	if err != nil {
		t.Fatalf("writing a Multikey: %v", err)
	}
	return text
}

// writtenKeyList runs keys for two fresh signers and writes its answer to keys.json in dir.
func writtenKeyList(t *testing.T, dir string) string {
	t.Helper()
	got := runTool("keys", "-release", signerMultikey(t), "-standby", signerMultikey(t))
	if got.code != 0 {
		t.Fatalf("keys = %d, %q, want 0", got.code, got.stderr)
	}
	return writeFile(t, dir, "keys.json", got.stdout)
}

// readList returns the key list in the file at path, signed in the file beside it, under the roots.
func readList(t *testing.T, path string, roots ...*ecdsa.PrivateKey) (update.KeyList, error) {
	t.Helper()
	public := make([]*ecdsa.PublicKey, len(roots))
	for i, root := range roots {
		public[i] = &root.PublicKey
	}
	return update.ReadKeyList([]byte(readText(t, path)), []byte(readText(t, path+".sig")), public, time.Now())
}

func TestKeysWritesAKeyListOfTheReleaseKeyAndTheStandby(t *testing.T) {
	t.Parallel()

	release, standby := signerMultikey(t), signerMultikey(t)
	before := time.Now()

	got := runTool("keys", "-release", release, "-standby", standby)

	after := time.Now()
	var list update.KeyList
	if err := json.Unmarshal([]byte(got.stdout), &list); got.code != 0 || err != nil {
		t.Fatalf("keys = %d, %q, %v, want 0 and one key list", got.code, got.stderr, err)
	}
	want := []update.ListedKey{
		{Role: update.RoleRelease, Multikey: release}, {Role: update.RoleStandby, Multikey: standby},
	}
	if list.Schema != 1 || !slices.Equal(list.Keys, want) {
		t.Errorf("keys wrote %+v, want schema 1 and the two keys in order", list)
	}
	if list.Serial < before.Unix() || list.Serial > after.Unix() {
		t.Errorf("keys wrote serial %d, want the Unix time of the run", list.Serial)
	}
	year := 8760 * time.Hour
	if list.Expires.Before(before.Add(year).Truncate(time.Second)) || list.Expires.After(after.Add(year)) {
		t.Errorf("keys wrote the expiry %v, want 8760 hours after the run", list.Expires)
	}
}

func TestKeysTakesASerialAndAValidity(t *testing.T) {
	t.Parallel()

	before := time.Now()
	got := runTool("keys", "-release", signerMultikey(t), "-standby", signerMultikey(t), "-serial", "7", "-valid", "48h")

	var list update.KeyList
	if err := json.Unmarshal([]byte(got.stdout), &list); got.code != 0 || err != nil {
		t.Fatalf("keys = %d, %q, %v, want 0 and one key list", got.code, got.stderr, err)
	}
	if list.Serial != 7 || list.Expires.Before(before.Add(48*time.Hour).Truncate(time.Second)) ||
		list.Expires.After(time.Now().Add(48*time.Hour)) {
		t.Errorf("keys wrote serial %d and expiry %v, want 7 and two days on", list.Serial, list.Expires)
	}
}

func TestKeysRefusesAMissingOrWrongKey(t *testing.T) {
	t.Parallel()

	release := signerMultikey(t)
	for name, tc := range map[string]struct {
		args []string
		code int
	}{
		"no release key":           {args: []string{"-standby", release}, code: 2},
		"no standby":               {args: []string{"-release", release}, code: 2},
		"a validity of zero":       {args: []string{"-release", release, "-standby", release, "-valid", "0s"}, code: 2},
		"a file to read":           {args: []string{"-release", release, "-standby", release, "keys.json"}, code: 2},
		"a release key on P-256":   {args: []string{"-release", p256Multikey(t), "-standby", release}, code: 1},
		"a standby that is no key": {args: []string{"-release", release, "-standby", "z6Mk"}, code: 1},
		"a flag it does not know":  {args: []string{"-root", release}, code: 2},
	} {
		if got := runTool(append([]string{"keys"}, tc.args...)...); got.code != tc.code {
			t.Errorf("keys with %s = %d, %q, want %d", name, got.code, got.stderr, tc.code)
		}
	}
}

// p256Multikey returns the Multikey of a fresh P-256 key.
func p256Multikey(t *testing.T) string {
	t.Helper()
	text, err := signature.Multikey(&newTestP256(t).PublicKey)
	if err != nil {
		t.Fatalf("writing a Multikey: %v", err)
	}
	return text
}

func TestSignKeysAddsALineForEachRoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	list := writtenKeyList(t, dir)
	root1, root2 := newTestP256(t), newTestP256(t)
	plain := writeFile(t, dir, "root1.pem", plainPEM(t, root1))
	sealed := writeFile(t, dir, "root2.pem", sealedPEM(t, root2))
	passphrase := writeFile(t, dir, "passphrase", string(testSealing.passphrase)+"\n")

	first := runTool("sign-keys", "-key", plain, list)
	second := runTool("sign-keys", "-key", sealed, "-passphrase-file", passphrase, list)

	if first.code != 0 || second.code != 0 || !strings.HasPrefix(first.stdout, "signed "+list+" as zDn") {
		t.Fatalf("sign-keys = %d %q and %d %q, want both signed", first.code, first.stderr+first.stdout, second.code,
			second.stderr)
	}
	if lines := strings.Count(readText(t, list+".sig"), "\n"); lines != 2 {
		t.Errorf("keys.json.sig holds %d lines, want one for each root", lines)
	}
	for name, roots := range map[string][]*ecdsa.PrivateKey{"both": {root1, root2}, "the first": {root1},
		"the second": {root2}} {
		if _, err := readList(t, list, roots...); err != nil {
			t.Errorf("the signed key list under %s = %v, want it taken", name, err)
		}
	}
}

func TestSignKeysRefusesARootThatSignedAlready(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	list := writtenKeyList(t, dir)
	root := writeFile(t, dir, "root1.pem", plainPEM(t, newTestP256(t)))
	if got := runTool("sign-keys", "-key", root, list); got.code != 0 {
		t.Fatalf("sign-keys = %d, %q, want 0", got.code, got.stderr)
	}

	got := runTool("sign-keys", "-key", root, list)

	if got.code != 1 || !strings.Contains(got.stderr, "already signed") {
		t.Errorf("sign-keys again = %d, %q, want 1 and the root named as having signed", got.code, got.stderr)
	}
	if lines := strings.Count(readText(t, list+".sig"), "\n"); lines != 1 {
		t.Errorf("keys.json.sig holds %d lines, want the one line kept", lines)
	}
}

func TestSignKeysRefusesWhatItCannotSignWith(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	list := writtenKeyList(t, dir)
	_, ed25519Key, _ := ed25519.GenerateKey(rand.Reader)
	release := writeFile(t, dir, "release.pem", plainPEM(t, ed25519Key))
	p384 := writeFile(t, dir, "p384.pem", plainPEM(t, newTestP384(t)))
	root := writeFile(t, dir, "root.pem", plainPEM(t, newTestP256(t)))
	for name, tc := range map[string]struct {
		args []string
		code int
	}{
		"no key":                  {args: []string{list}, code: 2},
		"no key list":             {args: []string{"-key", root}, code: 2},
		"two key lists":           {args: []string{"-key", root, list, list}, code: 2},
		"an Ed25519 key":          {args: []string{"-key", release, list}, code: 1},
		"a P-384 key":             {args: []string{"-key", p384, list}, code: 1},
		"a key file not there":    {args: []string{"-key", filepath.Join(dir, "missing.pem"), list}, code: 1},
		"a key list not there":    {args: []string{"-key", root, filepath.Join(dir, "missing.json")}, code: 1},
		"a passphrase not there":  {args: []string{"-key", root, "-passphrase-file", filepath.Join(dir, "x"), list}, code: 1},
		"a flag it does not know": {args: []string{"-release", root, list}, code: 2},
	} {
		if got := runTool(append([]string{"sign-keys"}, tc.args...)...); got.code != tc.code {
			t.Errorf("sign-keys with %s = %d, %q, want %d", name, got.code, got.stderr, tc.code)
		}
	}
	if _, err := os.Stat(list + ".sig"); err == nil {
		t.Error("a refused sign-keys wrote keys.json.sig, want nothing written")
	}
}

func TestSignKeysReportsASignatureFileItCannotWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	list := writtenKeyList(t, dir)
	root := writeFile(t, dir, "root.pem", plainPEM(t, newTestP256(t)))
	if err := os.Mkdir(list+".sig", 0o700); err != nil {
		t.Fatalf("making a folder where the signatures go: %v", err)
	}

	if got := runTool("sign-keys", "-key", root, list); got.code != 1 {
		t.Errorf("sign-keys into a folder = %d, %q, want 1", got.code, got.stderr)
	}
}

func TestSignKeysReportsASignatureFileItCannotAddTo(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root writes a read-only file, so the refusal cannot be shown")
	}

	dir := t.TempDir()
	list := writtenKeyList(t, dir)
	root := writeFile(t, dir, "root.pem", plainPEM(t, newTestP256(t)))
	if err := os.WriteFile(list+".sig", nil, 0o444); err != nil {
		t.Fatalf("writing a read-only signature file: %v", err)
	}

	if got := runTool("sign-keys", "-key", root, list); got.code != 1 || !strings.Contains(got.stderr, "writing") {
		t.Errorf("sign-keys onto a read-only file = %d, %q, want 1 and the write named", got.code, got.stderr)
	}
}
