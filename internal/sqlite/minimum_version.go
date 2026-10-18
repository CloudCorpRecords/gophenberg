// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// minimumVersionKey is the site setting that holds the oldest version allowed to run on the database.
	minimumVersionKey = "minimum_version"
	// settingsLookup reports whether the file holds the settings table the minimum version is kept in.
	settingsLookup = "SELECT EXISTS (SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = 'core_settings')"
	// minimumVersionLookup reads the stored minimum version.
	minimumVersionLookup = "SELECT value FROM core_settings WHERE key = ?"
	// minimumVersionRaise stores a minimum version in place of the one read before it, unless one was stored since.
	minimumVersionRaise = `INSERT INTO core_settings (key, value) VALUES (?1, ?2)
ON CONFLICT (key) DO UPDATE SET value = excluded.value WHERE core_settings.value = ?3`
)

// StoredMinimumVersion returns the minimum version stored on db, empty before the core tables exist or while none is.
func StoredMinimumVersion(ctx context.Context, db *sql.DB) (string, error) {
	var ready bool
	if err := db.QueryRowContext(ctx, settingsLookup).Scan(&ready); err != nil || !ready {
		return "", failed("look for the settings table", err)
	}
	var minimum string
	err := db.QueryRowContext(ctx, minimumVersionLookup, minimumVersionKey).Scan(&minimum)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return minimum, failed("read the stored minimum version", err)
}

// RaiseMinimumVersion stores minimum on db when the stored one comes before it, never over one stored since.
func RaiseMinimumVersion(ctx context.Context, db *sql.DB, minimum string) error {
	stored, err := StoredMinimumVersion(ctx, db)
	if err != nil || !version.Before(stored, minimum) {
		return err
	}
	_, err = db.ExecContext(ctx, minimumVersionRaise, minimumVersionKey, minimum, stored)
	return failed(fmt.Sprintf("raise the stored minimum version to %s", minimum), err)
}
