// SPDX-License-Identifier: Apache-2.0

package dbconns_test

import (
	"math"
	"runtime"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/dbconns"
)

// environment returns the Gophenberg settings reader over values, each key unset when values lacks it.
func environment(values map[string]string) gonsole.Env {
	return gonsole.Env{Prefix: "GOPHENBERG_", Getenv: testkit.Getenv(values)}
}

func TestMaxConnsFromTakesEveryCapFromTheLeastToTheGreatest(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]int{"2": 2, "3": 3, "2147483647": math.MaxInt32} {
		got, err := dbconns.MaxConnsFrom(environment(map[string]string{"GOPHENBERG_DATABASE_MAX_CONNS": value}))
		if err != nil || got != want {
			t.Errorf("MaxConnsFrom() with GOPHENBERG_DATABASE_MAX_CONNS=%s = %d, %v, want %d, nil", value, got, err, want)
		}
	}
}

func TestMaxConnsFromFallsBackToTheLargerOfFourAndTheCPUCount(t *testing.T) {
	t.Parallel()

	got, err := dbconns.MaxConnsFrom(environment(nil))

	if want := max(4, runtime.NumCPU()); err != nil || got != want {
		t.Errorf("MaxConnsFrom() with no cap set = %d, %v, want %d, nil", got, err, want)
	}
}

func TestMaxConnsFromNamesTheSettingAndQuotesEveryCapItRefuses(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]string{
		"1":          `GOPHENBERG_DATABASE_MAX_CONNS: must be 2 or more, got "1"`,
		"2147483648": `GOPHENBERG_DATABASE_MAX_CONNS: must stand at or below 2147483647, got "2147483648"`,
		"many":       `GOPHENBERG_DATABASE_MAX_CONNS: must be a whole number, got "many"`,
	} {
		got, err := dbconns.MaxConnsFrom(environment(map[string]string{"GOPHENBERG_DATABASE_MAX_CONNS": value}))
		if err == nil || err.Error() != want || got != 0 {
			t.Errorf("MaxConnsFrom() with GOPHENBERG_DATABASE_MAX_CONNS=%s = %d, %v, want 0, %s", value, got, err, want)
		}
	}
}
