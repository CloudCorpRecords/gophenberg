// SPDX-License-Identifier: Apache-2.0

// Package testdb provides fresh PostgreSQL test databases on the server GOPHENBERG_TEST_DATABASE_URL names.
package testdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit/postgres/pgtest"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/joho/godotenv"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/postgres"
)

const (
	// addressKey names the variable that holds the address of the test database server.
	addressKey = "GOPHENBERG_TEST_DATABASE_URL"
	// maxAgeKey names the variable that holds the age past which a stale test database is swept.
	maxAgeKey = "GOPHENBERG_TEST_DATABASE_MAX_AGE"
	// whereToSet tells the reader of a refused setting where to set it.
	whereToSet = "in the environment or .env, see .env.example"
)

// rootDotenv reads the .env entries of the module the test process started in, once per process.
var rootDotenv = dotenvOnce(os.Getwd())

// New returns the address of a fresh database that migrator builds on the test server, after a sweep of stale ones.
func New(t testing.TB, migrator pgtestdb.Migrator) string {
	t.Helper()
	address, maxAge, err := settings(os.LookupEnv, rootDotenv)
	if err != nil {
		t.Fatal(err)
	}
	return pgtest.NewSwept(t, address, maxAge, migrator)
}

// settings returns the test server address and the sweep age, each read from environ first and from file second.
func settings(
	environ func(string) (string, bool), file func() (map[string]string, error),
) (string, time.Duration, error) {
	address, err := value(environ, file, addressKey)
	if err != nil {
		return "", 0, err
	}
	if address == "" {
		return "", 0, fmt.Errorf("testdb: set %s %s", addressKey, whereToSet)
	}
	text, err := value(environ, file, maxAgeKey)
	if err != nil {
		return "", 0, err
	}
	maxAge, err := time.ParseDuration(text)
	if err != nil || maxAge <= 0 {
		return "", 0, fmt.Errorf("testdb: set %s to a Go duration above zero, such as 1h, %s, got %q",
			maxAgeKey, whereToSet, text)
	}
	return address, maxAge, nil
}

// value returns the trimmed value of key from environ, or reads it from file only when environ does not set it.
func value(environ func(string) (string, bool), file func() (map[string]string, error), key string) (string, error) {
	if text, ok := environ(key); ok {
		return strings.TrimSpace(text), nil
	}
	entries, err := file()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(entries[key]), nil
}

// dotenvOnce returns a reader of the .env beside the nearest go.mod at or above dir that reads it once.
func dotenvOnce(dir string, dirErr error) func() (map[string]string, error) {
	return sync.OnceValues(func() (map[string]string, error) {
		if dirErr != nil {
			return nil, fmt.Errorf("testdb: find the working directory: %w", dirErr)
		}
		return dotenv(dir)
	})
}

// dotenv returns the entries of the .env file beside the nearest go.mod at or above dir, or none without one.
func dotenv(dir string) (map[string]string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			path := filepath.Join(dir, ".env")
			entries, err := godotenv.Read(path)
			if errors.Is(err, fs.ErrNotExist) {
				return map[string]string{}, nil
			}
			if err != nil {
				return nil, fmt.Errorf("testdb: read %s: %w", path, err)
			}
			return entries, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return map[string]string{}, nil
		}
		dir = parent
	}
}

// Migrator returns a pgtestdb migrator that applies the authkit migrations and then the core ones.
func Migrator() pgtestdb.Migrator {
	return migrator{}
}

type migrator struct{}

// Hash fingerprints the authkit and core migration files together.
func (migrator) Hash() (string, error) {
	h := sha256.New()
	sources := []struct {
		label string
		files fs.FS
	}{
		{"auth", authkitpg.Migrations},
		{"core", postgres.Migrations},
	}
	for _, source := range sources {
		err := fs.WalkDir(source.files, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			content, err := fs.ReadFile(source.files, path)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(h, source.label, ":", path)
			_, _ = h.Write(content)
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("testdb: hash %s migrations: %w", source.label, err)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Migrate applies the authkit migrations and then the core ones to the template database.
func (migrator) Migrate(ctx context.Context, _ *sql.DB, cfg pgtestdb.Config) error {
	if err := authkitpg.Migrate(ctx, cfg.URL()); err != nil {
		return fmt.Errorf("testdb: auth migrations: %w", err)
	}
	if err := postgres.Migrate(ctx, cfg.URL()); err != nil {
		return fmt.Errorf("testdb: core migrations: %w", err)
	}
	return nil
}
