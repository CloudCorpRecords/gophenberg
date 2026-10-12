// SPDX-License-Identifier: Apache-2.0

package testdb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
)

const (
	// fileAddress is the address the .env file holds in these tests.
	fileAddress = "postgres://postgres@file.example.com:5432/postgres?sslmode=disable"
	// environAddress is the address the environment holds in these tests.
	environAddress = "postgres://postgres@environ.example.com:5432/postgres?sslmode=disable"
	// movedAddress is the address the .env file of a later working directory holds in these tests.
	movedAddress = "postgres://postgres@moved.example.com:5432/postgres?sslmode=disable"
	// where is the hint every refused setting gives about where to set it.
	where = "in the environment or .env, see .env.example"
)

// errUnreadable is the error of a .env file that cannot be read in these tests.
var errUnreadable = errors.New("the .env file cannot be read")

// unreadable is a reader of .env entries whose file cannot be read.
func unreadable() (map[string]string, error) {
	return nil, errUnreadable
}

// lookup returns an environment that holds values and nothing else.
func lookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

// fileOf returns a reader of .env entries that answers values.
func fileOf(values map[string]string) func() (map[string]string, error) {
	return func() (map[string]string, error) {
		return values, nil
	}
}

// writeFile writes content to the file at path, making its folders first.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("making the folders of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// module returns a new module folder whose .env file holds address.
func module(t *testing.T, address string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/site\n")
	writeFile(t, filepath.Join(root, ".env"), addressKey+"="+address+"\n")
	return root
}

func TestSettingsPreferTheEnvironmentToTheFile(t *testing.T) {
	t.Parallel()

	environ := lookup(map[string]string{addressKey: environAddress, maxAgeKey: "2h"})
	file := map[string]string{addressKey: fileAddress, maxAgeKey: "1h"}

	address, maxAge, err := settings(environ, fileOf(file))
	if err != nil {
		t.Fatalf("settings() error = %v, want nil", err)
	}
	if address != environAddress || maxAge != 2*time.Hour {
		t.Errorf("settings() = %q, %v, want the environment values %q, 2h", address, maxAge, environAddress)
	}
}

func TestSettingsFallBackToTheFile(t *testing.T) {
	t.Parallel()

	file := map[string]string{addressKey: " " + fileAddress + " ", maxAgeKey: " 90m "}

	address, maxAge, err := settings(lookup(nil), fileOf(file))
	if err != nil {
		t.Fatalf("settings() error = %v, want nil", err)
	}
	if address != fileAddress || maxAge != 90*time.Minute {
		t.Errorf("settings() = %q, %v, want the trimmed file values %q, 1h30m", address, maxAge, fileAddress)
	}
}

func TestSettingsKeepTheEnvironmentWhenTheFileCannotBeRead(t *testing.T) {
	t.Parallel()

	environ := lookup(map[string]string{addressKey: environAddress, maxAgeKey: "2h"})

	address, maxAge, err := settings(environ, unreadable)
	if err != nil {
		t.Fatalf("settings() error = %v, want nil when the environment sets both", err)
	}
	if address != environAddress || maxAge != 2*time.Hour {
		t.Errorf("settings() = %q, %v, want the environment values %q, 2h", address, maxAge, environAddress)
	}
}

func TestSettingsReportAnUnreadableFileForAKeyTheEnvironmentLacks(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]string{
		"the address": {maxAgeKey: "1h"},
		"the max age": {addressKey: environAddress},
	}
	for name, environ := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := settings(lookup(environ), unreadable); !errors.Is(err, errUnreadable) {
				t.Errorf("settings() error = %v, want one wrapping %v", err, errUnreadable)
			}
		})
	}
}

func TestSettingsNameAMissingAddressAndWhereToSetIt(t *testing.T) {
	t.Parallel()

	filled := map[string]string{addressKey: fileAddress, maxAgeKey: "1h"}
	cases := map[string]struct {
		environ map[string]string
		file    map[string]string
	}{
		"set nowhere":                           {file: map[string]string{maxAgeKey: "1h"}},
		"blank in the file":                     {file: map[string]string{addressKey: "  ", maxAgeKey: "1h"}},
		"empty in the environment over a value": {environ: map[string]string{addressKey: ""}, file: filled},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, _, err := settings(lookup(c.environ), fileOf(c.file))
			if err == nil || !strings.Contains(err.Error(), addressKey) || !strings.Contains(err.Error(), where) {
				t.Errorf("settings() error = %v, want one naming %s and %q", err, addressKey, where)
			}
		})
	}
}

func TestSettingsNameARefusedAddressAndWhereToSetIt(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"another scheme":      "mysql://postgres@db.example.com/postgres",
		"a pool setting":      "postgres://postgres@db.example.com:5432/postgres?pool_max_conns=4",
		"a user in the query": "postgres://postgres@db.example.com:5432/postgres?user=other",
	}
	for name, address := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			file := map[string]string{addressKey: address, maxAgeKey: "1h"}
			_, _, err := settings(lookup(nil), fileOf(file))
			if !errors.Is(err, dbkit.ErrAddress) {
				t.Fatalf("settings() error = %v, want the pgtest refusal of the address", err)
			}
			if want := addressKey + " to an address pgtest takes, " + where; !strings.Contains(err.Error(), want) {
				t.Errorf("settings() error = %v, want one holding %q", err, want)
			}
		})
	}
}

func TestSettingsNameABadMaxAgeAndWhereToSetIt(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing":               "",
		"a word":                "soon",
		"a number with no unit": "60",
		"zero":                  "0s",
		"below zero":            "-1h",
	}
	for name, maxAge := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			file := map[string]string{addressKey: fileAddress, maxAgeKey: maxAge}
			_, _, err := settings(lookup(nil), fileOf(file))
			if err == nil || !strings.Contains(err.Error(), maxAgeKey) || !strings.Contains(err.Error(), where) {
				t.Errorf("settings() error = %v, want one naming %s and %q", err, maxAgeKey, where)
			}
		})
	}
}

func TestDotenvReadsTheFileBesideTheNearestGoMod(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/site\n")
	writeFile(t, filepath.Join(root, ".env"), addressKey+"="+fileAddress+"\n"+maxAgeKey+"=1h # an hour\n")
	start := filepath.Join(root, "internal", "store")
	if err := os.MkdirAll(start, 0o750); err != nil {
		t.Fatalf("making %s: %v", start, err)
	}

	values, err := dotenv(start)
	if err != nil {
		t.Fatalf("dotenv() error = %v, want nil", err)
	}
	if values[addressKey] != fileAddress || values[maxAgeKey] != "1h" {
		t.Errorf("dotenv() = %v, want the address and 1h from the .env file", values)
	}
}

func TestDotenvReadsExportedIndentedAndReferencedLines(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/site\n")
	writeFile(t, filepath.Join(root, ".env"), "GOPHENBERG_DATABASE_URL="+fileAddress+"\n"+
		"  export "+addressKey+"=${GOPHENBERG_DATABASE_URL}\n"+
		"export "+maxAgeKey+"=1h\n")

	values, err := dotenv(root)
	if err != nil {
		t.Fatalf("dotenv() error = %v, want nil", err)
	}
	if values[addressKey] != fileAddress || values[maxAgeKey] != "1h" {
		t.Errorf("dotenv() = %v, want the referenced address and 1h from the exported lines", values)
	}
}

func TestDotenvStopsAtTheNearestGoMod(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".env"), addressKey+"="+fileAddress+"\n")
	writeFile(t, filepath.Join(root, "nested", "go.mod"), "module example.com/nested\n")

	values, err := dotenv(filepath.Join(root, "nested"))
	if err != nil {
		t.Fatalf("dotenv() error = %v, want nil", err)
	}
	if len(values) != 0 {
		t.Errorf("dotenv() = %v, want nothing from a .env above the module root", values)
	}
}

func TestDotenvIsEmptyOutsideAModule(t *testing.T) {
	t.Parallel()

	values, err := dotenv(t.TempDir())
	if err != nil {
		t.Fatalf("dotenv() error = %v, want nil", err)
	}
	if len(values) != 0 {
		t.Errorf("dotenv() = %v, want nothing with no go.mod above", values)
	}
}

func TestDotenvReportsAFileItCannotRead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/site\n")
	if err := os.Mkdir(filepath.Join(root, ".env"), 0o750); err != nil {
		t.Fatalf("making a folder named .env: %v", err)
	}

	if _, err := dotenv(root); err == nil || !strings.Contains(err.Error(), filepath.Join(root, ".env")) {
		t.Errorf("dotenv() error = %v, want one naming the .env file", err)
	}
}

func TestDotenvOnceReadsTheFolderItWasGiven(t *testing.T) {
	read := dotenvOnce(module(t, fileAddress), nil)
	t.Chdir(module(t, movedAddress))

	values, err := read()
	if err != nil {
		t.Fatalf("read() error = %v, want nil", err)
	}
	if values[addressKey] != fileAddress {
		t.Errorf("read() address = %q, want %q from the folder it was given", values[addressKey], fileAddress)
	}
}

func TestDotenvOnceReadsTheFileOnce(t *testing.T) {
	t.Parallel()

	root := module(t, fileAddress)
	read := dotenvOnce(root, nil)
	if _, err := read(); err != nil {
		t.Fatalf("first read() error = %v, want nil", err)
	}
	writeFile(t, filepath.Join(root, ".env"), addressKey+"="+movedAddress+"\n")

	values, err := read()
	if err != nil {
		t.Fatalf("second read() error = %v, want nil", err)
	}
	if values[addressKey] != fileAddress {
		t.Errorf("second read() address = %q, want %q from the first read", values[addressKey], fileAddress)
	}
}

func TestDotenvOnceReportsAWorkingDirectoryItCouldNotFind(t *testing.T) {
	t.Parallel()

	lost := errors.New("the working directory is gone")

	if _, err := dotenvOnce("", lost)(); !errors.Is(err, lost) {
		t.Errorf("read() error = %v, want one wrapping %v", err, lost)
	}
}
