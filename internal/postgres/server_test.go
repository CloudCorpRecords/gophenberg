// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

func TestCheckMajorTakesTheServerTheTestsRunOn(t *testing.T) {
	t.Parallel()

	if err := postgres.CheckMajor(t.Context(), emptyDB(t)); err != nil {
		t.Errorf("CheckMajor() error = %v on the test server, want nil", err)
	}
}

func TestServerMajorReadsTheMajorOfTheTestServer(t *testing.T) {
	t.Parallel()

	major, err := postgres.ServerMajor(t.Context(), emptyDB(t))

	if err != nil || major < postgres.OldestMajor {
		t.Errorf("ServerMajor() = %d, %v on the test server, want a major of %d or newer", major, err, postgres.OldestMajor)
	}
}

func TestSuperuserIsFalseForTheRoleTestDatabasesBelongTo(t *testing.T) {
	t.Parallel()

	super, err := postgres.Superuser(t.Context(), emptyDB(t))

	if err != nil || super {
		t.Errorf("Superuser() = %v, %v for the owner of a test database, want false and nil", super, err)
	}
}

func TestSuperuserIsTrueForTheRoleTheTestServerIsReachedAs(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}

	super, err := postgres.Superuser(t.Context(), testdb.Open(t, testdb.Server(t)))

	if err != nil || !super {
		t.Errorf("Superuser() = %v, %v for the role the test server is reached as, want true and nil", super, err)
	}
}

func TestServerChecksReportDatabaseFailures(t *testing.T) {
	t.Parallel()

	db := emptyDB(t)
	_ = db.Close()

	if err := postgres.CheckMajor(t.Context(), db); err == nil {
		t.Error("CheckMajor() on a closed handle error = nil, want a failure")
	}
	if _, err := postgres.ServerMajor(t.Context(), db); err == nil {
		t.Error("ServerMajor() on a closed handle error = nil, want a failure")
	}
	if _, err := postgres.Superuser(t.Context(), db); err == nil {
		t.Error("Superuser() on a closed handle error = nil, want a failure")
	}
}
