// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// readerWithNoAccount is the id of a reader that no account row holds.
const readerWithNoAccount = "019a0000-0000-7000-8000-0000000000a1"

func TestAReaderSettingKeepsAReaderWithNoAccount(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	if checked := count(t, handle, "PRAGMA foreign_keys"); checked != 1 {
		t.Fatalf("PRAGMA foreign_keys = %d, want the handle to check every foreign key", checked)
	}
	reader := uuid.MustParse(readerWithNoAccount)
	store := sqlite.NewUserSettingStore(handle)
	if held, found, err := store.Lookup(t.Context(), reader, "locale"); err != nil || found {
		t.Fatalf("Lookup() before any save = %q, %v, error %v, want the key unset", held, found, err)
	}

	err := store.Save(t.Context(), reader, "locale", "es-ES")

	if err != nil {
		t.Fatalf("Save() for a reader with no account error = %v, want nil, as a reader setting names no account", err)
	}
	if held, found, err := store.Lookup(t.Context(), reader, "locale"); err != nil || !found || held != "es-ES" {
		t.Errorf("Lookup() = %q, %v, error %v, want the language the reader chose", held, found, err)
	}
	if accounts := count(t, handle, "SELECT count(*) FROM auth_users"); accounts != 0 {
		t.Errorf("the file holds %d accounts, want none, so the reader has no account", accounts)
	}
}

func TestAReaderSettingReportsAStatementThatFails(t *testing.T) {
	t.Parallel()

	reader := uuid.MustParse(readerWithNoAccount)
	for _, tc := range []struct {
		name, statement, prefix string
		call                    func(ctx context.Context, store *sqlite.UserSettingStore) error
	}{
		{"a read names its key", db.GetUserSetting, `sqlite: reading the setting "locale": `,
			func(ctx context.Context, store *sqlite.UserSettingStore) error {
				_, _, err := store.Lookup(ctx, reader, "locale")
				return err
			}},
		{"a write names its key", db.SetUserSetting, `sqlite: writing the setting "locale": `,
			func(ctx context.Context, store *sqlite.UserSettingStore) error {
				return store.Save(ctx, reader, "locale", "es-ES")
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handle, faults := faultyHandle(t)
			faults.FailStatement(tc.statement, errPlanted)

			err := tc.call(t.Context(), sqlite.NewUserSettingStore(handle))

			wantPlanted(t, err, tc.prefix)
		})
	}
}

func TestAReaderSettingReportsAHandleItCannotReach(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewUserSettingStore(handle)
	if err := handle.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	reader := uuid.MustParse(readerWithNoAccount)

	_, _, lookupErr := store.Lookup(t.Context(), reader, "locale")
	saveErr := store.Save(t.Context(), reader, "locale", "es-ES")

	if !strings.HasPrefix(fmt.Sprint(lookupErr), `sqlite: reading the setting "locale": `) {
		t.Errorf("Lookup() on a closed handle error = %v, want the read named", lookupErr)
	}
	if !strings.HasPrefix(fmt.Sprint(saveErr), `sqlite: writing the setting "locale": `) {
		t.Errorf("Save() on a closed handle error = %v, want the write named", saveErr)
	}
}

func TestAReaderSettingSaveBehindAnotherWriterAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle := impatientHandle(t)
	holdWrites(t, handle)

	err := sqlite.NewUserSettingStore(handle).Save(t.Context(), uuid.MustParse(readerWithNoAccount), "locale", "es-ES")

	if !errors.Is(err, dbkit.ErrBusy) || !strings.HasPrefix(fmt.Sprint(err), `sqlite: writing the setting "locale": `) {
		t.Errorf("Save() behind another writer error = %v, want dbkit.ErrBusy under the write", err)
	}
}
