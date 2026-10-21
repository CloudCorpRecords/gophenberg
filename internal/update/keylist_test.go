// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/update"
)

func TestReadKeyListTakesEitherRootAlone(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body := marshal(t, k.keyList(t))
	for name, roots := range map[string][]*ecdsa.PrivateKey{
		"the first root alone":    {k.root1},
		"the second root alone":   {k.root2},
		"both roots":              {k.root1, k.root2},
		"a stranger, then a root": {k.strangerRoot, k.root2},
	} {
		list, err := update.ReadKeyList(body, signLines(t, body, roots...), k.trusted(), now)
		if err != nil || list.Serial != 5 || len(list.Keys) != 3 {
			t.Errorf("ReadKeyList() signed by %s = %+v, %v, want the list taken", name, list, err)
		}
	}
}

func TestReadKeyListRefusesAListNoTrustedRootSigned(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body := marshal(t, k.keyList(t))

	_, err := update.ReadKeyList(body, signLines(t, body, k.strangerRoot), k.trusted(), now)

	wantRefused(t, err, "no trusted root signed it")
}

func TestReadKeyListRefusesASignatureThatFailsUnderARoot(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body := marshal(t, k.keyList(t))
	other := marshal(t, update.KeyList{Schema: 1, Serial: 6, Expires: now.AddDate(1, 0, 0)})
	forged := strings.Join([]string{
		strings.TrimSpace(string(signLines(t, body, k.root1))),
		strings.TrimSpace(string(signLines(t, other, k.root2))),
	}, "\n")

	_, err := update.ReadKeyList(body, []byte(forged), k.trusted(), now)

	wantRefused(t, err, "does not hold")
}

func TestReadKeyListRefusesAMalformedSignatureFile(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body := marshal(t, k.keyList(t))
	signed := strings.TrimSpace(string(signLines(t, body, k.root1)))

	for name, sigs := range map[string]string{
		"a line of one word":        "zDnaesSvh9HYr1uiuwM9WMLyRiwQ9mu1jogeggWXieTyN9ypm",
		"a line of three words":     signed + " more",
		"a key that does not parse": "z0 " + strings.Fields(signed)[1],
		"an empty file":             "",
	} {
		if _, err := update.ReadKeyList(body, []byte(sigs), k.trusted(), now); err == nil {
			t.Errorf("ReadKeyList() with %s = nil, want it refused", name)
		}
	}
}

func TestReadKeyListRefusesAnExpiredList(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	list := k.keyList(t)
	list.Expires = now.Add(-1)
	body := marshal(t, list)

	_, err := update.ReadKeyList(body, signLines(t, body, k.root1), k.trusted(), now)

	wantRefused(t, err, "which expired")
}

func TestReadKeyListRefusesAListItCannotRead(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	ed25519Root := k.keyList(t)
	ed25519Root.Keys = []update.ListedKey{{Role: update.RoleRelease, Multikey: multikey(t, &k.root1.PublicKey)}}
	unknownSchema := k.keyList(t)
	unknownSchema.Schema = 2
	for name, body := range map[string][]byte{
		"text that is not JSON":             []byte("keys"),
		"a schema this version cannot read": marshal(t, unknownSchema),
		"a release key that is not Ed25519": marshal(t, ed25519Root),
		"a list with no expiry":             []byte(`{"schema":1,"serial":5,"keys":[]}`),
	} {
		if _, err := update.ReadKeyList(body, signLines(t, body, k.root1), k.trusted(), now); err == nil {
			t.Errorf("ReadKeyList() of %s = nil, want it refused", name)
		}
	}
}

func TestReadKeyListRefusesAKeyItCannotTrustInItsRole(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	for name, tc := range map[string]struct {
		edit func(keys []update.ListedKey) []update.ListedKey
		text string
	}{
		"a timestamp key that is not Ed25519": {
			edit: func(keys []update.ListedKey) []update.ListedKey {
				keys[2].Multikey = multikey(t, &k.root1.PublicKey)
				return keys
			},
			text: "its timestamp key",
		},
		"no timestamp key": {
			edit: func(keys []update.ListedKey) []update.ListedKey { return keys[:2] },
			text: "names no timestamp key",
		},
		"the release key as the timestamp key": {
			edit: func(keys []update.ListedKey) []update.ListedKey {
				keys[2].Multikey = keys[0].Multikey
				return keys
			},
			text: "names one key as its release key and its timestamp key",
		},
		"the release key as the standby": {
			edit: func(keys []update.ListedKey) []update.ListedKey {
				keys[1].Multikey = keys[0].Multikey
				return keys
			},
			text: "names one key as its release key and its standby key",
		},
	} {
		list := k.keyList(t)
		list.Keys = tc.edit(list.Keys)
		body := marshal(t, list)

		_, err := update.ReadKeyList(body, signLines(t, body, k.root1), k.trusted(), now)

		if err == nil || !strings.Contains(err.Error(), tc.text) {
			t.Errorf("ReadKeyList() of a list with %s = %v, want a refusal saying %q", name, err, tc.text)
		}
	}
}

func TestReadKeyListLeavesARoleItDoesNotKnowOut(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	list := k.keyList(t)
	list.Keys = append(list.Keys, update.ListedKey{Role: "mirror", Multikey: multikey(t, &k.root1.PublicKey)})
	body := marshal(t, list)

	got, err := update.ReadKeyList(body, signLines(t, body, k.root1), k.trusted(), now)

	if err != nil || len(got.Signers()) != 2 || len(got.TimestampKeys()) != 1 {
		t.Errorf("ReadKeyList() = %d signers and %d timestamp keys, %v, want the three it knows", len(got.Signers()),
			len(got.TimestampKeys()), err)
	}
}

func TestKeyListKeepsTheTimestampKeyApartFromTheSigners(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	list := k.keyList(t)

	signers, stamps := list.Signers(), list.TimestampKeys()

	wantSigners := []ed25519.PublicKey{k.release.Public().(ed25519.PublicKey), k.standby.Public().(ed25519.PublicKey)}
	if !slices.EqualFunc(signers, wantSigners, func(a, b ed25519.PublicKey) bool { return a.Equal(b) }) {
		t.Errorf("Signers() = %x, want the release key and the standby", signers)
	}
	if len(stamps) != 1 || !stamps[0].Equal(k.timestamp.Public()) {
		t.Errorf("TimestampKeys() = %x, want the timestamp key alone", stamps)
	}
}

func TestSignKeyListRefusesARootNotOnP256(t *testing.T) {
	t.Parallel()

	if _, err := update.SignKeyList([]byte("keys"), &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{}}); err == nil {
		t.Error("SignKeyList() with a key on no curve = nil, want it refused")
	}
}
