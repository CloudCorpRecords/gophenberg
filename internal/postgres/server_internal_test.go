// SPDX-License-Identifier: Apache-2.0

package postgres

import "testing"

func TestMajorRefusalNamesTheServerAndTheOldestMajor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		number  int
		shown   string
		refused bool
	}{
		{number: 140012, shown: "14.12", refused: true},
		{number: 149999, shown: "14.99", refused: true},
		{number: 150000, shown: "15.0"},
		{number: 180001, shown: "18.1 (Debian 18.1-1.pgdg13+2)"},
	}
	for _, tc := range cases {
		err := majorRefusal(tc.number, tc.shown)
		want := "this database runs PostgreSQL " + tc.shown + " and Gophenberg needs PostgreSQL 15 or newer"
		switch {
		case !tc.refused && err != nil:
			t.Errorf("majorRefusal(%d) = %v, want nil", tc.number, err)
		case tc.refused && (err == nil || err.Error() != want):
			t.Errorf("majorRefusal(%d) = %v, want %q", tc.number, err, want)
		}
	}
}
