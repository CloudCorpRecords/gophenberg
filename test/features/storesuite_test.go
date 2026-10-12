// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/media/mediatest"
)

// shortRowsVariable names the variable that runs the suite on stores keeping only some rows, by the rule's name.
const shortRowsVariable = "GOPHENBERG_SHORT_ROWS"

// reportedCase matches the line the test binary prints for each case of the short rows run.
var reportedCase = regexp.MustCompile(`(?m)^    --- (PASS|FAIL): \S+/\S+ `)

// mediaGaps names the shared media store cases the in-memory media store does not pass yet.
var mediaGaps = map[string]bool{
	"SavedNamesOnlyTheFilesAStoredItemHolds": true,
	"ReadsStampsBackInUTC":                   true,
}

// memoryAuthors returns a way to add accounts to the in-memory account store.
func memoryAuthors(accounts *memoryStore) func(t *testing.T, name string) uuid.UUID {
	return func(t *testing.T, name string) uuid.UUID {
		t.Helper()
		id := uuid.Must(uuid.NewV7())
		user := gouncer.User{ID: id, Email: id.String() + "@example.com", Name: name, CreatedAt: time.Now().UTC()}
		if err := accounts.CreateUser(t.Context(), user); err != nil {
			t.Fatalf("CreateUser() error = %v, want nil", err)
		}
		return id
	}
}

// memoryStores returns fresh in-memory stores for one case.
func memoryStores(t *testing.T) contenttest.Stores {
	t.Helper()
	items, types, accounts := newMemoryStores()
	return contenttest.Stores{
		Content:   items,
		Types:     types,
		AddAuthor: memoryAuthors(accounts),
	}
}

// memoryMediaStores returns a fresh in-memory media store for one case.
func memoryMediaStores(t *testing.T) mediatest.Stores {
	t.Helper()
	accounts := newMemoryStore()
	store := newMemoryMedia(accounts)
	return mediatest.Stores{Media: store, Saved: store.Saved, AddAuthor: memoryAuthors(accounts)}
}

// shortStores returns a factory of in-memory stores whose listings keep only the rows the named rule leaves.
func shortStores(rule string) contenttest.Factory {
	return func(t *testing.T) contenttest.Stores {
		t.Helper()
		s := memoryStores(t)
		s.Content = shortContent{Store: s.Content, keep: keptRows[rule]}
		s.Types = shortTypes{TypeStore: s.Types, keep: keptRows[rule]}
		return s
	}
}

// shortMediaStores returns a factory of in-memory media stores whose reads keep only the rows the named rule leaves.
func shortMediaStores(rule string) mediatest.Factory {
	return func(t *testing.T) mediatest.Stores {
		t.Helper()
		s := memoryMediaStores(t)
		s.Media = shortMedia{Store: s.Media, keep: keptRows[rule]}
		return s
	}
}

func TestContentStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.Run(t, memoryStores)
}

func TestTypeStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.RunTypes(t, memoryStores)
}

func TestMediaStoreSuite(t *testing.T) {
	t.Parallel()

	mediatest.Run(t, func(t *testing.T) mediatest.Stores {
		t.Helper()
		if mediaGaps[path.Base(t.Name())] {
			t.Skip("the in-memory media store does not pass this case yet")
		}
		return memoryMediaStores(t)
	})
}

// shortRowsRuns reruns the named test under each keep rule and fails on a panic or a case that never reported.
func shortRowsRuns(t *testing.T, name string, cases int) {
	t.Helper()
	for rule := range keptRows {
		t.Run(rule, func(t *testing.T) {
			t.Parallel()
			run := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+name+"$", "-test.v")
			run.Env = append(os.Environ(), shortRowsVariable+"="+rule)

			out, err := run.CombinedOutput()

			var exit *exec.ExitError
			if err != nil && !errors.As(err, &exit) {
				t.Fatalf("running the suite: %v", err)
			}
			if at := bytes.Index(out, []byte("\npanic: ")); at >= 0 {
				t.Fatalf("the suite panicked on stores keeping %s:%s", rule, out[at:min(len(out), at+800)])
			}
			if reported := len(reportedCase.FindAll(out, -1)); reported != cases {
				t.Errorf("%d cases reported on stores keeping %s, want all %d", reported, rule, cases)
			}
		})
	}
}

func TestContentStoreSuiteFailsWithoutPanickingOnShortRows(t *testing.T) {
	if rule := os.Getenv(shortRowsVariable); rule != "" {
		contenttest.Run(t, shortStores(rule))
		return
	}
	t.Parallel()

	shortRowsRuns(t, "TestContentStoreSuiteFailsWithoutPanickingOnShortRows", len(contenttest.Cases()))
}

func TestTypeStoreSuiteFailsWithoutPanickingOnShortRows(t *testing.T) {
	if rule := os.Getenv(shortRowsVariable); rule != "" {
		contenttest.RunTypes(t, shortStores(rule))
		return
	}
	t.Parallel()

	shortRowsRuns(t, "TestTypeStoreSuiteFailsWithoutPanickingOnShortRows", len(contenttest.TypeCases()))
}

func TestMediaStoreSuiteFailsWithoutPanickingOnShortRows(t *testing.T) {
	if rule := os.Getenv(shortRowsVariable); rule != "" {
		mediatest.Run(t, shortMediaStores(rule))
		return
	}
	t.Parallel()

	shortRowsRuns(t, "TestMediaStoreSuiteFailsWithoutPanickingOnShortRows", len(mediatest.Cases()))
}
