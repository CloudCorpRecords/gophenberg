// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"
	authkitsqlite "github.com/gopherium/gouncer/authkit/sqlite"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

// testMigrate is how every test run of Migrate waits for the migration lock.
var testMigrate = sqlite.MigrateOptions{LockWait: 5 * time.Second, LockPoll: 10 * time.Millisecond}

// template is the database with the account and core tables that each test opens a copy of.
var template *sqlitetest.Template

// migrateAccounts applies the account tables the core tables point at to db.
func migrateAccounts(ctx context.Context, db *sql.DB) error {
	return authkitsqlite.Migrate(ctx, db, authkitsqlite.MigrateOptions{
		LockWait: testMigrate.LockWait, LockPoll: testMigrate.LockPoll,
	})
}

// migrateAll applies the account tables, then the core tables, to db.
func migrateAll(ctx context.Context, db *sql.DB) error {
	if err := migrateAccounts(ctx, db); err != nil {
		return err
	}
	return sqlite.Migrate(ctx, db, testMigrate)
}

// migrated returns a handle on a fresh copy of the migrated template, closed when t ends.
func migrated(t *testing.T) *sql.DB {
	t.Helper()
	return template.Open(t)
}

// mustExec runs statement with args on db and fails t when it errs.
func mustExec(t *testing.T, db *sql.DB, statement string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), statement, args...); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
}

// count returns the single integer lookup with args answers on db.
func count(t *testing.T, db *sql.DB, lookup string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRowContext(t.Context(), lookup, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	return n
}

// TestMain builds the migrated template, runs the tests, then removes the template.
func TestMain(m *testing.M) {
	built, err := sqlitetest.NewTemplate(context.Background(), testSettings().Options(), migrateAll)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "building the migrated template:", err)
		os.Exit(1)
	}
	template = built
	code := m.Run()
	_ = template.Close()
	os.Exit(code)
}
