// SPDX-License-Identifier: Apache-2.0

package version_test

import (
	"regexp"
	"testing"

	"github.com/gopherium/gophenberg/internal/version"
)

func TestVersionIsCleanSemver(t *testing.T) {
	t.Parallel()

	got := version.Version()

	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(got) {
		t.Errorf("Version() = %q, want a trimmed semver like 0.0.0", got)
	}
}

func TestMinimumIsAFullVersionNoNewerThanTheBinary(t *testing.T) {
	t.Parallel()

	minimum := version.Minimum()

	if !version.Valid(minimum) {
		t.Errorf("Minimum() = %q, want a trimmed full version like 0.19.0", minimum)
	}
	if version.Before(version.Version(), minimum) {
		t.Errorf("Minimum() = %q comes after Version() = %q, want a minimum no newer than the binary",
			minimum, version.Version())
	}
}

func TestBeforeOrdersVersionsByTheirNumbers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want bool
	}{
		{a: "0.19.0", b: "0.20.0", want: true},
		{a: "0.20.0", b: "0.19.0", want: false},
		{a: "0.20.0", b: "0.20.0", want: false},
		{a: "0.9.0", b: "0.10.0", want: true},
		{a: "0.20.0-rc.1", b: "0.20.0", want: true},
		{a: "0.20.0-rc.2", b: "0.20.0-rc.1", want: false},
		{a: "0.20.1", b: "1.0.0", want: true},
	}
	for _, tc := range cases {
		if got := version.Before(tc.a, tc.b); got != tc.want {
			t.Errorf("Before(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestBeforeFollowsThePrecedenceOfTheSemanticVersioningSpecification(t *testing.T) {
	t.Parallel()

	ordered := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11",
		"1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "1.10.0", "2.0.0", "18446744073709551616.0.0",
	}
	for i, earlier := range ordered {
		for _, later := range ordered[i+1:] {
			if !version.Before(earlier, later) || version.Before(later, earlier) {
				t.Errorf("Before(%q, %q) and Before(%q, %q) = %v, %v, want true then false", earlier, later, later,
					earlier, version.Before(earlier, later), version.Before(later, earlier))
			}
		}
	}
}

func TestCompareOrdersTwoVersions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want int
	}{
		{a: "0.19.0", b: "0.20.0", want: -1},
		{a: "0.20.0", b: "0.20.0", want: 0},
		{a: "0.20.0", b: "0.20.0-rc.1", want: 1},
		{a: "", b: "not a version", want: 0},
	}
	for _, tc := range cases {
		if got := version.Compare(tc.a, tc.b); got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestBeforePutsAnInvalidVersionFirst(t *testing.T) {
	t.Parallel()

	if !version.Before("", "0.0.1") || version.Before("0.0.1", "") || version.Before("", "not a version") {
		t.Error("Before() puts an invalid version after a valid one, or between two invalid ones, want it first")
	}
}

func TestValidTakesOnlyAFullVersionWithoutAPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want bool
	}{
		{in: "0.19.0", want: true},
		{in: "0.20.0-rc.1", want: true},
		{in: "10.20.30", want: true},
		{in: "1.2", want: false},
		{in: "v1.2.3", want: false},
		{in: "1.2.3+build.4", want: false},
		{in: " 0.19.0", want: false},
		{in: "", want: false},
		{in: "01.2.3", want: false},
		{in: "1.2.3-", want: false},
		{in: "1.2.3-01", want: false},
		{in: "1.2.3-a..b", want: false},
		{in: "1.2.3-rc_1", want: false},
		{in: "1.2.3.4", want: false},
		{in: "1.2.x", want: false},
		{in: "1.0.0-x-y.0.z1", want: true},
	}
	for _, tc := range cases {
		if got := version.Valid(tc.in); got != tc.want {
			t.Errorf("Valid(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestGeneratorNamesTheProductAndItsFeatureVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{in: "1.2.3", want: "Gophenberg 1.2"},
		{in: "0.0.0", want: "Gophenberg 0.0"},
		{in: "10.20.30", want: "Gophenberg 10.20"},
	}
	for _, tc := range cases {
		if got := version.Generator(tc.in); got != tc.want {
			t.Errorf("Generator(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStampIsEmptyInACustomBuild(t *testing.T) {
	t.Parallel()

	if got := version.Stamp(); got != "" {
		t.Errorf("Stamp() = %q in a build no release workflow made, want empty", got)
	}
}

func TestCommitIsEmptyWithoutTheReleaseFlag(t *testing.T) {
	t.Parallel()

	if got := version.Commit(); got != "" {
		t.Errorf("Commit() = %q in a build that named no commit, want empty", got)
	}
}

func TestMajorMinorDropsThePatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{in: "1.2.3", want: "1.2"},
		{in: "0.0.0", want: "0.0"},
		{in: "10.20.30", want: "10.20"},
		{in: "1.2", want: "1.2"},
		{in: "1.2.3-beta.4", want: "1.2"},
		{in: "1", want: "1"},
		{in: "", want: ""},
	}
	for _, tc := range cases {
		if got := version.MajorMinor(tc.in); got != tc.want {
			t.Errorf("MajorMinor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
