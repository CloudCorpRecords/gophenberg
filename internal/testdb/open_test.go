// SPDX-License-Identifier: Apache-2.0

package testdb_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/testdb"
)

// offlineAddress names a server no test reaches, which Open never dials.
const offlineAddress = "postgres://postgres@db.example.com/postgres"

// fatalOfOpen returns the fatal message Open stopped with on address, or nothing when it returned.
func fatalOfOpen(t *testing.T, address string) string {
	t.Helper()
	recorder := &fatal{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		testdb.Open(recorder, address)
	}()
	<-done
	return recorder.message
}

func TestOpenNamesTheConnectionCapItRefuses(t *testing.T) {
	t.Setenv("GOPHENBERG_DATABASE_MAX_CONNS", "1")

	message := fatalOfOpen(t, offlineAddress)

	if want := `testdb: GOPHENBERG_DATABASE_MAX_CONNS: must be 2 or more, got "1"`; message != want {
		t.Errorf("Open() stopped with %q, want %q", message, want)
	}
}

func TestOpenRefusesAnAddressDbkitRefuses(t *testing.T) {
	t.Setenv("GOPHENBERG_DATABASE_MAX_CONNS", "")

	message := fatalOfOpen(t, offlineAddress+"?pool_max_conns=4")

	if !strings.Contains(message, dbkit.ErrAddress.Error()) {
		t.Errorf("Open() stopped with %q, want the dbkit error for the address", message)
	}
}

func TestOpenHoldsTheHandleAtTheConnectionCap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	t.Setenv("GOPHENBERG_DATABASE_MAX_CONNS", "2")
	db := testdb.Open(t, testdb.New(t, pgtestdb.NoopMigrator{}))
	for range 2 {
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("taking a connection under the cap: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
	}
	waiting, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	third, err := db.Conn(waiting)
	if err == nil {
		t.Cleanup(func() { _ = third.Close() })
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a third connection under a cap of 2 = %v, want the wait to run out", err)
	}
}

// configOf returns the pgtestdb configuration of the database at address.
func configOf(t *testing.T, address string) pgtestdb.Config {
	t.Helper()
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatalf("parsing the test database address: %v", err)
	}
	password, _ := parsed.User.Password()
	return pgtestdb.Config{
		DriverName: "pgx", Host: parsed.Hostname(), Port: parsed.Port(), User: parsed.User.Username(),
		Password: password, Database: strings.TrimPrefix(parsed.Path, "/"), Options: parsed.RawQuery,
	}
}

func TestMigratorBuildsBothSchemasOnTheDatabaseItIsHanded(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	address := testdb.New(t, pgtestdb.NoopMigrator{})

	if err := testdb.Migrator().Migrate(t.Context(), nil, configOf(t, address)); err != nil {
		t.Fatalf("Migrate() error = %v, want both schemas built", err)
	}

	db := testdb.Open(t, address)
	for _, table := range []string{"auth.users", "core.content"} {
		var found *string
		if err := db.QueryRow("SELECT to_regclass($1)::text", table).Scan(&found); err != nil || found == nil {
			t.Errorf("table %s = %v (%v) after Migrate(), want it built", table, found, err)
		}
	}
}
