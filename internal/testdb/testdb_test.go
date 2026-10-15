// SPDX-License-Identifier: Apache-2.0

package testdb_test

import (
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/testdb"
)

// where is the hint every refused setting gives about where to set it.
const where = "in the environment or .env, see .env.example"

// fatal is a test that records its fatal message and ends its goroutine on a fatal call.
type fatal struct {
	testing.TB
	// message is the text of the fatal call that ended the goroutine.
	message string
}

// Fatal records args and ends the calling goroutine.
func (f *fatal) Fatal(args ...any) {
	f.message = fmt.Sprint(args...)
	runtime.Goexit()
}

// Fatalf records the message of format and args and ends the calling goroutine.
func (f *fatal) Fatalf(format string, args ...any) {
	f.message = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// fatalOfNew returns the fatal message New stopped with, or nothing when it returned.
func fatalOfNew(t *testing.T) string {
	t.Helper()
	recorder := &fatal{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		testdb.New(recorder, testdb.Migrator())
	}()
	<-done
	return recorder.message
}

func TestNewNamesAMissingAddressAndWhereToSetIt(t *testing.T) {
	t.Setenv("GOPHENBERG_TEST_DATABASE_URL", "")
	t.Setenv("GOPHENBERG_TEST_DATABASE_MAX_AGE", "1h")

	message := fatalOfNew(t)
	if !strings.Contains(message, "GOPHENBERG_TEST_DATABASE_URL") || !strings.Contains(message, where) {
		t.Errorf("New() stopped with %q, want a message naming GOPHENBERG_TEST_DATABASE_URL and %q", message, where)
	}
}

func TestNewNamesABadMaxAgeAndWhereToSetIt(t *testing.T) {
	t.Setenv("GOPHENBERG_TEST_DATABASE_URL", "postgres://postgres@db.example.com/postgres")
	t.Setenv("GOPHENBERG_TEST_DATABASE_MAX_AGE", "0s")

	message := fatalOfNew(t)
	if !strings.Contains(message, "GOPHENBERG_TEST_DATABASE_MAX_AGE") || !strings.Contains(message, where) {
		t.Errorf("New() stopped with %q, want a message naming GOPHENBERG_TEST_DATABASE_MAX_AGE and %q", message, where)
	}
}

func TestNewNamesTheSettingOfARefusedAddress(t *testing.T) {
	t.Setenv("GOPHENBERG_TEST_DATABASE_URL", "mysql://postgres@db.example.com/postgres")
	t.Setenv("GOPHENBERG_TEST_DATABASE_MAX_AGE", "1h")

	message := fatalOfNew(t)
	if !strings.HasPrefix(message, "testdb: set GOPHENBERG_TEST_DATABASE_URL to an address pgtest takes") {
		t.Errorf("New() stopped with %q, want the setting of the refused address named first", message)
	}
	if !strings.Contains(message, "dbkit: refused database address") {
		t.Errorf("New() stopped with %q, want the dbkit error for the address kept", message)
	}
}

func TestMigratorCreatesAuthAndCoreSchemas(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}

	db := testdb.Open(t, testdb.New(t, testdb.Migrator()))

	for _, schema := range []string{"auth", "core"} {
		var found bool
		err := db.QueryRow(
			"SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)", schema,
		).Scan(&found)
		if err != nil {
			t.Fatalf("querying schemata for %q: %v", schema, err)
		}
		if !found {
			t.Errorf("schema %q not found after migrating", schema)
		}
	}

	var usersTable *string
	if err := db.QueryRow("SELECT to_regclass('auth.users')::text").Scan(&usersTable); err != nil {
		t.Fatalf("querying auth.users: %v", err)
	}
	if usersTable == nil {
		t.Fatal("auth.users not found: authkit migrations did not run before core ones")
	}
}

func TestMigratorHashIsStableHex(t *testing.T) {
	t.Parallel()

	first, err := testdb.Migrator().Hash()
	if err != nil {
		t.Fatalf("Hash() error = %v, want nil", err)
	}
	second, err := testdb.Migrator().Hash()
	if err != nil {
		t.Fatalf("second Hash() error = %v, want nil", err)
	}

	if first != second {
		t.Errorf("Hash() = %q then %q, want a stable fingerprint", first, second)
	}
	if _, err := hex.DecodeString(first); err != nil || first == "" {
		t.Errorf("Hash() = %q, want non-empty hex", first)
	}
}
