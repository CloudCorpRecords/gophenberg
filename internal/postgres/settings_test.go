// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres"
)

// newSettingStoreWithPool returns a store over a migrated database and its pool.
func newSettingStoreWithPool(t *testing.T) (*postgres.SettingStore, *pgxpool.Pool) {
	t.Helper()
	pool := migratedPool(t)
	return postgres.NewSettingStore(pool), pool
}

func TestSettingWritesNothingWhenTheWriteCannotStart(t *testing.T) {
	store, pool := newSettingStoreWithPool(t)
	if err := store.Save(t.Context(), map[string]string{"theme.active": "aurora"}); err != nil {
		t.Fatalf("seeding the active theme: %v", err)
	}
	pool.Close()

	err := store.Save(t.Context(), map[string]string{
		"theme.active":   "riverbed",
		"theme.previous": "aurora",
	})

	if err == nil {
		t.Fatal("Save() on a closed pool error = nil, want a failure")
	}
}

func TestSettingReportsDatabaseFailures(t *testing.T) {
	store, pool := newSettingStoreWithPool(t)
	pool.Close()

	if _, _, err := store.Lookup(t.Context(), "theme.active"); err == nil {
		t.Error("Lookup() on a closed pool error = nil, want a failure")
	}
	if err := store.Save(t.Context(), map[string]string{"theme.active": "aurora"}); err == nil {
		t.Error("Save() on a closed pool error = nil, want a failure")
	}
}
