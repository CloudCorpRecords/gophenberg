// SPDX-License-Identifier: Apache-2.0

package mediatest

import (
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
)

// mediaCases are the cases over creating, reading, editing, deleting and finding the files of media items.
var mediaCases = []Case{
	{"CreateAndReadBack", createAndReadBack},
	{"AssignsRisingIdentifiers", assignsRisingIdentifiers},
	{"NeverHandsOutADeletedIdentityAgain", neverHandsOutADeletedIdentityAgain},
	{"CreateRefusesAnAuthorWithoutAnAccount", createRefusesAnAuthorWithoutAnAccount},
	{"ByIDReportsAMissingItem", byIDReportsAMissingItem},
	{"UpdateEditsDescriptions", updateEditsDescriptions},
	{"UpdateRefusesAStaleEdit", updateRefusesAStaleEdit},
	{"UpdateReportsAMissingItem", updateReportsAMissingItem},
	{"DeleteReturnsTheDeletedItem", deleteReturnsTheDeletedItem},
	{"DeleteReportsAMissingItem", deleteReportsAMissingItem},
	{"SavedNamesOnlyTheFilesAStoredItemHolds", savedNamesOnlyTheFilesAStoredItemHolds},
	{"ReadsSeveralByTheirIdentities", readsSeveralByTheirIdentities},
	{"ReadsFewerWhenAnIdentityIsGone", readsFewerWhenAnIdentityIsGone},
	{"ReadsNothingFromNoIdentities", readsNothingFromNoIdentities},
	{"ReadsStampsBackInUTC", readsStampsBackInUTC},
	{"TouchesOnlyTheNamedItem", touchesOnlyTheNamedItem},
}

// createAndReadBack reads a created item back with its descriptions, measurements, sub-sizes and author.
func createAndReadBack(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	m := MustImage(t, "2026/08/harbor.jpg", "harbor", author)
	m.AltText, m.Caption, m.Description = "Boats at the quay", "The harbor at dawn", "Taken from the pier"

	created := MustCreate(t, s.Media, m)

	if created.ID <= 0 {
		t.Errorf("ID = %d, want a positive assigned identifier", created.ID)
	}

	got, err := s.Media.ByID(t.Context(), created.ID)

	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if got.Type != media.TypeImage || got.File != m.File || got.Title != m.Title {
		t.Errorf("ByID() = %+v, want the created item", got)
	}
	if got.AltText != m.AltText || got.Caption != m.Caption || got.Description != m.Description {
		t.Errorf("ByID() = alt %q, caption %q, description %q, want %q, %q and %q",
			got.AltText, got.Caption, got.Description, m.AltText, m.Caption, m.Description)
	}
	if got.MimeType != m.MimeType || got.Width != m.Width || got.Height != m.Height || got.Filesize != m.Filesize {
		t.Errorf("ByID() measurements = %+v, want the created ones", got)
	}
	if !maps.Equal(got.Sizes, m.Sizes) {
		t.Errorf("Sizes = %v, want the stored sub-sizes", got.Sizes)
	}
	if got.AuthorID != author {
		t.Errorf("AuthorID = %v, want the uploading author", got.AuthorID)
	}
	if got.CreatedAt.Location() != time.UTC || got.UpdatedAt.Location() != time.UTC {
		t.Errorf("timestamps carry location %v, want UTC", got.CreatedAt.Location())
	}
}

// assignsRisingIdentifiers gives each new item a larger identifier than the one before.
func assignsRisingIdentifiers(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)

	first := MustCreate(t, s.Media, MustImage(t, "2026/08/first.jpg", "first", author))
	second := MustCreate(t, s.Media, MustImage(t, "2026/08/second.jpg", "second", author))

	if second.ID <= first.ID {
		t.Errorf("identifiers = %d then %d, want them rising", first.ID, second.ID)
	}
}

// neverHandsOutADeletedIdentityAgain gives a new item a larger identifier than a deleted newest item held.
func neverHandsOutADeletedIdentityAgain(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	day := func(month time.Month) time.Time { return time.Date(2026, month, 10, 0, 0, 0, 0, time.UTC) }
	mustCreateMediaAt(t, s.Media, "harbor", author, day(time.March), day(time.March))
	newest := mustCreateMediaAt(t, s.Media, "cliff", author, day(time.May), day(time.May))
	if _, err := s.Media.Delete(t.Context(), newest.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	next := mustCreateMediaAt(t, s.Media, "pier", author, day(time.July), day(time.July))

	if next.ID <= newest.ID {
		t.Errorf("ID = %d after deleting %d, want a larger identifier than the deleted item held", next.ID, newest.ID)
	}
}

// createRefusesAnAuthorWithoutAnAccount refuses an item whose author holds no account and stores nothing.
func createRefusesAnAuthorWithoutAnAccount(t *testing.T, s Stores) {
	orphan := MustImage(t, "2026/10/orphan.jpg", "orphan", uuid.Must(uuid.NewV7()))

	_, err := s.Media.Create(t.Context(), orphan)

	if err == nil {
		t.Fatal("Create() error = nil, want the author refused for holding no account")
	}
	_, total, err := s.Media.List(t.Context(), media.Filter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 0 {
		t.Errorf("List() total = %d, want the refused item left out", total)
	}
}

// byIDReportsAMissingItem reports a read of an identifier nothing holds as not found.
func byIDReportsAMissingItem(t *testing.T, s Stores) {
	if _, err := s.Media.ByID(t.Context(), 12345); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("ByID() on a missing id error = %v, want ErrNotFound", err)
	}
}

// updateEditsDescriptions stores edited descriptions and a later stamp and keeps the file, sub-sizes and upload date.
func updateEditsDescriptions(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))
	edited := created
	edited.Title = "Harbor at dawn"
	edited.AltText = "Fishing boats moored at sunrise"
	edited.Caption = "The harbor before the market opens"
	edited.Description = "Taken from the eastern pier"
	edited.UpdatedAt = created.UpdatedAt.Add(time.Second)
	edited.File, edited.Sizes = "2026/08/other.jpg", media.Sizes{}
	edited.CreatedAt = created.CreatedAt.Add(-time.Hour)

	updated, err := s.Media.Update(t.Context(), edited, created.UpdatedAt)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Title != edited.Title || updated.AltText != edited.AltText {
		t.Errorf("Update() = %+v, want the edited descriptions", updated)
	}
	if updated.Caption != edited.Caption || updated.Description != edited.Description {
		t.Errorf("Update() = %+v, want the edited descriptions", updated)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want it moved past %v", updated.UpdatedAt, created.UpdatedAt)
	}
	if updated.File != created.File || !maps.Equal(updated.Sizes, created.Sizes) {
		t.Errorf("Update() = %+v, want the stored file untouched", updated)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt = %v, want the upload date %v untouched", updated.CreatedAt, created.CreatedAt)
	}
}

// updateRefusesAStaleEdit reports an edit built on an older stamp as a conflict and keeps the first edit.
func updateRefusesAStaleEdit(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))
	edited := created
	edited.Title = "Harbor at dawn"
	edited.UpdatedAt = created.UpdatedAt.Add(time.Second)
	if _, err := s.Media.Update(t.Context(), edited, created.UpdatedAt); err != nil {
		t.Fatalf("first Update() error = %v, want nil", err)
	}

	staleEdit := created
	staleEdit.Title = "Harbor at noon"
	staleEdit.UpdatedAt = created.UpdatedAt.Add(2 * time.Second)

	_, err := s.Media.Update(t.Context(), staleEdit, created.UpdatedAt)

	if !errors.Is(err, media.ErrConflict) {
		t.Errorf("stale Update() error = %v, want ErrConflict", err)
	}

	got, err := s.Media.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if got.Title != "Harbor at dawn" {
		t.Errorf("Title = %q, want the first edit untouched", got.Title)
	}
}

// updateReportsAMissingItem reports an edit of an identifier nothing holds as not found.
func updateReportsAMissingItem(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	ghost := MustImage(t, "2026/08/ghost.jpg", "ghost", author)
	ghost.ID = 12345

	if _, err := s.Media.Update(t.Context(), ghost, ghost.UpdatedAt); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("Update() on a missing id error = %v, want ErrNotFound", err)
	}
}

// deleteReturnsTheDeletedItem hands back the deleted item with its files and reads it as gone after.
func deleteReturnsTheDeletedItem(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))

	deleted, err := s.Media.Delete(t.Context(), created.ID)

	if err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	if deleted.File != created.File || !maps.Equal(deleted.Sizes, created.Sizes) {
		t.Errorf("Delete() = %+v, want the files the caller must remove", deleted)
	}

	if _, err := s.Media.ByID(t.Context(), created.ID); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("ByID() after delete error = %v, want ErrNotFound", err)
	}
}

// deleteReportsAMissingItem reports a second delete of the same item as not found.
func deleteReportsAMissingItem(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))
	if _, err := s.Media.Delete(t.Context(), created.ID); err != nil {
		t.Fatalf("first Delete() error = %v, want nil", err)
	}

	if _, err := s.Media.Delete(t.Context(), created.ID); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("second Delete() error = %v, want ErrNotFound", err)
	}
}

// savedNamesOnlyTheFilesAStoredItemHolds names the asked files a stored item holds as its own and leaves out the rest.
func savedNamesOnlyTheFilesAStoredItemHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author))
	MustCreate(t, s.Media, MustImage(t, "2026/08/pier.jpg", "pier", author))

	saved, err := s.Saved(t.Context(), []string{"2026/08/harbor.jpg", "harbor-150x150.jpg", "2026/08/cliff.jpg"})

	if err != nil {
		t.Fatalf("Saved() error = %v, want nil", err)
	}
	if want := map[string]bool{"2026/08/harbor.jpg": true}; !maps.Equal(saved, want) {
		t.Errorf("Saved() = %v, want %v", saved, want)
	}
}

// readsSeveralByTheirIdentities reads every item named by its identity with its file, title and sub-sizes.
func readsSeveralByTheirIdentities(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	first := MustCreate(t, s.Media, MustImage(t, "2026/08/first.jpg", "first", author))
	second := MustCreate(t, s.Media, MustImage(t, "2026/08/second.jpg", "second", author))

	listed, err := s.Media.ByIDs(t.Context(), []int64{first.ID, second.ID})

	if err != nil {
		t.Fatalf("ByIDs() error = %v, want nil", err)
	}
	byID := make(map[int64]media.Media, len(listed))
	for _, held := range listed {
		byID[held.ID] = held
	}
	if len(byID) != 2 {
		t.Fatalf("ByIDs() = %d items, want both", len(byID))
	}
	if byID[first.ID].File != "2026/08/first.jpg" || byID[first.ID].Title != "first" {
		t.Errorf("first = %+v, want the stored file and title read back", byID[first.ID])
	}
	if len(byID[second.ID].Sizes) != 2 {
		t.Errorf("second sizes = %v, want the sub-sizes read back", byID[second.ID].Sizes)
	}
}

// readsFewerWhenAnIdentityIsGone leaves out an identity nothing holds and reads the rest.
func readsFewerWhenAnIdentityIsGone(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	kept := MustCreate(t, s.Media, MustImage(t, "2026/08/kept.jpg", "kept", author))

	listed, err := s.Media.ByIDs(t.Context(), []int64{kept.ID, kept.ID + 1000})

	if err != nil {
		t.Fatalf("ByIDs() error = %v, want nil", err)
	}
	if len(listed) != 1 || listed[0].ID != kept.ID {
		t.Errorf("ByIDs() = %+v, want only the stored item", listed)
	}
}

// readsNothingFromNoIdentities reads no items when it is given no identities.
func readsNothingFromNoIdentities(t *testing.T, s Stores) {
	listed, err := s.Media.ByIDs(t.Context(), nil)

	if err != nil {
		t.Fatalf("ByIDs() error = %v, want nil", err)
	}
	if len(listed) != 0 {
		t.Errorf("ByIDs() = %+v, want nothing", listed)
	}
}

// readsStampsBackInUTC hands back stamps given in another zone as the same moments in UTC on every read.
func readsStampsBackInUTC(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	added := time.Date(2026, time.July, 10, 9, 30, 0, 0, time.FixedZone("UTC-5", -5*60*60))
	edited := added.Add(time.Hour)
	item := MustImage(t, "2026/07/harbor.jpg", "harbor", author)
	item.CreatedAt, item.UpdatedAt = added, added

	created := MustCreate(t, s.Media, item)
	read, err := s.Media.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	listed, _, err := s.Media.List(t.Context(), media.Filter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List() = %d items, want the stored item", len(listed))
	}
	change := read
	change.Title, change.UpdatedAt = "Harbor at dawn", edited
	updated, err := s.Media.Update(t.Context(), change, read.UpdatedAt)
	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	for _, stamp := range []struct {
		name      string
		got, want time.Time
	}{
		{"Create() added", created.CreatedAt, added}, {"Create() modified", created.UpdatedAt, added},
		{"ByID() added", read.CreatedAt, added}, {"ByID() modified", read.UpdatedAt, added},
		{"List() added", listed[0].CreatedAt, added}, {"List() modified", listed[0].UpdatedAt, added},
		{"Update() added", updated.CreatedAt, added}, {"Update() modified", updated.UpdatedAt, edited},
	} {
		if stamp.got.Location() != time.UTC || !stamp.got.Equal(stamp.want) {
			t.Errorf("%s = %v, want %v in UTC", stamp.name, stamp.got, stamp.want.UTC())
		}
	}
}

// touchesOnlyTheNamedItem reads, edits and deletes the item its identity names and leaves the others alone.
func touchesOnlyTheNamedItem(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	held := []media.Media{
		MustCreate(t, s.Media, MustImage(t, "2026/08/harbor.jpg", "harbor", author)),
		MustCreate(t, s.Media, MustImage(t, "2026/08/cliff.jpg", "cliff", author)),
		MustCreate(t, s.Media, MustImage(t, "2026/08/pier.jpg", "pier", author)),
	}
	for _, item := range held {
		if got, err := s.Media.ByID(t.Context(), item.ID); err != nil || got.File != item.File {
			t.Fatalf("ByID(%d) = %q, error %v, want %q", item.ID, got.File, err, item.File)
		}
	}
	if listed, err := s.Media.ByIDs(t.Context(), nil); err != nil || len(listed) != 0 {
		t.Fatalf("ByIDs(nil) = %d items, error %v, want none", len(listed), err)
	}
	middle := held[1]
	edited := middle
	edited.Title, edited.UpdatedAt = "Cliff at dusk", middle.UpdatedAt.Add(time.Second)

	updated, err := s.Media.Update(t.Context(), edited, middle.UpdatedAt)
	if err != nil || updated.ID != middle.ID {
		t.Fatalf("Update(%d) = item %d, error %v, want the named item edited", middle.ID, updated.ID, err)
	}
	if deleted, err := s.Media.Delete(t.Context(), middle.ID); err != nil || deleted.File != middle.File {
		t.Fatalf("Delete(%d) = %q, error %v, want %q", middle.ID, deleted.File, err, middle.File)
	}

	for _, item := range []media.Media{held[0], held[2]} {
		got, err := s.Media.ByID(t.Context(), item.ID)
		if err != nil || got.Title != item.Title {
			t.Errorf("ByID(%d) = %q, error %v, want %q left alone", item.ID, got.Title, err, item.Title)
		}
	}
}
