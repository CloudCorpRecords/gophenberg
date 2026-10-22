// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
)

// releaseFilesOf writes the release record and the release.json of version to dir, and returns their paths.
func releaseFilesOf(t *testing.T, dir, version, minimum string) (string, string) {
	t.Helper()
	record := writeFile(t, dir, "record-"+version+".json", fmt.Sprintf(
		`{"version":%q,"artifacts":{"package":[{"id":"linux-amd64"}]}}`, version))
	facts := writeFile(t, dir, "release-"+version+".json", fmt.Sprintf(
		`{"schema":1,"name":"gophenberg","version":%q,"minimum_version":%q,"postgresql":15,"kits":["0.9.0"],`+
			`"platform":"linux/amd64","notes":"https://example.com/releases/v%s"}`, version, minimum, version))
	return record, facts
}

// readIndexOf returns the index a run of index printed.
func readIndexOf(t *testing.T, got runResult) update.Index {
	t.Helper()
	var index update.Index
	if err := json.Unmarshal([]byte(got.stdout), &index); got.code != 0 || err != nil {
		t.Fatalf("index = %d, %q, %v, want 0 and one index", got.code, got.stderr, err)
	}
	return index
}

// versionsOf returns the versions an index offers, in order.
func versionsOf(index update.Index) []string {
	versions := make([]string, len(index.Offers))
	for i, offer := range index.Offers {
		versions[i] = offer.Version()
	}
	return versions
}

// siteWithStable writes a clone whose stable index of serial offers the versions in order, signed by the release key.
func siteWithStable(t *testing.T, serial int64, versions ...string) *testSite {
	t.Helper()
	site := newTestSite(t)
	site.putIndex("stable", indexOf("stable", serial, versions...), site.release)
	return site
}

func TestIndexStartsAChannelWithItsFirstRelease(t *testing.T) {
	t.Parallel()

	site := newTestSite(t)
	record, facts := releaseFilesOf(t, t.TempDir(), "0.20.0", "0.19.0")
	before := time.Now()

	index := readIndexOf(t, site.run("index", "-record", record, "-release-json", facts, "-valid", "2160h"))

	if index.Schema != 1 || index.Channel != "stable" || index.Serial < before.Unix() || index.Serial > time.Now().Unix() {
		t.Errorf("index wrote %+v, want schema 1, channel stable and the Unix time of the run as its serial", index)
	}
	if index.Expires.Before(before.Add(2160 * time.Hour).Truncate(time.Second)) {
		t.Errorf("index expires at %v, want 2160 hours after the run", index.Expires)
	}
	if len(index.Offers) != 1 {
		t.Fatalf("index offers %d versions, want one", len(index.Offers))
	}
	got := index.Offers[0]
	if got.Version() != "0.20.0" || got.Rollout != 100 || got.Auto || got.Withdrawn || got.MinimumVersion != "0.19.0" ||
		got.PostgreSQL != 15 || !slices.Equal(got.Kits, []string{"0.9.0"}) ||
		got.Notes != "https://example.com/releases/v0.20.0" || !strings.Contains(string(got.Release), "linux-amd64") {
		t.Errorf("index offers %+v, want the release offered to every site, notify only", got)
	}
}

func TestIndexPutsEachReleaseInOrderAfterTheNewest(t *testing.T) {
	t.Parallel()

	for added, want := range map[string][]string{
		"0.22.0": {"0.22.0", "0.21.0", "0.20.0"},
		"0.20.1": {"0.21.0", "0.20.1", "0.20.0"},
		"0.21.0": {"0.21.0", "0.20.0"},
	} {
		site := siteWithStable(t, 7, "0.21.0", "0.20.0")
		record, facts := releaseFilesOf(t, t.TempDir(), added, added)

		index := readIndexOf(t, site.run("index", "-record", record, "-release-json", facts, "-valid", "2160h"))

		if !slices.Equal(versionsOf(index), want) {
			t.Errorf("index adding %s offers %v, want %v", added, versionsOf(index), want)
		}
	}
}

func TestIndexRaisesTheSerialPastThePreviousOne(t *testing.T) {
	t.Parallel()

	future := time.Now().AddDate(10, 0, 0).Unix()
	site := siteWithStable(t, future, "0.20.0")

	index := readIndexOf(t, site.run("index", "-withdraw", "0.20.0", "-valid", "2160h"))

	if index.Serial != future+1 {
		t.Errorf("index wrote serial %d after %d, want the next one", index.Serial, future)
	}
}

func TestIndexWithdrawsAVersionAndKeepsTheOthers(t *testing.T) {
	t.Parallel()

	site := siteWithStable(t, 7, "0.21.0", "0.20.0")

	index := readIndexOf(t, site.run("index", "-withdraw", "0.21.0", "-valid", "48h"))

	if !slices.Equal(versionsOf(index), []string{"0.21.0", "0.20.0"}) || !index.Offers[0].Withdrawn ||
		index.Offers[1].Withdrawn {
		t.Errorf("index -withdraw 0.21.0 offers %+v, want 0.21.0 withdrawn and 0.20.0 kept", index.Offers)
	}
	if index.Expires.After(time.Now().Add(48 * time.Hour)) {
		t.Errorf("index expires at %v, want two days on", index.Expires)
	}
}

func TestIndexTakesAnExpiredPreviousIndex(t *testing.T) {
	t.Parallel()

	site := newTestSite(t)
	expired := indexOf("stable", 7, "0.20.0")
	expired.Expires = time.Now().AddDate(-1, 0, 0)
	site.putIndex("stable", expired, site.standby)
	record, facts := releaseFilesOf(t, t.TempDir(), "0.21.0", "0.21.0")

	index := readIndexOf(t, site.run("index", "-record", record, "-release-json", facts, "-valid", "2160h"))

	if !slices.Equal(versionsOf(index), []string{"0.21.0", "0.20.0"}) || !index.Expires.After(time.Now()) {
		t.Errorf("index after an expired index offers %v until %v, want 0.21.0 and 0.20.0 and a new expiry",
			versionsOf(index), index.Expires)
	}
}

func TestIndexRefusesAPreviousIndexNoKeyOfTheListSigned(t *testing.T) {
	t.Parallel()

	for name, signer := range map[string]func(site *testSite) ed25519.PrivateKey{
		"a key the list does not name": func(*testSite) ed25519.PrivateKey { return newTestEd25519(t) },
		"the timestamp key":            func(site *testSite) ed25519.PrivateKey { return site.timestamp },
	} {
		site := newTestSite(t)
		site.putIndex("stable", indexOf("stable", 7, "0.20.0"), signer(site))

		got := site.run("index", "-withdraw", "0.20.0", "-valid", "2160h")

		if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "no release or standby key the list names") {
			t.Errorf("index over a previous index signed by %s = %d, %q, %q, want 1 and nothing written", name,
				got.code, got.stdout, got.stderr)
		}
	}
}

func TestIndexRefusesAKeyListNamingOtherKeys(t *testing.T) {
	t.Parallel()

	site := siteWithStable(t, 7, "0.20.0")
	list := site.keyList()
	list.Keys[1].Multikey = signerMultikey(t)
	site.putKeyList(list, site.root)

	got := site.run("index", "-withdraw", "0.20.0", "-valid", "2160h")

	if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "does not name exactly the keys") {
		t.Errorf("index over a key list naming another standby = %d, %q, %q, want 1 and nothing written", got.code,
			got.stdout, got.stderr)
	}
}

func TestIndexRefusesWhatItCannotWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	record, facts := releaseFilesOf(t, dir, "0.21.0", "0.21.0")
	otherRecord, _ := releaseFilesOf(t, dir, "0.22.0", "0.22.0")
	badRecord, badMinimum := releaseFilesOf(t, dir, "0.23.0", "0.23")
	notJSON := writeFile(t, dir, "not.json", "index")
	missing := filepath.Join(dir, "missing.json")
	site := siteWithStable(t, 7, "0.20.0")
	site.putIndex("candidate", indexOf("beta", 1), site.release)
	site.put(update.IndexPath("broken"), []byte("index"), update.SignIndex([]byte("index"), site.release))
	site.putIndex("unsigned", indexOf("unsigned", 1), site.release)
	removeFile(t, site.path(update.IndexPath("unsigned")+".sig"))
	args := func(more ...string) []string { return slices.Concat(site.flags(), []string{"-valid", "2160h"}, more) }
	adding := []string{"-record", record, "-release-json", facts}
	for _, tc := range []struct {
		name string
		code int
		args []string
		text string
	}{
		{"nothing to add or withdraw", 2, args(), "index needs -record and -release-json"},
		{"both an addition and a withdrawal", 2, args(append([]string{"-withdraw", "0.20.0"}, adding...)...),
			"not both in one run"},
		{"a record without its release.json", 2, args("-record", record), "index needs -record and -release-json"},
		{"no validity", 2, slices.Concat(site.flags(), adding), "index needs a -valid above zero"},
		{"a validity of zero", 2, args(append(adding, "-valid", "0s")...), "index needs a -valid above zero"},
		{"no offer to keep", 2, args(append(adding, "-keep", "0")...), "index needs a -keep of one offer"},
		{"an argument it does not take", 2, args(append(adding, "more")...), "reads no other file"},
		{"a flag it does not know", 2, args("-offer", record), "flag provided but not defined: -offer"},
		{"the -previous it no longer takes", 2, args("-previous", notJSON, "-withdraw", "0.20.0"),
			"flag provided but not defined: -previous"},
		{"no clone", 2, without(args(adding...), "-site"), "index needs -site"},
		{"no timestamp key", 2, without(args(adding...), "-timestamp"), "index needs -site"},
		{"a root that is no key", 2, args(append(adding, "-root", "z6Mk")...), "invalid value"},
		{"a version no offer names", 1, args("-withdraw", "0.19.0"), "refused to withdraw 0.19.0"},
		{"a withdrawal with no index", 1, args("-channel", "beta", "-withdraw", "0.20.0"),
			`refused to withdraw 0.20.0, which the index of channel "beta" does not offer`},
		{"a record of another version", 1, args("-record", otherRecord, "-release-json", facts), `a record of "0.22.0"`},
		{"a minimum version it cannot read", 1, args("-record", badRecord, "-release-json", badMinimum),
			`minimum_version "0.23"`},
		{"a previous index of another channel", 1, args(append(adding, "-channel", "candidate")...),
			`the site asks for "candidate"`},
		{"a previous index that is not JSON", 1, args("-channel", "broken", "-withdraw", "0.20.0"), "not JSON"},
		{"a previous index with no signature", 1, args("-channel", "unsigned", "-withdraw", "0.20.0"),
			"reading the update site"},
		{"a release.json that is not JSON", 1, args("-record", record, "-release-json", notJSON), "not JSON"},
		{"a record that is not JSON", 1, args("-record", notJSON, "-release-json", facts), "not JSON"},
		{"a record not there", 1, args("-record", missing, "-release-json", facts), "reading the release record"},
		{"a release.json not there", 1, args("-record", record, "-release-json", missing), "reading the release.json"},
	} {
		got := runTool(append([]string{"index"}, tc.args...)...)
		if got.code != tc.code || got.stdout != "" || !strings.Contains(got.stderr, tc.text) {
			t.Errorf("index with %s = %d, %q, %q, want %d and %q said", tc.name, got.code, got.stdout, got.stderr,
				tc.code, tc.text)
		}
	}
}

// versionsFrom returns the versions 0.newest.0 down to 0.oldest.0.
func versionsFrom(newest, oldest int) []string {
	versions := make([]string, 0, newest-oldest+1)
	for minor := newest; minor >= oldest; minor-- {
		versions = append(versions, fmt.Sprintf("0.%d.0", minor))
	}
	return versions
}

func TestIndexKeepsOnlyTheNewestOffers(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		keep []string
		want []string
	}{
		"twenty by default":      {want: versionsFrom(41, 22)},
		"as many as -keep names": {keep: []string{"-keep", "2"}, want: versionsFrom(41, 40)},
	} {
		site := siteWithStable(t, 7, versionsFrom(40, 20)...)
		record, facts := releaseFilesOf(t, t.TempDir(), "0.41.0", "0.41.0")
		args := append([]string{"-record", record, "-release-json", facts, "-valid", "2160h"}, tc.keep...)

		if got := versionsOf(readIndexOf(t, site.run("index", args...))); !slices.Equal(got, tc.want) {
			t.Errorf("index keeping %s offers %v, want %v", name, got, tc.want)
		}
	}
}

func TestIndexRefusesAnIndexLargerThanASiteReads(t *testing.T) {
	t.Parallel()

	site := siteWithStable(t, 7, versionsFrom(400, 1)...)
	record, facts := releaseFilesOf(t, t.TempDir(), "0.401.0", "0.401.0")

	got := site.run("index", "-record", record, "-release-json", facts, "-keep", "1000", "-valid", "2160h")

	if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "-keep") {
		t.Errorf("index of 401 offers = %d, %q, want 1, nothing written and -keep named", got.code, got.stderr)
	}
}

func TestIndexWritesTheChannelItIsNamed(t *testing.T) {
	t.Parallel()

	site := siteWithStable(t, 7, "0.20.0")
	record, facts := releaseFilesOf(t, t.TempDir(), "0.21.0-rc.1", "0.21.0-rc.1")

	index := readIndexOf(t, site.run("index", "-channel", "beta", "-record", record, "-release-json", facts,
		"-valid", "2160h"))

	if index.Channel != "beta" || !slices.Equal(versionsOf(index), []string{"0.21.0-rc.1"}) {
		t.Errorf("index -channel beta wrote the channel %q offering %v, want beta offering 0.21.0-rc.1 alone",
			index.Channel, versionsOf(index))
	}
}
