// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"fmt"
	"testing"
)

func TestMigrateRequiresDatabase(t *testing.T) {
	t.Parallel()

	err := Migrate(t.Context(), nil)

	if want := "postgres: apply migrations: dbkit: Migrate needs a database handle, got nil"; fmt.Sprint(err) != want {
		t.Fatalf("Migrate(nil) error = %v, want %q", err, want)
	}
}

func TestMustSubRejectsInvalidDir(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("mustSub(..) did not panic, want a panic")
		}
	}()

	mustSub(Migrations, "..")
}
