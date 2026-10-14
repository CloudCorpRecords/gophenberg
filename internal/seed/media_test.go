// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/media"
)

// recordingLibrary remembers what it was asked to ingest, finish and remove, stamping uploads at now when set.
type recordingLibrary struct {
	ingested []string
	finished []string
	removed  []string
	err      error
	now      time.Time
}

// Ingest records the upload and returns the media item it stands for.
func (l *recordingLibrary) Ingest(
	_ context.Context, name string, data []byte, authorID uuid.UUID,
) (media.Media, error) {
	if l.err != nil {
		return media.Media{}, l.err
	}
	l.ingested = append(l.ingested, name)
	m, err := media.New("2026/08/"+name, name, "image/jpeg", authorID)
	if err != nil {
		return media.Media{}, err
	}
	m.Filesize = int64(len(data))
	if !l.now.IsZero() {
		m.CreatedAt, m.UpdatedAt = l.now, l.now
	}
	return m, nil
}

// Finish records the upload marked finished.
func (l *recordingLibrary) Finish(m media.Media) {
	l.finished = append(l.finished, m.File)
}

// Remove records the files handed back for cleanup.
func (l *recordingLibrary) Remove(m media.Media) error {
	l.removed = append(l.removed, m.File)
	return nil
}

// countingMediaStore counts what a seeding stored.
type countingMediaStore struct {
	created   int
	stored    []media.Media
	listErr   error
	createErr error
}

// Create counts the stored item.
func (s *countingMediaStore) Create(_ context.Context, m media.Media) (media.Media, error) {
	if s.createErr != nil {
		return media.Media{}, s.createErr
	}
	s.created++
	m.ID = int64(s.created)
	s.stored = append(s.stored, m)
	return m, nil
}

// ByID reports every item missing.
func (s *countingMediaStore) ByID(context.Context, int64) (media.Media, error) {
	return media.Media{}, media.ErrNotFound
}

// ByIDs answers no items.
func (s *countingMediaStore) ByIDs(context.Context, []int64) ([]media.Media, error) {
	return nil, nil
}

// List returns what the store holds matching the search, one page at a time.
func (s *countingMediaStore) List(_ context.Context, f media.Filter) ([]media.Listed, int, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	matched := make([]media.Listed, 0, len(s.stored))
	for i := len(s.stored) - 1; i >= 0; i-- {
		item := s.stored[i]
		if f.Search == "" || strings.Contains(strings.ToLower(item.Title), strings.ToLower(f.Search)) {
			matched = append(matched, media.Listed{Media: item})
		}
	}
	start := min((f.Page-1)*f.PerPage, len(matched))
	end := min(start+f.PerPage, len(matched))
	return matched[start:end], len(matched), nil
}

// Update reports the item missing.
func (s *countingMediaStore) Update(context.Context, media.Media, time.Time) (media.Media, error) {
	return media.Media{}, media.ErrNotFound
}

// Delete reports the item missing.
func (s *countingMediaStore) Delete(context.Context, int64) (media.Media, error) {
	return media.Media{}, media.ErrNotFound
}

func TestMediaStoresEveryScriptedUpload(t *testing.T) {
	t.Parallel()

	library := &recordingLibrary{}
	store := &countingMediaStore{}

	if err := Media(t.Context(), library, store, stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Media() error = %v, want nil", err)
	}

	if store.created != len(demoMedia()) {
		t.Errorf("created %d items, want %d", store.created, len(demoMedia()))
	}
	if len(library.ingested) != len(demoMedia()) {
		t.Errorf("ingested %d uploads, want %d", len(library.ingested), len(demoMedia()))
	}
}

func TestMediaMarksEveryStoredUploadFinished(t *testing.T) {
	t.Parallel()

	library := &recordingLibrary{}
	store := &countingMediaStore{}

	if err := Media(t.Context(), library, store, stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Media() error = %v, want nil", err)
	}

	stored := make([]string, len(store.stored))
	for i, item := range store.stored {
		stored[i] = item.File
	}
	if !slices.Equal(library.finished, stored) {
		t.Errorf("finished %v, want every stored upload %v", library.finished, stored)
	}
}

func TestMediaSkipsWhatItAlreadyStored(t *testing.T) {
	t.Parallel()

	library := &recordingLibrary{}
	store := &countingMediaStore{}
	users := stubUserStore{id: uuid.New()}
	if err := Media(t.Context(), library, store, users); err != nil {
		t.Fatalf("first Media() error = %v, want nil", err)
	}
	stored := store.created

	if err := Media(t.Context(), library, store, users); err != nil {
		t.Fatalf("second Media() error = %v, want nil", err)
	}

	if store.created != stored {
		t.Errorf("created %d items over two runs, want %d", store.created, stored)
	}
}

func TestMediaSkipsWhatItStoredHoweverDeepTheLibraryIs(t *testing.T) {
	t.Parallel()

	library := &recordingLibrary{}
	store := &countingMediaStore{}
	users := stubUserStore{id: uuid.New()}
	if err := Media(t.Context(), library, store, users); err != nil {
		t.Fatalf("first Media() error = %v, want nil", err)
	}
	for range 150 {
		filler, err := media.New("2026/08/filler.jpg", "Filler upload", "image/jpeg", uuid.New())
		if err != nil {
			t.Fatalf("building filler: %v", err)
		}
		if _, err := store.Create(t.Context(), filler); err != nil {
			t.Fatalf("storing filler: %v", err)
		}
	}
	seeded := len(library.ingested)

	if err := Media(t.Context(), library, store, users); err != nil {
		t.Fatalf("second Media() error = %v, want nil", err)
	}

	if len(library.ingested) != seeded {
		t.Errorf("ingested %d uploads after a deep reseed, want %d", len(library.ingested), seeded)
	}
}

func TestMediaRemovesTheFilesOfAStoreThatRefused(t *testing.T) {
	t.Parallel()

	library := &recordingLibrary{}
	store := &countingMediaStore{createErr: errStub}

	if err := Media(t.Context(), library, store, stubUserStore{id: uuid.New()}); err == nil {
		t.Fatal("Media() error = nil, want the store failure reported")
	}

	if len(library.removed) != 1 {
		t.Errorf("removed %d uploads, want the orphaned files cleaned up", len(library.removed))
	}
	if len(library.finished) != 0 {
		t.Errorf("finished %v, want no upload marked finished when its item was never stored", library.finished)
	}
}

func TestMediaReportsFailures(t *testing.T) {
	t.Parallel()

	signedIn := stubUserStore{id: uuid.New()}
	tests := map[string]struct {
		library *recordingLibrary
		store   *countingMediaStore
		users   stubUserStore
	}{
		"admin lookup": {library: &recordingLibrary{}, store: &countingMediaStore{}, users: stubUserStore{err: errStub}},
		"listing":      {library: &recordingLibrary{}, store: &countingMediaStore{listErr: errStub}, users: signedIn},
		"ingest":       {library: &recordingLibrary{err: errStub}, store: &countingMediaStore{}, users: signedIn},
		"create":       {library: &recordingLibrary{}, store: &countingMediaStore{createErr: errStub}, users: signedIn},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := Media(t.Context(), test.library, test.store, test.users); err == nil {
				t.Error("Media() error = nil, want a failure")
			}
		})
	}
}

// unpaintable returns a scripted picture too large for the encoder to write.
func unpaintable() demoUpload {
	return demoUpload{
		file:   "too-large.jpg",
		title:  "Too large",
		width:  1 << 16,
		height: 1,
		paint:  cityPixel,
	}
}

func TestDemoUploadReportsAPictureItCannotPaint(t *testing.T) {
	t.Parallel()

	if _, err := unpaintable().build(); err == nil {
		t.Error("build() of an oversized picture error = nil, want a failure")
	}
}

func TestStoreDemoUploadReportsAPictureItCannotPaint(t *testing.T) {
	t.Parallel()

	err := storeDemoUpload(t.Context(), &recordingLibrary{}, &countingMediaStore{}, unpaintable(), uuid.New())

	if err == nil {
		t.Error("storeDemoUpload() of an oversized picture error = nil, want a failure")
	}
}

// emailUserStore answers each demo account under its own identity.
type emailUserStore struct {
	gouncer.Store
	ids map[string]uuid.UUID
}

// UserByEmail returns the account stored under the email.
func (s emailUserStore) UserByEmail(_ context.Context, email string) (gouncer.User, error) {
	return gouncer.User{ID: s.ids[email]}, nil
}

// demoPictures returns the demo uploads that are pictures, leaving the documents out.
func demoPictures() []demoUpload {
	return slices.DeleteFunc(demoMedia(), demoUpload.document)
}

func TestDemoMediaHoldsElevenPicturesAndOneDocument(t *testing.T) {
	t.Parallel()

	documents := len(demoMedia()) - len(demoPictures())

	if len(demoPictures()) != 11 || documents != 1 {
		t.Errorf("the demo media holds %d pictures and %d documents, want 11 and 1", len(demoPictures()), documents)
	}
}

func TestDemoMediaSpreadsOverEighteenMonthsByThreeAuthors(t *testing.T) {
	t.Parallel()

	authors := make(map[string]bool)
	days := make(map[int]bool)
	oldest := 0
	for _, scripted := range demoMedia() {
		authors[scripted.author] = true
		days[scripted.daysAgo] = true
		oldest = max(oldest, scripted.daysAgo)
	}

	if want := map[string]bool{AdminEmail: true, EditorEmail: true, AuthorEmail: true}; !maps.Equal(authors, want) {
		t.Errorf("the demo media is uploaded by %v, want the three demo accounts", authors)
	}
	if len(days) != len(demoMedia()) {
		t.Errorf("the demo media arrives on %d days, want each upload on its own day", len(days))
	}
	if oldest < 480 || oldest > 548 {
		t.Errorf("the oldest upload arrived %d days ago, want about eighteen months", oldest)
	}
}

func TestDemoMediaEditsOneUploadAfterItArrived(t *testing.T) {
	t.Parallel()

	edited := 0
	for _, scripted := range demoMedia() {
		if scripted.editedAfter == 0 {
			continue
		}
		edited++
		if scripted.editedAfter > scripted.daysAgo {
			t.Errorf("%q is edited %d days after an upload %d days ago, want the edit in the past",
				scripted.title, scripted.editedAfter, scripted.daysAgo)
		}
	}

	if edited != 1 {
		t.Errorf("the demo media holds %d uploads edited later, want 1", edited)
	}
}

func TestMediaFilesEachUploadUnderItsAuthorOnItsDays(t *testing.T) {
	t.Parallel()

	ids := map[string]uuid.UUID{AdminEmail: uuid.New(), EditorEmail: uuid.New(), AuthorEmail: uuid.New()}
	store := &countingMediaStore{}
	seeded := time.Date(2026, time.October, 11, 9, 30, 0, 0, time.UTC)

	if err := Media(t.Context(), &recordingLibrary{now: seeded}, store, emailUserStore{ids: ids}); err != nil {
		t.Fatalf("Media() error = %v, want nil", err)
	}

	scripts := demoMedia()
	if len(store.stored) != len(scripts) {
		t.Fatalf("stored %d items, want %d", len(store.stored), len(scripts))
	}
	for i, stored := range store.stored {
		scripted := scripts[i]
		if stored.AuthorID != ids[scripted.author] {
			t.Errorf("%q is filed under %v, want the account of %s", scripted.title, stored.AuthorID, scripted.author)
		}
		if added := seeded.AddDate(0, 0, -scripted.daysAgo); !stored.CreatedAt.Equal(added) {
			t.Errorf("%q was added at %v, want %v, %d days before its upload", scripted.title, stored.CreatedAt, added,
				scripted.daysAgo)
		}
		if edit := stored.UpdatedAt.Sub(stored.CreatedAt); edit != time.Duration(scripted.editedAfter)*24*time.Hour {
			t.Errorf("%q was modified %v after its upload, want %d days", scripted.title, edit, scripted.editedAfter)
		}
	}
}

func TestDemoMediaCarriesAReadableDocument(t *testing.T) {
	t.Parallel()

	for _, scripted := range demoMedia() {
		if !scripted.document() {
			continue
		}
		data, err := scripted.data()
		if err != nil {
			t.Fatalf("building %q: %v", scripted.file, err)
		}
		if sniffed := http.DetectContentType(data); sniffed != "application/pdf" {
			t.Errorf("%q sniffs as %s, want application/pdf", scripted.file, sniffed)
		}
		if !bytes.Contains(data, []byte("("+scripted.title+")")) {
			t.Errorf("%q does not print its title on its page", scripted.file)
		}
		start := bytes.LastIndex(data, []byte("startxref\n"))
		var offset int
		if _, err := fmt.Sscanf(string(data[start+len("startxref\n"):]), "%d", &offset); err != nil {
			t.Fatalf("%q names no cross reference offset: %v", scripted.file, err)
		}
		if !bytes.HasPrefix(data[offset:], []byte("xref")) {
			t.Errorf("%q points its cross reference table at %d, want the xref keyword there", scripted.file, offset)
		}
	}
}

func TestDemoMediaVariesItsShapes(t *testing.T) {
	t.Parallel()

	shapes := make(map[string]bool)
	for _, scripted := range demoPictures() {
		shapes[fmt.Sprintf("%dx%d", scripted.width, scripted.height)] = true
	}

	if len(shapes) != len(demoPictures()) {
		t.Errorf("the demo pictures hold %d shapes, want one each", len(shapes))
	}
}

func TestDemoMediaCarriesReadableImages(t *testing.T) {
	t.Parallel()

	for _, scripted := range demoPictures() {
		data, err := scripted.build()
		if err != nil {
			t.Fatalf("building %q: %v", scripted.file, err)
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decoding %q: %v", scripted.file, err)
		}
		if format != "jpeg" {
			t.Errorf("%q is a %s, want a jpeg", scripted.file, format)
		}
		if cfg.Width < 600 || cfg.Height < 400 {
			t.Errorf("%q measures %dx%d, want a picture big enough to derive sub-sizes", scripted.file, cfg.Width, cfg.Height)
		}
	}
}
