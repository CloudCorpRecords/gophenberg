// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"time"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"
)

const (
	// versionTable is the goose version table of the core tables, beside the version tables of the other owners.
	versionTable = "core_goose_db_version"
	// accountsLookup reports whether the database holds the account table the core tables point at.
	accountsLookup = "SELECT EXISTS (SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = 'auth_users')"
)

// migrationSource holds the core migration files at its root.
var migrationSource = mustSub(migrations, "migrations")

// ErrNoAccounts reports that the core tables were to be migrated before the account tables they point at.
var ErrNoAccounts = errors.New("sqlite: the core tables need the account tables, so migrate the accounts first")

// MigrateOptions sets how a run of Migrate waits for the migration lock of the database file.
type MigrateOptions struct {
	LockWait time.Duration
	LockPoll time.Duration
}

// MigrateOptionsFrom reads the migration lock settings from env, each one left unset taking its documented default.
func MigrateOptionsFrom(env gonsole.Env) (MigrateOptions, error) {
	wait, err := env.Duration("SQLITE_MIGRATE_LOCK_WAIT", 30*time.Second)
	if err != nil {
		return MigrateOptions{}, err
	}
	poll, err := env.Duration("SQLITE_MIGRATE_LOCK_POLL", 250*time.Millisecond)
	if err != nil {
		return MigrateOptions{}, err
	}
	return MigrateOptions{LockWait: wait, LockPoll: poll}, nil
}

// Migrate applies the core table migrations to db under the migration lock of its file, after the account tables.
func Migrate(ctx context.Context, db *sql.DB, opts MigrateOptions) error {
	if err := checkAccounts(ctx, db); err != nil {
		return err
	}
	err := dbsqlite.Migrate(ctx, db, dbsqlite.Migrations{
		Table: versionTable, FS: migrationSource, LockWait: opts.LockWait, LockPoll: opts.LockPoll,
	})
	if err != nil {
		return fmt.Errorf("sqlite: apply migrations: %w", dbsqlite.Classify(err))
	}
	return nil
}

// checkAccounts returns ErrNoAccounts when db holds no account table for the core tables to point at.
func checkAccounts(ctx context.Context, db *sql.DB) error {
	var found bool
	if err := db.QueryRowContext(ctx, accountsLookup).Scan(&found); err != nil {
		return fmt.Errorf("sqlite: look for the account tables: %w", err)
	}
	if !found {
		return ErrNoAccounts
	}
	return nil
}

// mustSub returns the subtree of fsys at dir, and panics when dir is not a valid path.
func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
