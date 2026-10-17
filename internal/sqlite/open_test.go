// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

// testSettings returns the settings a test handle opens with, four connections and a five second busy timeout.
func testSettings() sqlite.Settings {
	return sqlite.Settings{
		BusyTimeout: 5 * time.Second, CacheSize: 2048, MaxConns: 4, Synchronous: dbsqlite.SynchronousFull,
	}
}

// environment returns the Gophenberg settings reader over values, each key unset when values lacks it.
func environment(values map[string]string) gonsole.Env {
	return gonsole.Env{Prefix: "GOPHENBERG_", Getenv: testkit.Getenv(values)}
}

// emptyFile creates an empty database file in a fresh folder of t and returns its address and path.
func emptyFile(t *testing.T) (address, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "site.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("creating the empty database file: %v", err)
	}
	return "sqlite:" + path, path
}

// open returns the handle Open gives on address under settings, closed when t ends.
func open(t *testing.T, address string, settings sqlite.Settings) *sql.DB {
	t.Helper()
	db, err := sqlite.Open(address, settings)
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
	})
	return db
}

// fullPool takes every connection of db up to size, each opened after the ones before, closed when t ends.
func fullPool(t *testing.T, db *sql.DB, size int) []*sql.Conn {
	t.Helper()
	conns := make([]*sql.Conn, size)
	for i := range conns {
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("Conn() error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		conns[i] = conn
	}
	if got := db.Stats().OpenConnections; got != size {
		t.Fatalf("Stats().OpenConnections = %d, want %d fresh connections", got, size)
	}
	return conns
}

func TestSettingsFallBackToTheDocumentedDefaults(t *testing.T) {
	t.Parallel()

	got, err := sqlite.SettingsFrom(environment(nil))
	if err != nil {
		t.Fatalf("SettingsFrom() error = %v, want nil", err)
	}

	got.MaxConns = 0
	want := sqlite.Settings{BusyTimeout: 5 * time.Second, CacheSize: 2000, Synchronous: dbsqlite.SynchronousFull}
	if got != want {
		t.Errorf("SettingsFrom() = %+v, want %+v", got, want)
	}
}

func TestSettingsReadEveryValueFromTheEnvironment(t *testing.T) {
	t.Parallel()

	got, err := sqlite.SettingsFrom(environment(map[string]string{
		"GOPHENBERG_DATABASE_MAX_CONNS":        "3",
		"GOPHENBERG_SQLITE_BUSY_TIMEOUT":       "1500ms",
		"GOPHENBERG_SQLITE_CACHE_SIZE":         "4096",
		"GOPHENBERG_SQLITE_SYNCHRONOUS":        "normal",
		"GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT": "3145728",
	}))
	if err != nil {
		t.Fatalf("SettingsFrom() error = %v, want nil", err)
	}

	if got.JournalSizeLimit == nil || *got.JournalSizeLimit != 3145728 {
		t.Fatalf("SettingsFrom().JournalSizeLimit = %v, want 3145728", got.JournalSizeLimit)
	}
	got.JournalSizeLimit = nil
	want := sqlite.Settings{
		BusyTimeout: 1500 * time.Millisecond, CacheSize: 4096, MaxConns: 3, Synchronous: dbsqlite.SynchronousNormal,
	}
	if got != want {
		t.Errorf("SettingsFrom() = %+v, want %+v", got, want)
	}
}

func TestSettingsTakeTheEdgeValues(t *testing.T) {
	t.Parallel()

	got, err := sqlite.SettingsFrom(environment(map[string]string{
		"GOPHENBERG_SQLITE_BUSY_TIMEOUT":       "596h31m23.647s",
		"GOPHENBERG_SQLITE_CACHE_SIZE":         "2147483647",
		"GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT": "1",
	}))
	if err != nil {
		t.Fatalf("SettingsFrom() error = %v, want nil", err)
	}

	if got.JournalSizeLimit == nil || *got.JournalSizeLimit != 1 {
		t.Fatalf("SettingsFrom().JournalSizeLimit = %v, want 1", got.JournalSizeLimit)
	}
	got.JournalSizeLimit = nil
	got.MaxConns = 0
	want := sqlite.Settings{
		BusyTimeout: math.MaxInt32 * time.Millisecond, CacheSize: math.MaxInt32, Synchronous: dbsqlite.SynchronousFull,
	}
	if got != want {
		t.Errorf("SettingsFrom() = %+v, want %+v", got, want)
	}
}

func TestSettingsReadSynchronousInAnyCase(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]dbsqlite.Synchronous{
		"FULL": dbsqlite.SynchronousFull, "Normal": dbsqlite.SynchronousNormal,
	} {
		got, err := sqlite.SettingsFrom(environment(map[string]string{"GOPHENBERG_SQLITE_SYNCHRONOUS": value}))
		if err != nil {
			t.Fatalf("SettingsFrom() with synchronous %q error = %v, want nil", value, err)
		}
		if got.Synchronous != want {
			t.Errorf("SettingsFrom() with synchronous %q = %d, want %d", value, got.Synchronous, want)
		}
	}
}

func TestSettingsRefuseABadValueByItsName(t *testing.T) {
	t.Parallel()

	for _, bad := range []struct{ name, value string }{
		{"GOPHENBERG_DATABASE_MAX_CONNS", "1"},
		{"GOPHENBERG_SQLITE_BUSY_TIMEOUT", "0s"},
		{"GOPHENBERG_SQLITE_BUSY_TIMEOUT", "1500us"},
		{"GOPHENBERG_SQLITE_BUSY_TIMEOUT", "596h31m23.648s"},
		{"GOPHENBERG_SQLITE_CACHE_SIZE", "0"},
		{"GOPHENBERG_SQLITE_CACHE_SIZE", "2147483648"},
		{"GOPHENBERG_SQLITE_SYNCHRONOUS", "off"},
		{"GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT", "0"},
		{"GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT", "lots"},
		{"GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT", "9223372036854775808"},
	} {
		_, err := sqlite.SettingsFrom(environment(map[string]string{bad.name: bad.value}))
		if err == nil || !strings.Contains(err.Error(), bad.name) {
			t.Errorf("SettingsFrom() with %s=%s error = %v, want one naming the setting", bad.name, bad.value, err)
		}
	}
}

func TestOpenRunsEveryFreshConnectionUnderTheSettingsAndTheRules(t *testing.T) {
	t.Parallel()

	address, _ := emptyFile(t)
	limit := int64(3145728)
	settings := sqlite.Settings{
		BusyTimeout: 1234 * time.Millisecond, CacheSize: 3000, MaxConns: 3,
		Synchronous: dbsqlite.SynchronousNormal, JournalSizeLimit: &limit,
	}

	db := open(t, address, settings)

	conns := fullPool(t, db, settings.MaxConns)
	for pragma, want := range map[string]string{
		"busy_timeout": "1234", "cache_size": "-3000", "synchronous": "1", "journal_size_limit": "3145728",
		"foreign_keys": "1", "journal_mode": "wal", "temp_store": "2",
	} {
		for i, conn := range conns {
			var got string
			if err := conn.QueryRowContext(t.Context(), "PRAGMA "+pragma).Scan(&got); err != nil {
				t.Fatalf("PRAGMA %s on connection %d error = %v, want nil", pragma, i, err)
			}
			if got != want {
				t.Errorf("PRAGMA %s on connection %d = %s, want %s", pragma, i, got, want)
			}
		}
	}
	if got := db.Stats().MaxOpenConnections; got != settings.MaxConns {
		t.Errorf("Stats().MaxOpenConnections = %d, want %d", got, settings.MaxConns)
	}
}

func TestOpenRunsFullSynchronousByDefault(t *testing.T) {
	t.Parallel()

	address, _ := emptyFile(t)

	db := open(t, address, testSettings())

	var got string
	if err := db.QueryRowContext(t.Context(), "PRAGMA synchronous").Scan(&got); err != nil {
		t.Fatalf("PRAGMA synchronous error = %v, want nil", err)
	}
	if got != "2" {
		t.Errorf("PRAGMA synchronous = %s, want 2 for full", got)
	}
}

func TestOpenConnectsToNothingBeforeTheFirstQuery(t *testing.T) {
	t.Parallel()

	address, path := emptyFile(t)

	open(t, address, testSettings())

	if _, err := os.Lstat(path + "-wal"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(%s-wal) error = %v, want no write-ahead log before the first query", path, err)
	}
}

func TestOpenRefusesAMissingFileAndSaysToCreateIt(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "site.db")

	_, err := sqlite.Open("sqlite:"+path, testSettings())

	if !errors.Is(err, sqlite.ErrNoFile) {
		t.Fatalf("Open() error = %v, want ErrNoFile", err)
	}
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "create an empty file") {
		t.Errorf("Open() error = %q, want the path and how to start a new site", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want the missing file left uncreated", statErr)
	}
}

func TestOpenLeavesALinkToAMissingFileToDbkit(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	link := filepath.Join(folder, "site.db")
	if err := os.Symlink(filepath.Join(folder, "gone.db"), link); err != nil {
		t.Fatalf("Symlink() error = %v, want nil", err)
	}

	_, err := sqlite.Open("sqlite:"+link, testSettings())

	if err == nil || errors.Is(err, sqlite.ErrNoFile) {
		t.Errorf("Open() on a link to a missing file error = %v, want the link refused by dbkit", err)
	}
}

func TestOpenLeavesAPathUnderAPlainFileToDbkit(t *testing.T) {
	t.Parallel()

	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(plain, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v, want nil", err)
	}

	_, err := sqlite.Open("sqlite:"+filepath.Join(plain, "site.db"), testSettings())

	if err == nil || errors.Is(err, sqlite.ErrNoFile) {
		t.Errorf("Open() on a path under a plain file error = %v, want the path refused by dbkit", err)
	}
}

func TestOpenRefusesAnAddressThatNamesNoSQLiteFile(t *testing.T) {
	t.Parallel()

	_, err := sqlite.Open("postgres://localhost/site", testSettings())

	if !errors.Is(err, dbkit.ErrAddress) {
		t.Errorf("Open() error = %v, want dbkit.ErrAddress", err)
	}
}

func TestOpenRefusesARelativePath(t *testing.T) {
	t.Parallel()

	_, err := sqlite.Open("sqlite:site.db", testSettings())

	if err == nil || errors.Is(err, sqlite.ErrNoFile) {
		t.Errorf("Open() error = %v, want the relative path refused", err)
	}
}
