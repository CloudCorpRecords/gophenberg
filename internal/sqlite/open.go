// SPDX-License-Identifier: Apache-2.0

// Package sqlite provides the SQLite data layer for the CMS core.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/dbconns"
)

// maxBusyTimeout is the longest busy timeout SQLite keeps.
const maxBusyTimeout = math.MaxInt32 * time.Millisecond

// ErrNoFile reports that the database file a sqlite: address names does not exist.
var ErrNoFile = errors.New("sqlite: the database file does not exist")

// Settings are the values every connection of the site's SQLite handle runs with.
type Settings struct {
	BusyTimeout      time.Duration
	BusyPoll         time.Duration
	CacheSize        int
	MaxConns         int
	Synchronous      dbsqlite.Synchronous
	JournalSizeLimit *int64
}

// SettingsFrom reads the SQLite settings from env, each one left unset taking its documented default.
func SettingsFrom(env gonsole.Env) (Settings, error) {
	maxConns, err := dbconns.MaxConnsFrom(env)
	if err != nil {
		return Settings{}, err
	}
	busyTimeout, err := env.Duration("SQLITE_BUSY_TIMEOUT", 5*time.Second,
		gonsole.WholeMilliseconds(), gonsole.AtMost(int64(maxBusyTimeout)))
	if err != nil {
		return Settings{}, err
	}
	busyPoll, err := busyPollFrom(env, busyTimeout)
	if err != nil {
		return Settings{}, err
	}
	cacheSize, err := env.Count("SQLITE_CACHE_SIZE", 2000, gonsole.AtMost(math.MaxInt32))
	if err != nil {
		return Settings{}, err
	}
	synchronous, err := gonsole.Parse(env, "SQLITE_SYNCHRONOUS", dbsqlite.SynchronousFull, parseSynchronous)
	if err != nil {
		return Settings{}, err
	}
	journalSizeLimit, err := gonsole.Parse[*int64](env, "SQLITE_JOURNAL_SIZE_LIMIT", nil, parseJournalSizeLimit)
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		BusyTimeout: busyTimeout, BusyPoll: busyPoll, CacheSize: cacheSize, MaxConns: maxConns,
		Synchronous: synchronous, JournalSizeLimit: journalSizeLimit,
	}, nil
}

// busyPollFrom reads how often opening a connection tries a lock again, up to busyTimeout, which caps the 10ms default.
func busyPollFrom(env gonsole.Env, busyTimeout time.Duration) (time.Duration, error) {
	busyPoll, err := env.Duration("SQLITE_BUSY_POLL", min(10*time.Millisecond, busyTimeout), gonsole.WholeMilliseconds())
	if err != nil {
		return 0, err
	}
	if busyPoll > busyTimeout {
		return 0, fmt.Errorf("%s: must stand at or below the %v of %s, got %q",
			env.Key("SQLITE_BUSY_POLL"), busyTimeout, env.Key("SQLITE_BUSY_TIMEOUT"), env.Value("SQLITE_BUSY_POLL"))
	}
	return busyPoll, nil
}

// parseSynchronous reads full or normal, in any case, as the Synchronous it names.
func parseSynchronous(value string) (dbsqlite.Synchronous, error) {
	switch strings.ToLower(value) {
	case "full":
		return dbsqlite.SynchronousFull, nil
	case "normal":
		return dbsqlite.SynchronousNormal, nil
	}
	return 0, fmt.Errorf("must be full or normal, got %q", value)
}

// parseJournalSizeLimit reads a whole number of bytes above zero.
func parseJournalSizeLimit(value string) (*int64, error) {
	limit, err := strconv.ParseInt(value, 10, 64)
	if err != nil || limit < 1 {
		return nil, fmt.Errorf("must be a whole number of bytes above zero, got %q", value)
	}
	return &limit, nil
}

// Options returns the dbkit options of s with every Go function of the site registered.
func (s Settings) Options() dbsqlite.Options {
	return dbsqlite.Options{
		BusyTimeout: s.BusyTimeout, BusyPoll: s.BusyPoll, CacheSize: s.CacheSize, MaxConns: s.MaxConns,
		Synchronous: s.Synchronous, JournalSizeLimit: s.JournalSizeLimit, Functions: functions,
	}
}

// Open returns a handle on the site's SQLite file at address under s with every rule on, and connects to nothing.
func Open(address string, s Settings) (*sql.DB, error) {
	if err := checkExists(address); err != nil {
		return nil, err
	}
	return dbsqlite.Open(address, s.Options())
}

// checkExists returns ErrNoFile when the absolute path of address names no file, and leaves every other check to dbkit.
func checkExists(address string) error {
	path, err := dbkit.SQLitePath(address)
	if err != nil || !filepath.IsAbs(path) {
		return nil
	}
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s, create an empty file at that path to start a new site", ErrNoFile, path)
	}
	return nil
}
