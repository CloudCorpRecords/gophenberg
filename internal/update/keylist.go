// SPDX-License-Identifier: Apache-2.0

package update

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/internal/signature"
)

// errNoTrustedRoot refuses a key list that carries no signature from a trusted root.
var errNoTrustedRoot = errors.New("update: refused the key list, no trusted root signed it")

// SignKeyList returns the line a root signs the exact bytes of a key list with: its Multikey, then its signature.
func SignKeyList(body []byte, root *ecdsa.PrivateKey) (string, error) {
	signed, err := signature.SignCompact(root, body)
	if err != nil {
		return "", err
	}
	name, _ := signature.Multikey(&root.PublicKey)
	return name + " " + signed, nil
}

// ReadKeyList returns the key list body holds once one trusted root signed it in sigs and it is still valid at now.
func ReadKeyList(body, sigs []byte, roots []*ecdsa.PublicKey, now time.Time) (KeyList, error) {
	if err := verifyKeyList(body, sigs, roots); err != nil {
		return KeyList{}, err
	}
	var list KeyList
	if err := json.Unmarshal(body, &list); err != nil {
		return KeyList{}, fmt.Errorf("update: refused the key list, which is not JSON: %w", err)
	}
	if err := readable("key list", list.Schema, list.Expires, now); err != nil {
		return KeyList{}, err
	}
	return list, list.checkSigners()
}

// Signers returns the Ed25519 keys the list names as release or standby keys.
func (l KeyList) Signers() []ed25519.PublicKey {
	var signers []ed25519.PublicKey
	for _, listed := range l.Keys {
		if key, ok := listed.signer(); ok {
			signers = append(signers, key)
		}
	}
	return signers
}

// checkSigners returns an error naming a release or standby key that is not an Ed25519 Multikey.
func (l KeyList) checkSigners() error {
	for _, listed := range l.Keys {
		if _, ok := listed.signer(); !ok && listed.signs() {
			return fmt.Errorf("update: refused the key list, its %s key %q is not an Ed25519 Multikey", listed.Role,
				listed.Multikey)
		}
	}
	return nil
}

// signs reports whether the listed key may sign an index.
func (k ListedKey) signs() bool {
	return k.Role == RoleRelease || k.Role == RoleStandby
}

// signer returns the Ed25519 key of a listed key that may sign an index.
func (k ListedKey) signer() (ed25519.PublicKey, bool) {
	if !k.signs() {
		return nil, false
	}
	key, err := signature.ParseMultikey(k.Multikey)
	signer, ok := key.(ed25519.PublicKey)
	return signer, err == nil && ok
}

// verifyKeyList returns nil once a trusted root signed body in sigs, refusing a root line that does not hold.
func verifyKeyList(body, sigs []byte, roots []*ecdsa.PublicKey) error {
	held := false
	for line := range strings.SplitSeq(strings.TrimSpace(string(sigs)), "\n") {
		root, signed, err := rootLine(line, roots)
		if err != nil {
			return err
		}
		if root == nil {
			continue
		}
		if err := signature.VerifyCompact(root, body, signed); err != nil {
			return fmt.Errorf("update: refused the key list, the signature of a trusted root does not hold: %w", err)
		}
		held = true
	}
	if !held {
		return errNoTrustedRoot
	}
	return nil
}

// rootLine returns the trusted root a signature line names and its signature, a nil root for any other key.
func rootLine(line string, roots []*ecdsa.PublicKey) (*ecdsa.PublicKey, string, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return nil, "", fmt.Errorf("update: refused the key list, a signature line holds %d words, not a key and "+
			"a signature", len(fields))
	}
	key, err := signature.ParseMultikey(fields[0])
	if err != nil {
		return nil, "", fmt.Errorf("update: refused the key list, a signature line names no key: %w", err)
	}
	for _, root := range roots {
		if named, ok := key.(*ecdsa.PublicKey); ok && sameKey(named, root) {
			return root, fields[1], nil
		}
	}
	return nil, "", nil
}

// sameKey reports whether two P-256 public keys are the same point.
func sameKey(a, b *ecdsa.PublicKey) bool {
	pa, errA := a.Bytes()
	pb, errB := b.Bytes()
	return errA == nil && errB == nil && bytes.Equal(pa, pb)
}
