// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

const (
	// heldTimeout is the busy timeout of a handle whose write waits behind a lock that is never let go.
	heldTimeout = 300 * time.Millisecond
	// slowPoll is a busy poll much longer than letGoAfter, so the next try comes well after the lock is free.
	slowPoll = time.Second
	// letGoAfter is how long another connection holds the write lock before it lets go.
	letGoAfter = 300 * time.Millisecond
	// waitMargin is how much longer than the wanted time a wait may take on a busy machine.
	waitMargin = 2 * time.Second
)

// testSettings returns the settings a test handle opens with: four connections and a 5s busy timeout tried every 10ms.
func testSettings() sqlite.Settings {
	return sqlite.Settings{
		BusyTimeout: 5 * time.Second, BusyPoll: 10 * time.Millisecond, CacheSize: 2048, MaxConns: 4,
		Synchronous: dbsqlite.SynchronousFull,
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
	want := sqlite.Settings{
		BusyTimeout: 5 * time.Second, BusyPoll: 10 * time.Millisecond, CacheSize: 2000,
		Synchronous: dbsqlite.SynchronousFull,
	}
	if got != want {
		t.Errorf("SettingsFrom() = %+v, want %+v", got, want)
	}
}

func TestSettingsReadEveryValueFromTheEnvironment(t *testing.T) {
	t.Parallel()

	got, err := sqlite.SettingsFrom(environment(map[string]string{
		"GOPHENBERG_DATABASE_MAX_CONNS":        "3",
		"GOPHENBERG_SQLITE_BUSY_TIMEOUT":       "1500ms",
		"GOPHENBERG_SQLITE_BUSY_POLL":          "25ms",
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
		BusyTimeout: 1500 * time.Millisecond, BusyPoll: 25 * time.Millisecond, CacheSize: 4096, MaxConns: 3,
		Synchronous: dbsqlite.SynchronousNormal,
	}
	if got != want {
		t.Errorf("SettingsFrom() = %+v, want %+v", got, want)
	}
}

func TestSettingsTakeTheEdgeValues(t *testing.T) {
	t.Parallel()

	got, err := sqlite.SettingsFrom(environment(map[string]string{
		"GOPHENBERG_SQLITE_BUSY_TIMEOUT":       "596h31m23.647s",
		"GOPHENBERG_SQLITE_BUSY_POLL":          "596h31m23.647s",
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
		BusyTimeout: math.MaxInt32 * time.Millisecond, BusyPoll: math.MaxInt32 * time.Millisecond,
		CacheSize: math.MaxInt32, Synchronous: dbsqlite.SynchronousFull,
	}
	if got != want {
		t.Errorf("SettingsFrom() = %+v, want %+v", got, want)
	}
}

func TestSettingsTakeABusyPollFromOneMillisecondUpToTheBusyTimeout(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		env  map[string]string
		want time.Duration
	}{
		{"1ms", map[string]string{"GOPHENBERG_SQLITE_BUSY_POLL": "1ms"}, time.Millisecond},
		{"as long as the default busy timeout", map[string]string{"GOPHENBERG_SQLITE_BUSY_POLL": "5s"}, 5 * time.Second},
		{"as long as a busy timeout set to 1500ms", map[string]string{
			"GOPHENBERG_SQLITE_BUSY_TIMEOUT": "1500ms", "GOPHENBERG_SQLITE_BUSY_POLL": "1500ms",
		}, 1500 * time.Millisecond},
		{"unset under a busy timeout of 4ms", map[string]string{"GOPHENBERG_SQLITE_BUSY_TIMEOUT": "4ms"},
			4 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := sqlite.SettingsFrom(environment(tc.env))

			if err != nil || got.BusyPoll != tc.want {
				t.Errorf("SettingsFrom(%v).BusyPoll = %v with error %v, want %v", tc.env, got.BusyPoll, err, tc.want)
			}
		})
	}
}

func TestSettingsRefuseABusyPollAboveTheBusyTimeoutNamingBoth(t *testing.T) {
	t.Parallel()

	_, err := sqlite.SettingsFrom(environment(map[string]string{
		"GOPHENBERG_SQLITE_BUSY_TIMEOUT": "1s", "GOPHENBERG_SQLITE_BUSY_POLL": "1001ms",
	}))

	want := `GOPHENBERG_SQLITE_BUSY_POLL: must stand at or below the 1s of GOPHENBERG_SQLITE_BUSY_TIMEOUT, got "1001ms"`
	if err == nil || err.Error() != want {
		t.Errorf("SettingsFrom() error = %v, want %q", err, want)
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
		{"GOPHENBERG_SQLITE_BUSY_POLL", "0s"},
		{"GOPHENBERG_SQLITE_BUSY_POLL", "-10ms"},
		{"GOPHENBERG_SQLITE_BUSY_POLL", "1500us"},
		{"GOPHENBERG_SQLITE_BUSY_POLL", "5001ms"},
		{"GOPHENBERG_SQLITE_CACHE_SIZE", "0"},
		{"GOPHENBERG_SQLITE_CACHE_SIZE", "2147483648"},
		{"GOPHENBERG_SQLITE_SYNCHRONOUS", "off"},
		{"GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT", "0"},
		{"GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT", "lots"},
		{"GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT", "9223372036854775808"},
	} {
		_, err := sqlite.SettingsFrom(environment(map[string]string{bad.name: bad.value}))
		if err == nil || !strings.Contains(err.Error(), bad.name) ||
			!strings.Contains(err.Error(), strconv.Quote(bad.value)) {
			t.Errorf("SettingsFrom() with %s=%s error = %v, want one naming the setting and quoting the value",
				bad.name, bad.value, err)
		}
	}
}

func TestSettingsHandDbkitEachValueAsSet(t *testing.T) {
	t.Parallel()

	limit := int64(3145728)
	settings := sqlite.Settings{
		BusyTimeout: 1234 * time.Millisecond, BusyPoll: 25 * time.Millisecond, CacheSize: 3000, MaxConns: 3,
		Synchronous: dbsqlite.SynchronousNormal, JournalSizeLimit: &limit,
	}

	got := settings.Options()

	if got.Functions == nil {
		t.Errorf("Options().Functions = nil, want the site's functions")
	}
	got.Functions = nil
	want := dbsqlite.Options{
		BusyTimeout: 1234 * time.Millisecond, BusyPoll: 25 * time.Millisecond, CacheSize: 3000, MaxConns: 3,
		Synchronous: dbsqlite.SynchronousNormal, JournalSizeLimit: &limit,
	}
	if got != want {
		t.Errorf("Options() = %+v, want %+v", got, want)
	}
}

func TestOpenRunsEveryFreshConnectionUnderTheSettingsAndTheRules(t *testing.T) {
	t.Parallel()

	address, _ := emptyFile(t)
	limit := int64(3145728)
	settings := sqlite.Settings{
		BusyTimeout: 1234 * time.Millisecond, BusyPoll: 10 * time.Millisecond, CacheSize: 3000, MaxConns: 3,
		Synchronous: dbsqlite.SynchronousNormal, JournalSizeLimit: &limit,
	}

	db := open(t, address, settings)

	conns := fullPool(t, db, settings.MaxConns)
	for pragma, want := range map[string]string{
		"cache_size": "-3000", "synchronous": "1", "journal_size_limit": "3145728",
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

// holderAndWaiter returns two connections of a handle on a new file under settings, the first holding the write lock.
func holderAndWaiter(t *testing.T, settings sqlite.Settings) (holder, waiter *sql.Conn) {
	t.Helper()
	address, _ := emptyFile(t)
	conns := fullPool(t, open(t, address, settings), 2)
	if _, err := conns[0].ExecContext(t.Context(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatalf("BEGIN IMMEDIATE error = %v, want nil", err)
	}
	return conns[0], conns[1]
}

func TestOpenWaitsTheBusyTimeoutForALockAnotherConnectionHolds(t *testing.T) {
	t.Parallel()

	settings := testSettings()
	settings.BusyTimeout = heldTimeout
	_, waiter := holderAndWaiter(t, settings)

	began := time.Now()
	_, err := waiter.ExecContext(t.Context(), "BEGIN IMMEDIATE")
	waited := time.Since(began)

	if !errors.Is(dbsqlite.Classify(err), dbkit.ErrBusy) {
		t.Errorf("BEGIN IMMEDIATE behind another writer error = %v, want dbkit.ErrBusy", err)
	}
	if waited < heldTimeout || waited > heldTimeout+waitMargin {
		t.Errorf("BEGIN IMMEDIATE behind another writer answered after %v, want about the busy timeout %v",
			waited, heldTimeout)
	}
}

func TestOpenTriesALockAgainOnceEachBusyPoll(t *testing.T) {
	t.Parallel()

	settings := testSettings()
	settings.BusyPoll = slowPoll
	holder, waiter := holderAndWaiter(t, settings)
	letGo := make(chan error, 1)
	time.AfterFunc(letGoAfter, func() {
		_, err := holder.ExecContext(t.Context(), "ROLLBACK")
		letGo <- err
	})

	began := time.Now()
	_, err := waiter.ExecContext(t.Context(), "BEGIN IMMEDIATE")
	waited := time.Since(began)

	if released := <-letGo; released != nil {
		t.Fatalf("ROLLBACK of the other writer error = %v, want nil", released)
	}
	if err != nil {
		t.Fatalf("BEGIN IMMEDIATE once the other writer let go error = %v, want nil", err)
	}
	if waited < slowPoll || waited > slowPoll+waitMargin {
		t.Errorf("BEGIN IMMEDIATE behind a writer that let go after %v answered after %v, want about the busy poll %v",
			letGoAfter, waited, slowPoll)
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
