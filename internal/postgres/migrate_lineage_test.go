// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// versionRow is one row of a goose version table, its stamp kept as the server prints it.
type versionRow struct {
	id      int64
	version int64
	applied bool
	stamp   string
}

// versionRows returns every row of the version table goose names goose_db_version in db, in the order it was written.
func versionRows(t *testing.T, db *sql.DB) []versionRow {
	t.Helper()
	rows, err := db.QueryContext(t.Context(),
		"SELECT id, version_id, is_applied, tstamp::text FROM public.goose_db_version ORDER BY id")
	if err != nil {
		t.Fatalf("reading goose_db_version: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var held []versionRow
	for rows.Next() {
		var row versionRow
		if err := rows.Scan(&row.id, &row.version, &row.applied, &row.stamp); err != nil {
			t.Fatalf("scanning goose_db_version: %v", err)
		}
		held = append(held, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading goose_db_version: %v", err)
	}
	return held
}

// tables returns the schema and name of every table outside the system schemas of db.
func tables(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), `SELECT table_schema || '.' || table_name FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema') ORDER BY 1`)
	if err != nil {
		t.Fatalf("listing the tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var held []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning a table name: %v", err)
		}
		held = append(held, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("listing the tables: %v", err)
	}
	return held
}

func TestMigrateRunsNothingAgainOnADatabaseGooseMigratedBefore(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t, accountsMigrated(t))
	if _, err := coreProvider(t, db).Up(t.Context()); err != nil {
		t.Fatalf("goose Up() error = %v, want the core schema built", err)
	}
	rowsBefore, tablesBefore := versionRows(t, db), tables(t, db)

	if err := postgres.Migrate(t.Context(), db); err != nil {
		t.Fatalf("Migrate() error = %v, want nothing left to apply", err)
	}

	if rowsAfter := versionRows(t, db); !slices.Equal(rowsAfter, rowsBefore) {
		t.Errorf("goose_db_version = %v after Migrate(), want it as goose left it, %v", rowsAfter, rowsBefore)
	}
	if tablesAfter := tables(t, db); !slices.Equal(tablesAfter, tablesBefore) {
		t.Errorf("tables = %v after Migrate(), want no table added, %v", tablesAfter, tablesBefore)
	}
}

func TestGooseFindsNothingToApplyAfterMigrate(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t, accountsMigrated(t))
	if err := postgres.Migrate(t.Context(), db); err != nil {
		t.Fatalf("Migrate() error = %v, want the core schema built", err)
	}

	pending, err := coreProvider(t, db).HasPending(t.Context())

	if err != nil || pending {
		t.Errorf("goose HasPending() = %v, %v after Migrate(), want nothing pending in goose_db_version", pending, err)
	}
}
