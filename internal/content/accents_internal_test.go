// SPDX-License-Identifier: Apache-2.0

package content

import (
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func TestLetterFoldsHoldEveryLatinLetterThatDoesNotDecompose(t *testing.T) {
	t.Parallel()

	for r := rune(0x00C0); r <= 0x017F; r++ {
		if !unicode.IsLower(r) || norm.NFD.String(string(r)) != string(r) {
			continue
		}
		if _, ok := letterFolds[r]; !ok {
			t.Errorf("letterFolds has no plain spelling for %q (%U)", r, r)
		}
	}
}
