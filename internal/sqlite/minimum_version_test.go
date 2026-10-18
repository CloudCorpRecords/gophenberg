// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"testing"

	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

// raiseMinimumVersion raises the stored minimum version on db to minimum, failing the test on an error.
func raiseMinimumVersion(t *testing.T, db *sql.DB, minimum string) {
	t.Helper()
	if err := sqlite.RaiseMinimumVersion(t.Context(), db, minimum); err != nil {
		t.Fatalf("RaiseMinimumVersion(%q) error = %v, want nil", minimum, err)
	}
}

func TestStoredMinimumVersionIsEmptyBeforeTheCoreTablesExist(t *testing.T) {
	t.Parallel()

	minimum, err := sqlite.StoredMinimumVersion(t.Context(), sqlitetest.Open(t, testSettings().Options()))

	if err != nil || minimum != "" {
		t.Errorf("StoredMinimumVersion() = %q, %v on a file without core tables, want an empty version and nil",
			minimum, err)
	}
}

func TestStoredMinimumVersionIsEmptyUntilABinaryRaisesIt(t *testing.T) {
	t.Parallel()

	minimum, err := sqlite.StoredMinimumVersion(t.Context(), migrated(t))

	if err != nil || minimum != "" {
		t.Errorf("StoredMinimumVersion() = %q, %v on a file no binary raised, want an empty version and nil",
			minimum, err)
	}
}

func TestRaiseMinimumVersionNeverLowersTheStoredOne(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		stored []string
		raise  string
		want   string
	}{
		{name: "a first minimum is stored", raise: "0.19.0", want: "0.19.0"},
		{name: "an older minimum is raised", stored: []string{"0.19.0"}, raise: "0.20.0", want: "0.20.0"},
		{name: "a newer minimum stays", stored: []string{"0.21.0"}, raise: "0.20.0", want: "0.21.0"},
		{name: "the same minimum stays", stored: []string{"0.20.0"}, raise: "0.20.0", want: "0.20.0"},
		{name: "numbers order the versions, not text", stored: []string{"0.9.0"}, raise: "0.10.0", want: "0.10.0"},
		{name: "a release raises its candidate's minimum", stored: []string{"0.20.0-rc.1"}, raise: "0.20.0",
			want: "0.20.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := migrated(t)
			for _, minimum := range tc.stored {
				raiseMinimumVersion(t, db, minimum)
			}

			raiseMinimumVersion(t, db, tc.raise)

			if got, err := sqlite.StoredMinimumVersion(t.Context(), db); err != nil || got != tc.want {
				t.Errorf("StoredMinimumVersion() = %q, %v after raising to %q, want %q", got, err, tc.raise, tc.want)
			}
		})
	}
}

func TestRaiseFromLeavesAMinimumVersionAnotherBinaryStoredSince(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	raiseMinimumVersion(t, db, "0.20.0")

	raised, err := sqlite.RaiseFrom(t.Context(), db, "0.19.0", "0.21.0")

	if err != nil || raised {
		t.Errorf("RaiseFrom() over a stale 0.19.0 = %v, %v, want nothing raised", raised, err)
	}
	if got, _ := sqlite.StoredMinimumVersion(t.Context(), db); got != "0.20.0" {
		t.Errorf("StoredMinimumVersion() = %q, want the 0.20.0 stored since kept", got)
	}
	if raised, err := sqlite.RaiseFrom(t.Context(), db, "0.20.0", "0.21.0"); err != nil || !raised {
		t.Errorf("RaiseFrom() over the stored 0.20.0 = %v, %v, want 0.21.0 raised", raised, err)
	}
}

func TestRaiseMinimumVersionFailsBeforeTheCoreTablesExist(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())
	if err := sqlite.RaiseMinimumVersion(t.Context(), db, "0.19.0"); err == nil {
		t.Error("RaiseMinimumVersion() on a file without core tables error = nil, want a failure")
	}
}

func TestMinimumVersionReportsDatabaseFailures(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	_ = db.Close()

	if _, err := sqlite.StoredMinimumVersion(t.Context(), db); err == nil {
		t.Error("StoredMinimumVersion() on a closed handle error = nil, want a failure")
	}
	if err := sqlite.RaiseMinimumVersion(t.Context(), db, "0.19.0"); err == nil {
		t.Error("RaiseMinimumVersion() on a closed handle error = nil, want a failure")
	}
}
