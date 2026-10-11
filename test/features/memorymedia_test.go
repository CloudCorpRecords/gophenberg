// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
)

// memoryMedia holds one scenario's media library in memory.
type memoryMedia struct {
	mu       sync.Mutex
	next     int64
	items    map[int64]media.Media
	accounts *memoryStore
}

// newMemoryMedia returns an empty in-memory media store naming its authors from the accounts.
func newMemoryMedia(accounts *memoryStore) *memoryMedia {
	return &memoryMedia{items: make(map[int64]media.Media), accounts: accounts}
}

// Create stores a new media item once its author holds an account and returns it with its assigned identifier.
func (s *memoryMedia) Create(_ context.Context, m media.Media) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.accounts.holdsUser(m.AuthorID) {
		return media.Media{}, fmt.Errorf("memory: create media: author %s holds no account", m.AuthorID)
	}
	s.next++
	m.ID = s.next
	s.items[m.ID] = m
	return m, nil
}

// ByID returns the media item with the given id, or [media.ErrNotFound].
func (s *memoryMedia) ByID(_ context.Context, id int64) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, found := s.items[id]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	return m, nil
}

// ByIDs returns the media items stored under the identities, leaving out any that are gone.
func (s *memoryMedia) ByIDs(_ context.Context, ids []int64) ([]media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	listed := make([]media.Media, 0, len(ids))
	for _, id := range ids {
		if m, found := s.items[id]; found {
			listed = append(listed, m)
		}
	}
	return listed, nil
}

// List returns the media items the filter matches, sorted as it names, beside their authors' names, with their total.
func (s *memoryMedia) List(_ context.Context, f media.Filter) ([]media.Listed, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := make([]media.Listed, 0, len(s.items))
	for _, m := range s.items {
		if matches(m, f) {
			matched = append(matched, media.Listed{Media: m, AuthorName: s.accounts.nameOf(m.AuthorID)})
		}
	}
	slices.SortFunc(matched, func(a, b media.Listed) int {
		return cmp.Or(mediaListOrder(f, a, b), cmp.Compare(b.ID, a.ID))
	})
	return page(matched, f), len(matched), nil
}

// mediaListOrder compares two listed items by the column and direction the filter names, the date added by default.
func mediaListOrder(f media.Filter, a, b media.Listed) int {
	var by int
	switch f.OrderBy {
	case media.OrderByTitle:
		by = collated(a.Title, b.Title)
	case media.OrderByAuthor:
		by = collated(a.AuthorName, b.AuthorName)
	case media.OrderByModified:
		by = a.UpdatedAt.Compare(b.UpdatedAt)
	default:
		by = a.CreatedAt.Compare(b.CreatedAt)
	}
	if f.Order != media.OrderAsc {
		return -by
	}
	return by
}

// changeTitled applies the change to the stored item carrying the title.
func (s *memoryMedia) changeTitled(title string, change func(*media.Media)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, item := range s.items {
		if item.Title == title {
			change(&item)
			s.items[id] = item
			return nil
		}
	}
	return fmt.Errorf("no media item carries the title %q", title)
}

// matches reports whether the item satisfies the filter.
func matches(m media.Media, f media.Filter) bool {
	if f.Type != "" && m.Type != f.Type {
		return false
	}
	if !matchesMime(m, f.Mimes) || !withinAuthorsAndDates(m, f) {
		return false
	}
	if f.Search == "" {
		return true
	}
	search := strings.ToLower(f.Search)
	return strings.Contains(strings.ToLower(m.Title), search) ||
		strings.Contains(strings.ToLower(m.File), search)
}

// withinAuthorsAndDates reports whether the item stands within the authors and the date bounds the filter narrows to.
func withinAuthorsAndDates(m media.Media, f media.Filter) bool {
	switch {
	case len(f.Authors) > 0 && !slices.Contains(f.Authors, m.AuthorID), slices.Contains(f.ExcludeAuthors, m.AuthorID):
		return false
	case f.Before != nil && !m.CreatedAt.Before(*f.Before), f.After != nil && !m.CreatedAt.After(*f.After):
		return false
	case f.ModifiedBefore != nil && !m.UpdatedAt.Before(*f.ModifiedBefore):
		return false
	}
	return f.ModifiedAfter == nil || m.UpdatedAt.After(*f.ModifiedAfter)
}

// matchesMime reports whether the item's content type starts with any prefix.
func matchesMime(m media.Media, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(m.MimeType, prefix) {
			return true
		}
	}
	return false
}

// page returns the slice of items the filter's page holds.
func page(items []media.Listed, f media.Filter) []media.Listed {
	start := (f.Page - 1) * f.PerPage
	if start >= len(items) {
		return nil
	}
	end := min(start+f.PerPage, len(items))
	return items[start:end]
}

// Update stores the media item's descriptions, or reports a missing item or a stale edit.
func (s *memoryMedia) Update(_ context.Context, m media.Media, expectedUpdatedAt time.Time) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, found := s.items[m.ID]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	if !stored.UpdatedAt.Equal(expectedUpdatedAt) {
		return media.Media{}, media.ErrConflict
	}
	stored.Title = m.Title
	stored.AltText = m.AltText
	stored.Caption = m.Caption
	stored.Description = m.Description
	stored.UpdatedAt = m.UpdatedAt
	s.items[m.ID] = stored
	return stored, nil
}

// Delete removes the media item and returns it, or reports [media.ErrNotFound].
func (s *memoryMedia) Delete(_ context.Context, id int64) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, found := s.items[id]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	delete(s.items, id)
	return m, nil
}

func TestMediaStoreRefusesAnAuthorWithoutAnAccount(t *testing.T) {
	t.Parallel()

	store := newMemoryMedia(newMemoryStore())
	orphan, err := media.New("2026/10/orphan.jpg", "orphan", "image/jpeg", uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	_, err = store.Create(t.Context(), orphan)

	if err == nil {
		t.Fatal("Create() error = nil, want the author refused for holding no account")
	}
	if _, total, _ := store.List(t.Context(), media.Filter{Page: 1, PerPage: 10}); total != 0 {
		t.Errorf("List() total = %d, want the refused item left out", total)
	}
}
