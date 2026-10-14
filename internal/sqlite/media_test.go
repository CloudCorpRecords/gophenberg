// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/media/mediatest"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// otherWriterHold is how long the busy test keeps another writer in the file before it lets go.
const otherWriterHold = 200 * time.Millisecond

// longestLikePattern is the longest pattern, in bytes, that SQLite's LIKE reads.
const longestLikePattern = 50000

// uploaded is the stamp every media item these tests store carries.
var uploaded = time.Date(2026, time.August, 10, 9, 30, 0, 0, time.UTC)

// mediaFault fails one statement and calls the store method that runs it on an item stored before.
type mediaFault struct {
	name, statement, prefix string
	call                    func(ctx context.Context, store *sqlite.MediaStore, stored media.Media) error
}

// mediaFaults are the failure cases, one for each statement a media store method runs.
var mediaFaults = []mediaFault{
	{"a create", db.CreateMedia, "sqlite: create media: ",
		func(ctx context.Context, store *sqlite.MediaStore, stored media.Media) error {
			return ignoreValue(store.Create(ctx, stored))
		}},
	{"a read", db.GetMedia, "sqlite: get media: ",
		func(ctx context.Context, store *sqlite.MediaStore, stored media.Media) error {
			return ignoreValue(store.ByID(ctx, stored.ID))
		}},
	{"a read of several", db.ListMediaByIDs, "sqlite: list media by ids: ",
		func(ctx context.Context, store *sqlite.MediaStore, stored media.Media) error {
			return ignoreValue(store.ByIDs(ctx, []int64{stored.ID}))
		}},
	{"a check of the saved files", db.ListSavedMediaFiles, "sqlite: list saved media files: ",
		func(ctx context.Context, store *sqlite.MediaStore, stored media.Media) error {
			return ignoreValue(store.Saved(ctx, []string{stored.File}))
		}},
	{"a count", db.CountMedia, "sqlite: count media: ", listFirstPage},
	{"a listing", db.ListMedia, "sqlite: list media: ", listFirstPage},
	{"an edit", db.UpdateMedia, "sqlite: update media: ",
		func(ctx context.Context, store *sqlite.MediaStore, stored media.Media) error {
			return ignoreValue(store.Update(ctx, stored, stored.UpdatedAt))
		}},
	{"the read that checks a stale edit", db.GetMedia, "sqlite: get media: ",
		func(ctx context.Context, store *sqlite.MediaStore, stored media.Media) error {
			return ignoreValue(store.Update(ctx, stored, stored.UpdatedAt.Add(-time.Second)))
		}},
	{"a delete", db.DeleteMedia, "sqlite: delete media: ",
		func(ctx context.Context, store *sqlite.MediaStore, stored media.Media) error {
			return ignoreValue(store.Delete(ctx, stored.ID))
		}},
}

// ignoreValue drops the value of a call and keeps its error.
func ignoreValue[T any](_ T, err error) error {
	return err
}

// listFirstPage lists the first ten items of store and keeps the error.
func listFirstPage(ctx context.Context, store *sqlite.MediaStore, _ media.Media) error {
	_, _, err := store.List(ctx, media.Filter{Page: 1, PerPage: 10})
	return err
}

// stampedImage returns an image of author under file and title, stamped at the fixed upload time.
func stampedImage(t *testing.T, file, title string, author uuid.UUID) media.Media {
	t.Helper()
	item := mediatest.MustImage(t, file, title, author)
	item.CreatedAt, item.UpdatedAt = uploaded, uploaded
	return item
}

// storedImage stores an image of a new author through store on handle and returns it as stored.
func storedImage(t *testing.T, handle *sql.DB, store *sqlite.MediaStore) media.Media {
	t.Helper()
	author := addAuthor(t, handle, mediatest.DefaultAuthor)
	return mediatest.MustCreate(t, store, stampedImage(t, "2026/08/harbor.jpg", "harbor", author))
}

// storedTitles stores an image under each title through a fresh store, all of one author and upload moment.
func storedTitles(t *testing.T, titles ...string) *sqlite.MediaStore {
	t.Helper()
	handle := migrated(t)
	store := sqlite.NewMediaStore(handle)
	author := addAuthor(t, handle, mediatest.DefaultAuthor)
	for i, title := range titles {
		file := "2026/08/item-" + string(rune('a'+i)) + ".jpg"
		mediatest.MustCreate(t, store, stampedImage(t, file, title, author))
	}
	return store
}

// listedTitlesOf returns the titles store lists for the filter on its first page of ten.
func listedTitlesOf(t *testing.T, store *sqlite.MediaStore, f media.Filter) []string {
	t.Helper()
	f.Page, f.PerPage = 1, 10
	items, total, err := store.List(t.Context(), f)
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != len(items) {
		t.Errorf("List() total = %d, want the %d rows it lists", total, len(items))
	}
	titles := make([]string, len(items))
	for i, item := range items {
		titles[i] = item.Title
	}
	return titles
}

// mustExecOn runs statement on conn and fails t when it errs.
func mustExecOn(t *testing.T, conn *sql.Conn, statement string) {
	t.Helper()
	if _, err := conn.ExecContext(t.Context(), statement); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
}

func TestMediaStoreKeepsAnAuthorWhoHoldsMediaSoTheListAndItsCountAgree(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewMediaStore(handle)
	stored := storedImage(t, handle, store)

	err := classed(t, handle, "DELETE FROM auth_users WHERE id = ?", stored.AuthorID)

	if !errors.Is(err, dbkit.ErrForeignKey) {
		t.Errorf("deleting the author of a stored item error = %v, want dbkit.ErrForeignKey", err)
	}
	listed, total, err := store.List(t.Context(), media.Filter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 1 || len(listed) != 1 || listed[0].AuthorName != mediatest.DefaultAuthor {
		t.Errorf("List() = %+v with total %d, want the item listed beside its author and counted once", listed, total)
	}
}

func TestMediaStoreReportsAStatementThatFails(t *testing.T) {
	t.Parallel()

	for _, tc := range mediaFaults {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handle, faults := faultyHandle(t)
			store := sqlite.NewMediaStore(handle)
			stored := storedImage(t, handle, store)
			faults.FailStatement(tc.statement, errPlanted)

			err := tc.call(t.Context(), store, stored)

			wantPlanted(t, err, tc.prefix)
		})
	}
}

func TestMediaStoreSurfacesSizesOfTheWrongShape(t *testing.T) {
	t.Parallel()

	for _, planted := range []struct{ name, sizes string }{
		{"an array", `[]`},
		{"a sub-size that is a number", `{"thumbnail":1}`},
	} {
		t.Run(planted.name, func(t *testing.T) {
			t.Parallel()
			handle := migrated(t)
			store := sqlite.NewMediaStore(handle)
			stored := storedImage(t, handle, store)
			mustExec(t, handle, "UPDATE core_media SET sizes = ? WHERE id = ?", planted.sizes, stored.ID)

			readErr := ignoreValue(store.ByID(t.Context(), stored.ID))
			severalErr := ignoreValue(store.ByIDs(t.Context(), []int64{stored.ID}))
			listErr := listFirstPage(t.Context(), store, stored)

			for _, read := range []struct {
				err    error
				prefix string
			}{
				{readErr, "sqlite: get media: "},
				{severalErr, "sqlite: list media by ids: "},
				{listErr, "sqlite: list media: "},
			} {
				text := fmt.Sprint(read.err)
				if !strings.HasPrefix(text, read.prefix) || !strings.Contains(text, "columns: read the sizes: ") {
					t.Errorf("error = %v, want the sizes it cannot read reported under %q", read.err, read.prefix)
				}
			}
		})
	}
}

func TestMediaStoreListsNothingForAPageSizeBelowOne(t *testing.T) {
	t.Parallel()

	for _, size := range []int{0, -1} {
		t.Run(fmt.Sprintf("a page size of %d", size), func(t *testing.T) {
			t.Parallel()
			handle := migrated(t)
			store := sqlite.NewMediaStore(handle)
			storedImage(t, handle, store)

			listed, total, err := store.List(t.Context(), media.Filter{Page: 1, PerPage: size})

			if err != nil || len(listed) != 0 || total != 1 {
				t.Errorf("List() with a page size of %d = %d items with total %d and error %v, "+
					"want nothing listed and the one stored item counted", size, len(listed), total, err)
			}
		})
	}
}

func TestMediaStoreListsNothingOnAPageBeyondWhatAnOffsetHolds(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewMediaStore(handle)
	storedImage(t, handle, store)

	listed, total, err := store.List(t.Context(), media.Filter{Page: math.MaxInt, PerPage: 100})

	if err != nil || len(listed) != 0 || total != 1 {
		t.Errorf("List() on the last page an int names = %d items with total %d and error %v, "+
			"want nothing listed and the one stored item counted", len(listed), total, err)
	}
}

func TestMediaStoreListsNothingForASearchLongerThanAnyPattern(t *testing.T) {
	t.Parallel()

	store := storedTitles(t, "harbor")

	got := listedTitlesOf(t, store, media.Filter{Search: strings.Repeat("a", longestLikePattern+1)})

	if len(got) != 0 {
		t.Errorf("List() searching %d bytes = %q, want nothing, as no stored text is that long",
			longestLikePattern+1, got)
	}
}

func TestMediaStoreReadsANulByteInASearchAsText(t *testing.T) {
	t.Parallel()

	store := storedTitles(t, "Old town plaza")

	got := listedTitlesOf(t, store, media.Filter{Search: "a\x00"})

	if len(got) != 0 {
		t.Errorf("List(%q) = %q, want nothing, as no stored text holds a NUL byte", "a\x00", got)
	}
}

func TestMediaStoreSortsTitlesByTheirFoldedAccentedLetters(t *testing.T) {
	t.Parallel()

	store := storedTitles(t, "Écru", "éclair")

	got := listedTitlesOf(t, store, media.Filter{OrderBy: media.OrderByTitle, Order: media.OrderAsc})

	if want := []string{"éclair", "Écru"}; !slices.Equal(got, want) {
		t.Errorf("List(title asc) = %q, want %q", got, want)
	}
}

func TestMediaStoreMatchesAContentTypePrefixHoldingAnUnderscoreAsText(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewMediaStore(handle)
	item, err := media.New("2026/08/plan.doc", "plan", "application/vnd.example_doc",
		addAuthor(t, handle, mediatest.DefaultAuthor))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	item.CreatedAt, item.UpdatedAt = uploaded, uploaded
	mediatest.MustCreate(t, store, item)

	got := listedTitlesOf(t, store, media.Filter{Mimes: []string{"application/vnd.example_"}})

	if want := []string{"plan"}; !slices.Equal(got, want) {
		t.Errorf("List(application/vnd.example_) = %q, want %q", got, want)
	}
}

func TestMediaStoreNamesASavedFileOnlyInItsExactCase(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewMediaStore(handle)
	storedImage(t, handle, store)

	saved, err := store.Saved(t.Context(), []string{"2026/08/HARBOR.jpg"})

	if err != nil || !maps.Equal(saved, map[string]bool{}) {
		t.Errorf("Saved(2026/08/HARBOR.jpg) = %v, error %v, want nothing, as only 2026/08/harbor.jpg is stored",
			saved, err)
	}
}

func TestMediaStoreReadsNoIdentitiesWithoutAskingTheFile(t *testing.T) {
	t.Parallel()

	handle, faults := faultyHandle(t)
	store := sqlite.NewMediaStore(handle)
	faults.FailStatement(db.ListMediaByIDs, errPlanted)

	listed, err := store.ByIDs(t.Context(), nil)

	if err != nil || listed != nil {
		t.Errorf("ByIDs(nil) = %v, error %v, want nil and no read of the file", listed, err)
	}
}

func TestMediaCreateBehindAnotherWriterWaitsForItAndReportsTheWait(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewMediaStore(handle)
	item := stampedImage(t, "2026/08/harbor.jpg", "harbor", addAuthor(t, handle, mediatest.DefaultAuthor))
	writer := otherWriter(t, handle)
	mustExecOn(t, writer, "BEGIN IMMEDIATE")
	done := make(chan struct{})
	var createErr error
	var landed time.Time
	started := time.Now()

	go func() {
		defer close(done)
		_, createErr = store.Create(t.Context(), item)
		landed = time.Now()
	}()
	time.Sleep(otherWriterHold)
	select {
	case <-done:
		t.Fatalf("Create() returned %v while another writer held the file, want it to wait", createErr)
	default:
	}
	mustExecOn(t, writer, "COMMIT")
	released := time.Now()
	<-done

	if createErr != nil {
		t.Fatalf("Create() behind another writer error = %v, want nil once the writer let go", createErr)
	}
	if stored := count(t, handle, "SELECT count(*) FROM core_media"); stored != 1 {
		t.Errorf("the file holds %d items, want the one stored after the wait", stored)
	}
	t.Logf("Create() waited %v in all and landed %v after the other writer let go",
		landed.Sub(started), landed.Sub(released))
}

func TestMediaCreateHeldPastTheBusyTimeoutAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle := impatientHandle(t)
	store := sqlite.NewMediaStore(handle)
	item := stampedImage(t, "2026/08/harbor.jpg", "harbor", addAuthor(t, handle, mediatest.DefaultAuthor))
	holdWrites(t, handle)

	_, err := store.Create(t.Context(), item)

	if !errors.Is(err, dbkit.ErrBusy) || !strings.HasPrefix(fmt.Sprint(err), "sqlite: create media: ") {
		t.Errorf("Create() behind another writer error = %v, want dbkit.ErrBusy under the create", err)
	}
}
