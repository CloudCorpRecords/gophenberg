// SPDX-License-Identifier: Apache-2.0

package update

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/internal/signature"
)

var (
	// errNoTrustedRoot refuses a key list that carries no signature from a trusted root.
	errNoTrustedRoot = errors.New("update: refused the key list, no trusted root signed it")
	// errNoTimestampKey refuses a key list that names no key to sign the timestamp file.
	errNoTimestampKey = errors.New("update: refused the key list, which names no timestamp key")
)

// knownRoles are the roles this version reads a listed key under.
var knownRoles = []Role{RoleRelease, RoleStandby, RoleTimestamp}

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
	return list, list.checkKeys()
}

// Signers returns the Ed25519 keys the list names as release or standby keys.
func (l KeyList) Signers() []ed25519.PublicKey {
	return l.keysOf(RoleRelease, RoleStandby)
}

// TimestampKeys returns the Ed25519 keys the list names as timestamp keys.
func (l KeyList) TimestampKeys() []ed25519.PublicKey {
	return l.keysOf(RoleTimestamp)
}

// keysOf returns the Ed25519 keys the list names under any of roles.
func (l KeyList) keysOf(roles ...Role) []ed25519.PublicKey {
	var named []ed25519.PublicKey
	for _, listed := range l.Keys {
		if key, ok := listed.publicKey(); ok && slices.Contains(roles, listed.Role) {
			named = append(named, key)
		}
	}
	return named
}

// checkKeys refuses a known key that is not Ed25519 or holds two roles, and a list with no timestamp key.
func (l KeyList) checkKeys() error {
	held := map[string]Role{}
	for _, listed := range l.Keys {
		if !slices.Contains(knownRoles, listed.Role) {
			continue
		}
		key, ok := listed.publicKey()
		if !ok {
			return fmt.Errorf("update: refused the key list, its %s key %q is not an Ed25519 Multikey", listed.Role,
				listed.Multikey)
		}
		if role, twice := held[string(key)]; twice {
			return fmt.Errorf("update: refused the key list, it names one key as its %s key and its %s key", role,
				listed.Role)
		}
		held[string(key)] = listed.Role
	}
	if len(l.TimestampKeys()) == 0 {
		return errNoTimestampKey
	}
	return nil
}

// publicKey returns the Ed25519 key a listed key names.
func (k ListedKey) publicKey() (ed25519.PublicKey, bool) {
	key, err := signature.ParseMultikey(k.Multikey)
	public, ok := key.(ed25519.PublicKey)
	return public, err == nil && ok
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
