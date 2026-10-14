// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// impatientWait is how long an impatient handle waits for another writer before it answers busy.
const impatientWait = 50 * time.Millisecond

// errPlanted is the failure a chosen statement or commit answers with.
var errPlanted = errors.New("planted failure")

// themeKeys are the two keys a theme switch writes together.
var themeKeys = []string{"theme.active", "theme.previous"}

// failLaterSettingWrites counts the writes to core_settings and fails every one after the first.
var failLaterSettingWrites = []string{
	"CREATE TABLE sabotage_writes (n INTEGER NOT NULL)",
	"INSERT INTO sabotage_writes (n) VALUES (0)",
	"CREATE TRIGGER sabotage BEFORE INSERT ON core_settings BEGIN " +
		"UPDATE sabotage_writes SET n = n + 1; " +
		"SELECT RAISE(ABORT, 'sabotaged') WHERE (SELECT n FROM sabotage_writes) > 1; END",
}

// faultyHandle returns a handle on a fresh migrated copy whose connections answer with the faults it returns.
func faultyHandle(t *testing.T) (*sql.DB, *sqlitetest.Faults) {
	t.Helper()
	faults := &sqlitetest.Faults{}
	return template.OpenWithFaults(t, faults), faults
}

// wantPlanted fails t unless err is the planted failure under the text prefix.
func wantPlanted(t *testing.T, err error, prefix string) {
	t.Helper()
	if !errors.Is(err, errPlanted) || !strings.HasPrefix(fmt.Sprint(err), prefix) {
		t.Errorf("error = %v, want the planted failure under %q", err, prefix)
	}
}

// impatientHandle returns a handle on a fresh migrated file that waits only impatientWait for another writer.
func impatientHandle(t *testing.T) *sql.DB {
	t.Helper()
	impatient := testSettings()
	impatient.BusyTimeout = impatientWait
	handle := sqlitetest.Open(t, impatient.Options())
	if err := migrateAll(t.Context(), handle); err != nil {
		t.Fatalf("migrating a fresh file: %v", err)
	}
	return handle
}

// otherWriter returns a connection of handle of its own, closed when t ends.
func otherWriter(t *testing.T, handle *sql.DB) *sql.Conn {
	t.Helper()
	writer, err := handle.Conn(t.Context())
	if err != nil {
		t.Fatalf("Conn() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	return writer
}

// holdWrites begins a write on a connection of handle of its own and holds it until t ends.
func holdWrites(t *testing.T, handle *sql.DB) {
	t.Helper()
	if _, err := otherWriter(t, handle).ExecContext(t.Context(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatalf("BEGIN IMMEDIATE error = %v, want nil", err)
	}
}

// mustSaveSettings stores values through store and fails t when it errs.
func mustSaveSettings(t *testing.T, store *sqlite.SettingStore, values map[string]string) {
	t.Helper()
	if err := store.Save(t.Context(), values); err != nil {
		t.Fatalf("Save(%v) error = %v, want nil", values, err)
	}
}

// heldThemes returns the value of each theme key store holds, leaving out the keys it does not hold.
func heldThemes(t *testing.T, store *sqlite.SettingStore) map[string]string {
	t.Helper()
	held := map[string]string{}
	for _, key := range themeKeys {
		value, found, err := store.Lookup(t.Context(), key)
		if err != nil {
			t.Fatalf("Lookup(%q) error = %v, want nil", key, err)
		}
		if found {
			held[key] = value
		}
	}
	return held
}

func TestSettingSaveStoresNoneOfAWriteWhoseSecondUpsertFails(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewSettingStore(handle)
	mustSaveSettings(t, store, map[string]string{"theme.active": "aurora"})
	for _, statement := range failLaterSettingWrites {
		mustExec(t, handle, statement)
	}

	err := store.Save(t.Context(), map[string]string{"theme.active": "riverbed", "theme.previous": "aurora"})

	text, want := fmt.Sprint(err), `sqlite: writing settings: "theme.previous": `
	if !strings.HasPrefix(text, want) || !strings.Contains(text, "sabotaged") {
		t.Fatalf("Save() error = %v, want the sabotaged write of theme.previous, the later key, named", err)
	}
	if held := heldThemes(t, store); !maps.Equal(held, map[string]string{"theme.active": "aurora"}) {
		t.Errorf("after the failed write the store holds %v, want only the seeded theme.active of aurora", held)
	}
}

func TestSettingSaveNamesTheKeyBeforeTheClassOfAWriteThatFails(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewSettingStore(handle)
	mustSaveSettings(t, store, map[string]string{"theme.active": "aurora"})
	mustExec(t, handle, "CREATE UNIQUE INDEX settings_value_once ON core_settings (value)")

	err := store.Save(t.Context(), map[string]string{"theme.previous": "aurora"})

	want := `sqlite: writing settings: "theme.previous": ` + dbkit.ErrUnique.Error() + ": "
	if !errors.Is(err, dbkit.ErrUnique) || !strings.HasPrefix(fmt.Sprint(err), want) {
		t.Errorf("Save() error = %v, want the key named before the dbkit class, as in %q", err, want)
	}
}

func TestSettingSaveAfterAFailedWriteTakesTheNextWrite(t *testing.T) {
	t.Parallel()

	handle := impatientHandle(t)
	store := sqlite.NewSettingStore(handle)
	mustSaveSettings(t, store, map[string]string{"theme.active": "aurora"})
	for _, statement := range failLaterSettingWrites {
		mustExec(t, handle, statement)
	}
	failing := map[string]string{"theme.active": "riverbed", "theme.previous": "aurora"}
	if err := store.Save(t.Context(), failing); err == nil {
		t.Fatal("Save() error = nil, want the sabotaged second upsert reported")
	}

	err := store.Save(t.Context(), map[string]string{"theme.active": "riverbed"})

	if err != nil {
		t.Errorf("Save() after a failed write error = %v, want nil, as the failed write let go of the file", err)
	}
}

func TestSettingSaveOfNoValuesWritesNothingBehindAnotherWriter(t *testing.T) {
	t.Parallel()

	handle := impatientHandle(t)
	holdWrites(t, handle)
	store := sqlite.NewSettingStore(handle)

	for _, values := range []map[string]string{nil, {}} {
		if err := store.Save(t.Context(), values); err != nil {
			t.Errorf("Save(%#v) behind another writer error = %v, want nil, as there is nothing to write", values, err)
		}
	}
}

func TestSettingSaveStoresNoneOfAWriteWhoseCommitFails(t *testing.T) {
	t.Parallel()

	handle, faults := faultyHandle(t)
	store := sqlite.NewSettingStore(handle)
	mustSaveSettings(t, store, map[string]string{"theme.active": "aurora"})
	faults.FailNextCommit(errPlanted)

	err := store.Save(t.Context(), map[string]string{"theme.active": "riverbed", "theme.previous": "aurora"})

	wantPlanted(t, err, "sqlite: writing settings: ")
	if held := heldThemes(t, store); !maps.Equal(held, map[string]string{"theme.active": "aurora"}) {
		t.Errorf("after the failed commit the store holds %v, want only the seeded theme.active of aurora", held)
	}
}

func TestSettingReportsAStatementThatFails(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, statement, prefix string
		call                    func(ctx context.Context, store *sqlite.SettingStore) error
	}{
		{"a read names its key", db.GetSetting, `sqlite: reading setting "theme.active": `,
			func(ctx context.Context, store *sqlite.SettingStore) error {
				_, _, err := store.Lookup(ctx, "theme.active")
				return err
			}},
		{"a write names the key it could not store", db.SetSetting, `sqlite: writing settings: "theme.active": `,
			func(ctx context.Context, store *sqlite.SettingStore) error {
				return store.Save(ctx, map[string]string{"theme.active": "aurora"})
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handle, faults := faultyHandle(t)
			faults.FailStatement(tc.statement, errPlanted)

			err := tc.call(t.Context(), sqlite.NewSettingStore(handle))

			wantPlanted(t, err, tc.prefix)
		})
	}
}

func TestSettingReportsAHandleItCannotReach(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewSettingStore(handle)
	if err := handle.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}

	_, _, lookupErr := store.Lookup(t.Context(), "theme.active")
	saveErr := store.Save(t.Context(), map[string]string{"theme.active": "aurora"})

	if !strings.HasPrefix(fmt.Sprint(lookupErr), `sqlite: reading setting "theme.active": `) {
		t.Errorf("Lookup() on a closed handle error = %v, want the read named", lookupErr)
	}
	if !strings.HasPrefix(fmt.Sprint(saveErr), "sqlite: writing settings: ") {
		t.Errorf("Save() on a closed handle error = %v, want the write named", saveErr)
	}
}

func TestSettingSaveBehindAnotherWriterAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle := impatientHandle(t)
	holdWrites(t, handle)

	err := sqlite.NewSettingStore(handle).Save(t.Context(), map[string]string{"theme.active": "aurora"})

	if !errors.Is(err, dbkit.ErrBusy) || !strings.HasPrefix(fmt.Sprint(err), "sqlite: writing settings: ") {
		t.Errorf("Save() behind another writer error = %v, want dbkit.ErrBusy under the write", err)
	}
}
