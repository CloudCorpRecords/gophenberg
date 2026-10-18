// SPDX-License-Identifier: Apache-2.0

package main

import (
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

// previousIndex writes an index of serial offering the versions in order to dir, and returns its path.
func previousIndex(t *testing.T, dir string, serial int64, versions ...string) string {
	t.Helper()
	index := update.Index{Schema: 1, Channel: "stable", Serial: serial, Expires: time.Now().AddDate(0, 0, 3)}
	for _, version := range versions {
		index.Offers = append(index.Offers, update.Offer{
			Rollout: 100, MinimumVersion: version, PostgreSQL: 15, Kits: []string{"0.9.0"},
			Notes: "https://example.com/releases/v" + version, Release: json.RawMessage(fmt.Sprintf(`{"version":%q}`, version)),
		})
	}
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("writing the previous index: %v", err)
	}
	return writeFile(t, dir, "previous.json", string(body))
}

func TestIndexStartsAChannelWithItsFirstRelease(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	record, facts := releaseFilesOf(t, dir, "0.20.0", "0.19.0")
	before := time.Now()

	index := readIndexOf(t, runTool("index", "-record", record, "-release-json", facts))

	if index.Schema != 1 || index.Channel != "stable" || index.Serial < before.Unix() || index.Serial > time.Now().Unix() {
		t.Errorf("index wrote %+v, want schema 1, channel stable and the Unix time of the run as its serial", index)
	}
	if index.Expires.Before(before.Add(720 * time.Hour).Truncate(time.Second)) {
		t.Errorf("index expires at %v, want 720 hours after the run", index.Expires)
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
		dir := t.TempDir()
		previous := previousIndex(t, dir, 7, "0.21.0", "0.20.0")
		record, facts := releaseFilesOf(t, dir, added, added)

		index := readIndexOf(t, runTool("index", "-previous", previous, "-record", record, "-release-json", facts))

		if !slices.Equal(versionsOf(index), want) {
			t.Errorf("index adding %s offers %v, want %v", added, versionsOf(index), want)
		}
	}
}

func TestIndexRaisesTheSerialPastThePreviousOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	future := time.Now().AddDate(10, 0, 0).Unix()
	previous := previousIndex(t, dir, future, "0.20.0")

	index := readIndexOf(t, runTool("index", "-previous", previous, "-withdraw", "0.20.0"))

	if index.Serial != future+1 {
		t.Errorf("index wrote serial %d after %d, want the next one", index.Serial, future)
	}
}

func TestIndexWithdrawsAVersionAndKeepsTheOthers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	previous := previousIndex(t, dir, 7, "0.21.0", "0.20.0")

	index := readIndexOf(t, runTool("index", "-previous", previous, "-withdraw", "0.21.0", "-valid", "48h"))

	if !slices.Equal(versionsOf(index), []string{"0.21.0", "0.20.0"}) || !index.Offers[0].Withdrawn ||
		index.Offers[1].Withdrawn {
		t.Errorf("index -withdraw 0.21.0 offers %+v, want 0.21.0 withdrawn and 0.20.0 kept", index.Offers)
	}
	if index.Expires.After(time.Now().Add(48 * time.Hour)) {
		t.Errorf("index expires at %v, want two days on", index.Expires)
	}
}

func TestIndexRefusesWhatItCannotWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	record, facts := releaseFilesOf(t, dir, "0.21.0", "0.21.0")
	otherRecord, _ := releaseFilesOf(t, dir, "0.22.0", "0.22.0")
	badRecord, badMinimum := releaseFilesOf(t, dir, "0.23.0", "0.23")
	previous := previousIndex(t, dir, 7, "0.20.0")
	notJSON := writeFile(t, dir, "not.json", "index")
	beta := writeFile(t, dir, "beta.json", `{"schema":1,"channel":"beta","serial":1,"offers":[]}`)
	missing := filepath.Join(dir, "missing.json")
	for _, tc := range []struct {
		name string
		code int
		args []string
	}{
		{"nothing to add or withdraw", 2, []string{"-previous", previous}},
		{"both an addition and a withdrawal", 2, []string{"-previous", previous, "-withdraw", "0.20.0",
			"-record", record, "-release-json", facts}},
		{"a record without its release.json", 2, []string{"-record", record}},
		{"a withdrawal with no index", 2, []string{"-withdraw", "0.20.0"}},
		{"a validity of zero", 2, []string{"-record", record, "-release-json", facts, "-valid", "0s"}},
		{"no offer to keep", 2, []string{"-record", record, "-release-json", facts, "-keep", "0"}},
		{"an argument it does not take", 2, []string{"-record", record, "-release-json", facts, "more"}},
		{"a flag it does not know", 2, []string{"-offer", record}},
		{"a version no offer names", 1, []string{"-previous", previous, "-withdraw", "0.19.0"}},
		{"a record of another version", 1, []string{"-record", otherRecord, "-release-json", facts}},
		{"a minimum version it cannot read", 1, []string{"-record", badRecord, "-release-json", badMinimum}},
		{"a previous index of another channel", 1, []string{"-previous", beta, "-record", record,
			"-release-json", facts}},
		{"a previous index that is not JSON", 1, []string{"-previous", notJSON, "-withdraw", "0.20.0"}},
		{"a release.json that is not JSON", 1, []string{"-record", record, "-release-json", notJSON}},
		{"a record that is not JSON", 1, []string{"-record", notJSON, "-release-json", facts}},
		{"a previous index not there", 1, []string{"-previous", missing, "-withdraw", "0.20.0"}},
		{"a record not there", 1, []string{"-record", missing, "-release-json", facts}},
		{"a release.json not there", 1, []string{"-record", record, "-release-json", missing}},
	} {
		if got := runTool(append([]string{"index"}, tc.args...)...); got.code != tc.code {
			t.Errorf("index with %s = %d, %q, want %d", tc.name, got.code, got.stderr, tc.code)
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
		dir := t.TempDir()
		previous := previousIndex(t, dir, 7, versionsFrom(40, 20)...)
		record, facts := releaseFilesOf(t, dir, "0.41.0", "0.41.0")
		args := append([]string{"index", "-previous", previous, "-record", record, "-release-json", facts}, tc.keep...)

		if got := versionsOf(readIndexOf(t, runTool(args...))); !slices.Equal(got, tc.want) {
			t.Errorf("index keeping %s offers %v, want %v", name, got, tc.want)
		}
	}
}

func TestIndexRefusesAnIndexLargerThanASiteReads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	previous := previousIndex(t, dir, 7, versionsFrom(400, 1)...)
	record, facts := releaseFilesOf(t, dir, "0.401.0", "0.401.0")

	got := runTool("index", "-previous", previous, "-record", record, "-release-json", facts, "-keep", "1000")

	if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "-keep") {
		t.Errorf("index of 401 offers = %d, %q, want 1, nothing written and -keep named", got.code, got.stderr)
	}
}

func TestIndexWritesTheChannelItIsNamed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	record, facts := releaseFilesOf(t, dir, "0.21.0-rc.1", "0.21.0-rc.1")

	index := readIndexOf(t, runTool("index", "-channel", "beta", "-record", record, "-release-json", facts))

	if index.Channel != "beta" {
		t.Errorf("index -channel beta wrote the channel %q, want beta", index.Channel)
	}
}
