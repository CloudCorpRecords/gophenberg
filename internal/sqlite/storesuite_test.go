// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"
	authkitsqlite "github.com/gopherium/gouncer/authkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/media/mediatest"
	"github.com/gopherium/gophenberg/internal/sqlite"
)

// accountsOpened is the stamp every account the tests add carries.
var accountsOpened = time.Date(2026, time.January, 5, 8, 0, 0, 0, time.UTC)

// addAuthor stores an account named name in the file behind handle and returns its id.
func addAuthor(t *testing.T, handle *sql.DB, name string) uuid.UUID {
	t.Helper()
	author := uuid.Must(uuid.NewV7())
	err := authkitsqlite.NewUserStore(handle).CreateUser(t.Context(), gouncer.User{
		ID: author, Email: author.String() + "@example.com", Name: name, PasswordHash: "hash", CreatedAt: accountsOpened,
	})
	if err != nil {
		t.Fatalf("adding the author %q: %v", name, err)
	}
	return author
}

// authorsIn returns a way to add accounts to the file behind handle.
func authorsIn(handle *sql.DB) func(t *testing.T, name string) uuid.UUID {
	return func(t *testing.T, name string) uuid.UUID {
		t.Helper()
		return addAuthor(t, handle, name)
	}
}

// sqliteStores returns fresh site and user setting stores over a migrated copy of their own.
func sqliteStores(t *testing.T) contenttest.Stores {
	t.Helper()
	handle := migrated(t)
	return contenttest.Stores{
		SiteSettings: sqlite.NewSettingStore(handle),
		UserSettings: sqlite.NewUserSettingStore(handle),
		AddAuthor:    authorsIn(handle),
	}
}

// sqliteMediaStores returns a fresh media store over a migrated copy of its own.
func sqliteMediaStores(t *testing.T) mediatest.Stores {
	t.Helper()
	handle := migrated(t)
	store := sqlite.NewMediaStore(handle)
	return mediatest.Stores{Media: store, Saved: store.Saved, AddAuthor: authorsIn(handle)}
}

func TestMediaStoreSuite(t *testing.T) {
	t.Parallel()

	mediatest.Run(t, sqliteMediaStores)
}

func TestSiteSettingStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.RunSiteSettings(t, sqliteStores)
}

func TestUserSettingStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.RunUserSettings(t, sqliteStores)
}
