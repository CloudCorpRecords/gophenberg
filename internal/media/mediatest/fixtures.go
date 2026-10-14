// SPDX-License-Identifier: Apache-2.0

package mediatest

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
)

// DefaultAuthor is the name the suite gives the account most cases upload as.
const DefaultAuthor = "Maria Perez"

// MustImage returns an image item measured and carrying two sub-sizes.
func MustImage(t *testing.T, file, title string, author uuid.UUID) media.Media {
	t.Helper()
	m, err := media.New(file, title, "image/jpeg", author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", file, err)
	}
	m.Width = 3000
	m.Height = 2000
	m.Filesize = 1_500_000
	m.Sizes = media.Sizes{
		"thumbnail": {File: title + "-150x150.jpg", Width: 150, Height: 150, MimeType: "image/jpeg", Filesize: 9_000},
		"large":     {File: title + "-1024x683.jpg", Width: 1024, Height: 683, MimeType: "image/jpeg", Filesize: 210_000},
	}
	return m
}

// MustCreate stores the given media item.
func MustCreate(t *testing.T, store media.Store, m media.Media) media.Media {
	t.Helper()
	created, err := store.Create(t.Context(), m)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", m.File, err)
	}
	return created
}
