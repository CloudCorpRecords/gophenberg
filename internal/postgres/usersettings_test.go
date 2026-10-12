// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres"
)

// newUserSettingStore returns a reader setting store over a migrated database and its pool.
func newUserSettingStore(t *testing.T) (*postgres.UserSettingStore, *pgxpool.Pool) {
	t.Helper()
	pool := migratedPool(t)
	return postgres.NewUserSettingStore(pool), pool
}

func TestAReaderSettingReportsADatabaseItCannotReach(t *testing.T) {
	store, pool := newUserSettingStore(t)
	pool.Close()

	if _, _, err := store.Lookup(t.Context(), uuid.New(), "locale"); err == nil {
		t.Error("Lookup() error = nil, want the closed pool reported")
	}
	if err := store.Save(t.Context(), uuid.New(), "locale", "es-ES"); err == nil {
		t.Error("Save() error = nil, want the closed pool reported")
	}
}
