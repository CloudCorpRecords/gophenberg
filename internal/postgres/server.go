// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// OldestMajor is the oldest PostgreSQL major version this release supports.
const OldestMajor = 15

const (
	// majorStep is how far apart two PostgreSQL majors sit in server_version_num.
	majorStep = 10000
	// serverLookup reads the server's version number and the version it shows.
	serverLookup = "SELECT current_setting('server_version_num')::int, current_setting('server_version')"
	// superuserLookup reads whether the role the session runs as holds superuser rights.
	superuserLookup = "SELECT rolsuper FROM pg_roles WHERE rolname = current_user"
)

// CheckMajor returns an error naming the server's version when it runs a PostgreSQL major older than OldestMajor.
func CheckMajor(ctx context.Context, db *sql.DB) error {
	var number int
	var shown string
	if err := db.QueryRowContext(ctx, serverLookup).Scan(&number, &shown); err != nil {
		return fmt.Errorf("postgres: read the server version: %w", err)
	}
	return majorRefusal(number, shown)
}

// majorRefusal returns an error naming shown when the version number belongs to a major older than OldestMajor.
func majorRefusal(number int, shown string) error {
	if number/majorStep >= OldestMajor {
		return nil
	}
	return fmt.Errorf("this database runs PostgreSQL %s and Gophenberg needs PostgreSQL %d or newer", shown, OldestMajor)
}

// Superuser reports whether db connects as a role that holds superuser rights.
func Superuser(ctx context.Context, db *sql.DB) (bool, error) {
	var super bool
	if err := db.QueryRowContext(ctx, superuserLookup).Scan(&super); err != nil {
		return false, fmt.Errorf("postgres: read the rights of the connected role: %w", err)
	}
	return super, nil
}
