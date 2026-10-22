// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"strings"
	"testing"
	"time"
)

func TestReadTimestampTakesAFileTheTimestampKeySigned(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body, sig := signedTimestamp(t, timestamp(), k.timestamp)

	got, err := readKeyList(t, k).ReadTimestamp(body, sig, now)

	if err != nil || string(marshal(t, got)) != string(body) {
		t.Errorf("ReadTimestamp() = %+v, %v, want the key list 5 and the stable index 9 named", got, err)
	}
}

func TestReadTimestampRefusesAnyKeyButATimestampKey(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	body := marshal(t, timestamp())
	for name, sig := range map[string][]byte{
		"the release key":              signIndex(body, k.release),
		"the standby":                  signIndex(body, k.standby),
		"a key the list does not name": signIndex(body, k.stranger),
		"a line that is no signature":  []byte("not a signature\n"),
	} {
		_, err := readKeyList(t, k).ReadTimestamp(body, sig, now)
		if text := "refused the timestamp file, no timestamp key the list names signed it"; err == nil ||
			!strings.Contains(err.Error(), text) {
			t.Errorf("ReadTimestamp() signed by %s = %v, want it refused saying %q", name, err, text)
		}
	}
}

func TestReadTimestampRefusesAChangedFile(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	_, sig := signedTimestamp(t, timestamp(), k.timestamp)
	changed := timestamp()
	changed.Indexes["stable"] = 10

	_, err := readKeyList(t, k).ReadTimestamp(marshal(t, changed), sig, now)

	wantRefused(t, err, "no timestamp key the list names signed it")
}

func TestReadTimestampRefusesAnExpiredFileAndNamesTheSiteClock(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	expired := timestamp()
	expired.Expires = now.Add(-time.Hour)
	body, sig := signedTimestamp(t, expired, k.timestamp)

	_, err := readKeyList(t, k).ReadTimestamp(body, sig, now)

	wantRefused(t, err, "refused the timestamp file, which expired at 2026-11-03T08:00:00Z, the site's clock reads "+
		"2026-11-03T09:00:00Z")
}

func TestReadTimestampRefusesAFileItCannotRead(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	list := readKeyList(t, k)
	unknownSchema := timestamp()
	unknownSchema.Schema = 2
	for name, body := range map[string][]byte{
		"text that is not JSON":             []byte("timestamp"),
		"a schema this version cannot read": marshal(t, unknownSchema),
		"a file with no expiry":             []byte(`{"schema":1,"keys":5,"indexes":{"stable":9}}`),
	} {
		if _, err := list.ReadTimestamp(body, signIndex(body, k.timestamp), now); err == nil {
			t.Errorf("ReadTimestamp() of %s = nil, want it refused", name)
		}
	}
}
