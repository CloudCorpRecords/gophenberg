// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

// RaiseFrom is raiseFrom, for the tests of a raise another binary got to first.
var RaiseFrom = raiseFrom

// MigrateDownTo rolls the core schema on db back to the given version for migration tests.
func MigrateDownTo(ctx context.Context, db *sql.DB, version int64) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationSource)
	if err != nil {
		return fmt.Errorf("postgres: migration provider: %w", err)
	}
	if _, err := provider.DownTo(ctx, version); err != nil {
		return fmt.Errorf("postgres: roll migrations back: %w", err)
	}
	return nil
}
