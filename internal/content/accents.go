// SPDX-License-Identifier: Apache-2.0

package content

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// letterFolds maps the lowercase Latin letters that have no canonical decomposition to the plain letters WordPress
// swaps them for when it saves a slug.
var letterFolds = map[rune]string{
	'\u00df': "ss", // ß
	'\u00e6': "ae", // æ
	'\u0153': "oe", // œ
	'\u00f8': "o",  // ø
	'\u0111': "d",  // đ
	'\u00f0': "d",  // ð
	'\u00fe': "th", // þ
	'\u0142': "l",  // ł
	'\u0127': "h",  // ħ
	'\u0131': "i",  // ı
	'\u0167': "t",  // ŧ
	'\u0133': "ij", // ĳ
	'\u0140': "l",  // ŀ
	'\u0138': "k",  // ĸ
	'\u0149': "n",  // ŉ
	'\u014b': "n",  // ŋ
	'\u017f': "s",  // ſ
}

// foldAccents returns s with accented Latin letters replaced by their plain forms, the way WordPress does when it
// saves a slug. It expects s in NFC form. Only marks that follow a Latin letter are dropped, so letters of other
// scripts, such as Cyrillic "й" or CJK characters, are left exactly as they are.
func foldAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r < utf8.RuneSelf:
			b.WriteRune(r)
		case letterFolds[r] != "":
			b.WriteString(letterFolds[r])
		default:
			b.WriteRune(plainLatin(r))
		}
	}
	return b.String()
}

// plainLatin returns the Latin letter r is built from when r is a Latin letter carrying only combining marks, and r
// itself otherwise.
func plainLatin(r rune) rune {
	if !unicode.Is(unicode.Latin, r) {
		return r
	}
	decomposed := []rune(norm.NFD.String(string(r)))
	if len(decomposed) < 2 || !unicode.Is(unicode.Latin, decomposed[0]) {
		return r
	}
	for _, mark := range decomposed[1:] {
		if !unicode.Is(unicode.Mn, mark) {
			return r
		}
	}
	return decomposed[0]
}
