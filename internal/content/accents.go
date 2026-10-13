// SPDX-License-Identifier: Apache-2.0

package content

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// letterFolds maps the lowercase Latin letters that have no canonical decomposition to the plain letters WordPress
// swaps them for when it saves a slug.
var letterFolds = map[rune]string{
	'ß': "ss",
	'æ': "ae",
	'œ': "oe",
	'ø': "o",
	'đ': "d",
	'ð': "d",
	'þ': "th",
	'ł': "l",
	'ħ': "h",
	'ı': "i",
	'ŧ': "t",
	'ĳ': "ij",
	'ŀ': "l",
	'ĸ': "k",
	'ŉ': "n",
	'ŋ': "n",
	'ſ': "s",
}

// foldAccents returns s with accented Latin letters replaced by their plain forms, the way WordPress does when it
// saves a slug. It expects s in NFC form, lowercased. Combining marks that follow a Latin letter are dropped, including
// ones NFC could not compose, so "x" followed by a separate acute accent folds to "x". Letters of other scripts and
// the marks that follow them, such as Cyrillic "й" or CJK characters, are left exactly as they are.
func foldAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	afterLatin := false
	for _, r := range s {
		if afterLatin && unicode.Is(unicode.Mn, r) {
			continue
		}
		plain := plainLatin(r)
		switch {
		case letterFolds[plain] != "":
			b.WriteString(letterFolds[plain])
		default:
			b.WriteRune(plain)
		}
		afterLatin = isLatinLetter(plain)
	}
	return b.String()
}

// plainLatin returns the Latin letter r is built from when r is a Latin letter carrying only combining marks, and r
// itself otherwise.
func plainLatin(r rune) rune {
	if r < utf8.RuneSelf || !unicode.Is(unicode.Latin, r) {
		return r
	}
	decomposed := []rune(norm.NFD.String(string(r)))
	if len(decomposed) < 2 || !unicode.Is(unicode.Latin, decomposed[0]) ||
		slices.ContainsFunc(decomposed[1:], func(mark rune) bool { return !unicode.Is(unicode.Mn, mark) }) {
		return r
	}
	return decomposed[0]
}

// isLatinLetter reports whether r is a letter of the Latin script.
func isLatinLetter(r rune) bool {
	if r < utf8.RuneSelf {
		return unicode.IsLetter(r)
	}
	return unicode.Is(unicode.Latin, r) && unicode.IsLetter(r)
}
