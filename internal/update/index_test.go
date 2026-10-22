// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"crypto/ed25519"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/update"
)

// readKeyList returns the key list k signs with its first root, failing the test when it is refused.
func readKeyList(t *testing.T, k keys) update.KeyList {
	t.Helper()
	body := marshal(t, k.keyList(t))
	list, err := update.ReadKeyList(body, signLines(t, body, k.root1), k.trusted(), now)
	if err != nil {
		t.Fatalf("ReadKeyList() error = %v, want nil", err)
	}
	return list
}

func TestReadIndexTakesTheReleaseKeyAndTheStandby(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	list := readKeyList(t, k)
	body := marshal(t, index(offer("0.21.0", "0.21.0")))
	for name, key := range map[string]ed25519.PrivateKey{"the release key": k.release, "the standby": k.standby} {
		got, err := list.ReadIndex(body, signIndex(body, key), "stable", now)
		if err != nil || got.Serial != 9 || len(got.Offers) != 1 {
			t.Errorf("ReadIndex() signed by %s = %+v, %v, want the index taken", name, got, err)
		}
	}
}

func TestReadIndexRefusesAKeyTheListDoesNotName(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body := marshal(t, index(offer("0.21.0", "0.21.0")))

	_, err := readKeyList(t, k).ReadIndex(body, signIndex(body, k.stranger), "stable", now)

	wantRefused(t, err, "no release or standby key the list names")
}

func TestReadIndexRefusesTheTimestampKey(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body := marshal(t, index(offer("0.21.0", "0.21.0")))

	_, err := readKeyList(t, k).ReadIndex(body, signIndex(body, k.timestamp), "stable", now)

	wantRefused(t, err, "no release or standby key the list names")
}

func TestReadIndexRefusesAChangedIndex(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body := marshal(t, index(offer("0.21.0", "0.21.0")))
	sig := signIndex(body, k.release)
	changed := marshal(t, index(offer("0.22.0", "0.22.0")))

	_, err := readKeyList(t, k).ReadIndex(changed, sig, "stable", now)

	wantRefused(t, err, "no release or standby key the list names")
}

func TestReadIndexRefusesASignatureFileThatIsNoSignature(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body := marshal(t, index(offer("0.21.0", "0.21.0")))

	_, err := readKeyList(t, k).ReadIndex(body, []byte("not a signature\n"), "stable", now)

	wantRefused(t, err, "no release or standby key the list names")
}

func TestReadIndexRefusesAnExpiredIndex(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	expired := index(offer("0.21.0", "0.21.0"))
	expired.Expires = now.Add(-1)
	body := marshal(t, expired)

	_, err := readKeyList(t, k).ReadIndex(body, signIndex(body, k.release), "stable", now)

	wantRefused(t, err, "which expired")
}

func TestReadIndexRefusesAnotherChannel(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	beta := index(offer("0.21.0", "0.21.0"))
	beta.Channel = "beta"
	body := marshal(t, beta)

	_, err := readKeyList(t, k).ReadIndex(body, signIndex(body, k.release), "stable", now)

	wantRefused(t, err, `the site asks for "stable"`)
}

func TestReadIndexOfAnyAgeTakesAnExpiredIndex(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	list := readKeyList(t, k)
	expired := index(offer("0.21.0", "0.21.0"))
	expired.Expires = now.AddDate(-1, 0, 0)
	body := marshal(t, expired)
	for name, key := range map[string]ed25519.PrivateKey{"the release key": k.release, "the standby": k.standby} {
		got, err := list.ReadIndexOfAnyAge(body, signIndex(body, key), "stable")
		if err != nil || got.Serial != 9 || len(got.Offers) != 1 {
			t.Errorf("ReadIndexOfAnyAge() of an expired index signed by %s = %+v, %v, want it taken", name, got, err)
		}
	}
}

func TestReadIndexOfAnyAgeRefusesWhatASiteRefusesButTheExpiry(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	list := readKeyList(t, k)
	body := marshal(t, index(offer("0.21.0", "0.21.0")))
	unknownSchema := index()
	unknownSchema.Schema = 2
	beta := index()
	beta.Channel = "beta"
	for name, tc := range map[string]struct {
		body, sig []byte
		text      string
	}{
		"a key the list does not name": {body, signIndex(body, k.stranger), "no release or standby key the list names"},
		"the timestamp key":            {body, signIndex(body, k.timestamp), "no release or standby key the list names"},
		"text that is not JSON":        {[]byte("index"), signIndex([]byte("index"), k.release), "not JSON"},
		"a schema this version cannot read": {marshal(t, unknownSchema), signIndex(marshal(t, unknownSchema), k.release),
			"of schema 2"},
		"another channel": {marshal(t, beta), signIndex(marshal(t, beta), k.release), `the site asks for "stable"`},
		"an offer with a short minimum": {marshal(t, index(offer("0.21.0", "0.21"))),
			signIndex(marshal(t, index(offer("0.21.0", "0.21"))), k.release), `the minimum version "0.21"`},
	} {
		_, err := list.ReadIndexOfAnyAge(tc.body, tc.sig, "stable")
		if err == nil || !strings.Contains(err.Error(), tc.text) {
			t.Errorf("ReadIndexOfAnyAge() of %s = %v, want it refused saying %q", name, err, tc.text)
		}
	}
}

func TestReadIndexRefusesAnIndexItCannotRead(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	list := readKeyList(t, k)
	unknownSchema := index()
	unknownSchema.Schema = 2
	noVersion := offer("0.21.0", "0.21.0")
	noVersion.Release = json.RawMessage(`{"artifacts":{}}`)
	badMinimum := offer("0.21.0", "0.21")
	badRollout := offer("0.21.0", "0.21.0")
	badRollout.Rollout = 101
	for name, body := range map[string][]byte{
		"text that is not JSON":             []byte("index"),
		"a schema this version cannot read": marshal(t, unknownSchema),
		"an offer naming no version":        marshal(t, index(noVersion)),
		"an offer with a short minimum":     marshal(t, index(badMinimum)),
		"an offer rolled out past 100":      marshal(t, index(badRollout)),
		"an index with no expiry":           []byte(`{"schema":1,"channel":"stable","serial":9,"offers":[]}`),
	} {
		if _, err := list.ReadIndex(body, signIndex(body, k.release), "stable", now); err == nil {
			t.Errorf("ReadIndex() of %s = nil, want it refused", name)
		}
	}
}
