// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
)

// readNote returns the timestamp file a run of timestamp printed.
func readNote(t *testing.T, got runResult) update.Timestamp {
	t.Helper()
	var note update.Timestamp
	if err := json.Unmarshal([]byte(got.stdout), &note); err != nil {
		t.Fatalf("timestamp = %d, %q, %q, want one timestamp file: %v", got.code, got.stdout, got.stderr, err)
	}
	return note
}

func TestTimestampNamesTheKeyListAndEachIndexASiteTakes(t *testing.T) {
	t.Parallel()

	site := newTestSite(t)
	site.putIndex("stable", indexOf("stable", 7, "0.21.0"), site.release)
	site.putIndex("beta", indexOf("beta", 3, "0.22.0-rc.1"), site.standby)
	before := time.Now()

	got := site.run("timestamp", "-valid", "168h")

	after := time.Now()
	note := readNote(t, got)
	want := map[string]int64{"stable": 7, "beta": 3}
	if got.code != 0 || note.Schema != 1 || note.Keys != 5 || !maps.Equal(note.Indexes, want) {
		t.Errorf("timestamp = %d, %q, %+v, want schema 1, the key list 5, the stable index 7 and the beta index 3",
			got.code, got.stderr, note)
	}
	week := 168 * time.Hour
	if note.Expires.Before(before.Add(week).Truncate(time.Second)) || note.Expires.After(after.Add(week)) {
		t.Errorf("timestamp wrote the expiry %v, want 168 hours after the run", note.Expires)
	}
	if note.Expires.Location() != time.UTC || note.Expires.Nanosecond() != 0 {
		t.Errorf("timestamp wrote the expiry %v, want it in UTC to the second", note.Expires)
	}
}

func TestTimestampWritesAnEmptyListOfIndexesBeforeTheFirstIndex(t *testing.T) {
	t.Parallel()

	site := newTestSite(t)

	got := site.run("timestamp", "-valid", "168h")

	if got.code != 0 || !strings.Contains(got.stdout, `"indexes": {}`) || readNote(t, got).Keys != 5 {
		t.Errorf("timestamp before the first index = %d, %q, %q, want 0 and no index named", got.code, got.stdout,
			got.stderr)
	}
}

func TestTimestampTakesAnIndexThatOffersTheNewestVersion(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		index  update.Index
		newest string
	}{
		"an offer of the newest version":          {indexOf("stable", 7, "0.21.0", "0.20.0"), "stable=0.21.0"},
		"a withdrawn offer of the newest version": {withdrawnFirst(indexOf("stable", 7, "0.21.0")), "stable=0.21.0"},
	} {
		site := newTestSite(t)
		site.putIndex("stable", tc.index, site.release)

		got := site.run("timestamp", "-valid", "168h", "-newest", tc.newest)

		if note := readNote(t, got); got.code != 0 || !maps.Equal(note.Indexes, map[string]int64{"stable": 7}) {
			t.Errorf("timestamp with %s = %d, %q, %+v, want the stable index 7 named", name, got.code, got.stderr, note)
		}
	}
}

// withdrawnFirst returns index with its first offer withdrawn.
func withdrawnFirst(index update.Index) update.Index {
	index.Offers[0].Withdrawn = true
	return index
}

func TestTimestampLeavesOutAnIndexASiteRefuses(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		put    func(site *testSite)
		newest []string
		text   string
	}{
		"an expired index": {put: func(site *testSite) {
			expired := indexOf("beta", 3, "0.22.0-rc.1")
			expired.Expires = time.Now().Add(-time.Hour)
			site.putIndex("beta", expired, site.release)
		}, text: "which expired at"},
		"an index the timestamp key signed": {put: func(site *testSite) {
			site.putIndex("beta", indexOf("beta", 3, "0.22.0-rc.1"), site.timestamp)
		}, text: "no release or standby key the list names signed it"},
		"an index of another channel than its file names": {put: func(site *testSite) {
			site.putIndex("beta", indexOf("stable", 3, "0.22.0"), site.release)
		}, text: `the site asks for "beta"`},
		"an index with no signature": {put: func(site *testSite) {
			site.putIndex("beta", indexOf("beta", 3, "0.22.0-rc.1"), site.release)
			removeFile(t, site.path(update.IndexPath("beta")+".sig"))
		}, text: "reading the update site"},
		"an index that is not JSON": {put: func(site *testSite) {
			site.put(update.IndexPath("beta"), []byte("index"), update.SignIndex([]byte("index"), site.release))
		}, text: "which is not JSON"},
		"an index without the newest version": {put: func(site *testSite) {
			site.putIndex("beta", indexOf("beta", 3, "0.22.0-rc.1"), site.release)
		}, newest: []string{"-newest", "beta=0.22.0-rc.2"}, text: "which does not offer 0.22.0-rc.2"},
		"no index of the channel -newest names": {put: func(*testSite) {}, newest: []string{"-newest", "beta=0.22.0-rc.1"},
			text: "the update site holds no index to offer 0.22.0-rc.1"},
	} {
		site := newTestSite(t)
		site.putIndex("stable", indexOf("stable", 7, "0.21.0"), site.release)
		tc.put(site)

		got := site.run("timestamp", append([]string{"-valid", "168h"}, tc.newest...)...)

		if note := readNote(t, got); got.code != 1 || !maps.Equal(note.Indexes, map[string]int64{"stable": 7}) {
			t.Errorf("timestamp beside %s = %d, %+v, want 1 and the stable index 7 named alone", name, got.code, note)
		}
		if !strings.Contains(got.stderr, `left the channel "beta" out`) || !strings.Contains(got.stderr, tc.text) {
			t.Errorf("timestamp beside %s said %q, want the beta channel left out because %q", name, got.stderr,
				tc.text)
		}
	}
}

// removeFile removes the file at path, failing the test when it cannot.
func removeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("removing %s: %v", path, err)
	}
}

func TestTimestampNamesEveryChannelItLeavesOut(t *testing.T) {
	t.Parallel()

	site := newTestSite(t)
	site.putIndex("beta", indexOf("beta", 3, "0.22.0-rc.1"), site.timestamp)
	site.putIndex("stable", indexOf("stable", 7, "0.21.0"), site.timestamp)

	got := site.run("timestamp", "-valid", "168h")

	if got.code != 1 || !strings.Contains(got.stdout, `"indexes": {}`) {
		t.Errorf("timestamp with no index a site takes = %d, %q, want 1 and no index named", got.code, got.stdout)
	}
	for _, channel := range []string{`"beta"`, `"stable"`} {
		if !strings.Contains(got.stderr, "left the channel "+channel+" out") {
			t.Errorf("timestamp said %q, want the channel %s named as left out", got.stderr, channel)
		}
	}
}

func TestTimestampTakesTheKeysInAnyOrder(t *testing.T) {
	t.Parallel()

	site := newTestSite(t)
	list := site.keyList()
	slices.Reverse(list.Keys)
	site.putKeyList(list, site.root)

	if got := site.run("timestamp", "-valid", "168h"); got.code != 0 {
		t.Errorf("timestamp over a key list in another order = %d, %q, want 0", got.code, got.stderr)
	}
}

func TestTimestampRefusesAKeyListNamingOtherKeys(t *testing.T) {
	t.Parallel()

	for name, change := range map[string]func(list *update.KeyList){
		"another release key": func(list *update.KeyList) { list.Keys[0].Multikey = signerMultikey(t) },
		"the release key and the standby swapped": func(list *update.KeyList) {
			list.Keys[0].Role, list.Keys[1].Role = update.RoleStandby, update.RoleRelease
		},
		"no standby": func(list *update.KeyList) { list.Keys = slices.Delete(list.Keys, 1, 2) },
		"a fourth key": func(list *update.KeyList) {
			list.Keys = append(list.Keys, update.ListedKey{Role: "mirror", Multikey: signerMultikey(t)})
		},
	} {
		site := newTestSite(t)
		list := site.keyList()
		change(&list)
		site.putKeyList(list, site.root)

		got := site.run("timestamp", "-valid", "168h")

		if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "does not name exactly the keys") {
			t.Errorf("timestamp over a key list with %s = %d, %q, %q, want 1, nothing written and the keys named",
				name, got.code, got.stdout, got.stderr)
		}
	}
}

func TestTimestampRefusesAKeyListASiteRefuses(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		arrange func(site *testSite) []string
		text    string
	}{
		"a key list another root signed": {func(site *testSite) []string {
			site.putKeyList(site.keyList(), newTestP256(t))
			return site.flags()
		}, "no trusted root signed it"},
		"the official roots": {func(site *testSite) []string {
			return without(site.flags(), "-root")
		}, "no trusted root signed it"},
		"an expired key list": {func(site *testSite) []string {
			list := site.keyList()
			list.Expires = time.Now().Add(-time.Hour)
			site.putKeyList(list, site.root)
			return site.flags()
		}, "which expired at"},
		"no key list": {func(site *testSite) []string {
			removeFile(t, site.path(update.KeyListPath))
			return site.flags()
		}, "reading the update site"},
		"a -release that is no Ed25519 key": {func(site *testSite) []string {
			return append(site.flags(), "-release", p256Multikey(t))
		}, "refused -release"},
	} {
		site := newTestSite(t)
		args := tc.arrange(site)

		got := runTool(slices.Concat([]string{"timestamp"}, args, []string{"-valid", "168h"})...)

		if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, tc.text) {
			t.Errorf("timestamp with %s = %d, %q, %q, want 1, nothing written and %q said", name, got.code,
				got.stdout, got.stderr, tc.text)
		}
	}
}

func TestTimestampReportsAnIndexFolderItCannotRead(t *testing.T) {
	t.Parallel()

	site := newTestSite(t)
	writeFile(t, site.path("v1"), "core", "a file where the indexes go")

	got := site.run("timestamp", "-valid", "168h")

	if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "reading the update site") {
		t.Errorf("timestamp over a file where the indexes go = %d, %q, %q, want 1 and nothing written", got.code,
			got.stdout, got.stderr)
	}
}

func TestTimestampExitsTwoWhenMisused(t *testing.T) {
	t.Parallel()

	site := newTestSite(t)
	flags := site.flags()
	valid := []string{"-valid", "168h"}
	twoRoots := rootMultikey(t, site.root) + "," + rootMultikey(t, newTestP256(t))
	for name, args := range map[string][]string{
		"no clone":                            slices.Concat(without(flags, "-site"), valid),
		"no release key":                      slices.Concat(without(flags, "-release"), valid),
		"no standby":                          slices.Concat(without(flags, "-standby"), valid),
		"no timestamp key":                    slices.Concat(without(flags, "-timestamp"), valid),
		"no validity":                         flags,
		"a validity of zero":                  slices.Concat(flags, []string{"-valid", "0s"}),
		"a file to read":                      slices.Concat(flags, valid, []string{"timestamp.json"}),
		"a newest version with no channel":    slices.Concat(flags, valid, []string{"-newest", "0.20.1"}),
		"a newest version that is no version": slices.Concat(flags, valid, []string{"-newest", "stable=0.20"}),
		"a channel named twice as newest": slices.Concat(flags, valid,
			[]string{"-newest", "stable=0.20.1", "-newest", "stable=0.20.2"}),
		"a root that is no key":   slices.Concat(flags, valid, []string{"-root", "z6Mk"}),
		"a blank root":            slices.Concat(flags, valid, []string{"-root", ""}),
		"two roots in one -root":  slices.Concat(flags, valid, []string{"-root", twoRoots}),
		"a flag it does not know": slices.Concat(flags, valid, []string{"-previous", "stable.json"}),
	} {
		if got := runTool(append([]string{"timestamp"}, args...)...); got.code != 2 || got.stdout != "" {
			t.Errorf("timestamp with %s = %d, %q, %q, want 2 and nothing written", name, got.code, got.stdout,
				got.stderr)
		}
	}
}
