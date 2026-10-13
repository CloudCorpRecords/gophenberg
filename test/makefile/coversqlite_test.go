// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sqliteRepo returns a module repo that also holds a SQLite package whose test passes or fails, and its generated code.
func sqliteRepo(t *testing.T, sqlitePasses bool) string {
	t.Helper()
	root := moduleRepo(t, true, false)
	verdict := `t.Error("the sqlite package ran and failed")`
	if sqlitePasses {
		verdict = `t.Log("the sqlite package ran")`
	}
	sqliteTest := "package sqlite\n\nimport \"testing\"\n\nfunc TestOpen(t *testing.T) {\n\tOpen()\n\t" + verdict + "\n}\n"
	files := map[string]string{
		"internal/sqlite/sqlite.go":      "package sqlite\n\nfunc Open() bool { return true }\n",
		"internal/sqlite/sqlite_test.go": sqliteTest,
		"internal/sqlite/db/db.go":       "package db\n\nfunc Query() bool { return true }\n",
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatalf("making the folder of %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	return root
}

// profileOf runs make with the target in the directory and returns what it printed and the profile it wrote.
func profileOf(t *testing.T, root, target string, profile ...string) (string, string, error) {
	t.Helper()
	out, err := makeIn(t, root, false, target)
	written, readErr := os.ReadFile(filepath.Join(append([]string{root}, profile...)...))
	if err == nil {
		err = readErr
	}
	return out, string(written), err
}

func TestMakeCoverSQLiteMeasuresOnlyTheSQLitePackages(t *testing.T) {
	t.Parallel()

	out, profile, err := profileOf(t, sqliteRepo(t, true), "cover-sqlite", ".covdata", "sqlite", "cover.out")

	if err != nil || !strings.Contains(profile, "example.com/site/internal/sqlite/sqlite.go") {
		t.Fatalf("make cover-sqlite = %v with output %q, want the SQLite package in its own profile", err, out)
	}
	for _, left := range []string{
		"example.com/site/internal/sqlite/db", "example.com/site/internal/app", "example.com/site/sdk",
	} {
		if strings.Contains(profile, left) || strings.Contains(out, left) {
			t.Errorf("make cover-sqlite measured %s, want only the SQLite packages outside the generated code", left)
		}
	}
}

func TestMakeCoverSQLiteFailsWhenASQLiteTestFails(t *testing.T) {
	t.Parallel()

	out, err := makeIn(t, sqliteRepo(t, false), false, "cover-sqlite")

	if err == nil || !strings.Contains(out, "the sqlite package ran and failed") {
		t.Errorf("make cover-sqlite = %v with output %q, want it to fail on the SQLite package's failing test", err, out)
	}
}

func TestMakeCoverLeavesTheSQLitePackagesToTheirOwnJob(t *testing.T) {
	t.Parallel()

	out, profile, err := profileOf(t, sqliteRepo(t, true), "cover", ".covdata", "cover.out")

	if err != nil || !strings.Contains(profile, "example.com/site/sdk/sdk.go") {
		t.Fatalf("make cover = %v with output %q, want the merged profile written", err, out)
	}
	if strings.Contains(profile, "example.com/site/internal/sqlite/") {
		t.Errorf("make cover measured the SQLite packages, want them left to make cover-sqlite")
	}
}
