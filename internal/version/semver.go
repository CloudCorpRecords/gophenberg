// SPDX-License-Identifier: Apache-2.0

package version

import (
	"cmp"
	"slices"
	"strings"
)

// wordRunes are the characters a pre-release identifier may hold.
const wordRunes = "-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// identifier is one dot-separated part of a semantic version: one of its three numbers or a pre-release word.
type identifier string

// semantic is a full semantic version split into its three numbers and its pre-release identifiers.
type semantic struct {
	numbers    []identifier
	prerelease []identifier
}

// Valid reports whether v is a full semantic version written without a leading v or build metadata.
func Valid(v string) bool {
	_, ok := parse(v)
	return ok
}

// Before reports whether version a comes before version b in semantic version order, an invalid one first.
func Before(a, b string) bool {
	return Compare(a, b) < 0
}

// Compare returns -1, 0 or 1 as version a comes before, with or after version b, every invalid one first and equal.
func Compare(a, b string) int {
	pa, okA := parse(a)
	pb, okB := parse(b)
	if !okA || !okB {
		return cmp.Compare(boolInt(okA), boolInt(okB))
	}
	return pa.compare(pb)
}

// parse returns the parts of a full semantic version, reporting whether v is one.
func parse(v string) (semantic, bool) {
	core, prerelease, marked := strings.Cut(v, "-")
	parsed := semantic{numbers: identifiers(core)}
	if marked {
		parsed.prerelease = identifiers(prerelease)
	}
	ok := len(parsed.numbers) == 3 && !slices.ContainsFunc(parsed.numbers, identifier.notNumber)
	return parsed, ok && !slices.ContainsFunc(parsed.prerelease, identifier.notWord)
}

// identifiers returns the dot-separated identifiers of text.
func identifiers(text string) []identifier {
	parts := strings.Split(text, ".")
	held := make([]identifier, len(parts))
	for i, part := range parts {
		held[i] = identifier(part)
	}
	return held
}

// compare orders two full semantic versions by their numbers, then by their pre-release identifiers.
func (s semantic) compare(other semantic) int {
	if order := slices.CompareFunc(s.numbers, other.numbers, identifier.compareNumber); order != 0 {
		return order
	}
	if len(s.prerelease) == 0 || len(other.prerelease) == 0 {
		return cmp.Compare(len(other.prerelease), len(s.prerelease))
	}
	return slices.CompareFunc(s.prerelease, other.prerelease, identifier.compareWord)
}

// notNumber reports whether the identifier is not a number written without a leading zero.
func (i identifier) notNumber() bool {
	return !i.digits() || len(i) > 1 && i[0] == '0'
}

// notWord reports whether the identifier is not a pre-release identifier of letters, digits and hyphens.
func (i identifier) notWord() bool {
	if i.digits() {
		return i.notNumber()
	}
	return i == "" || strings.ContainsFunc(string(i), func(r rune) bool { return !strings.ContainsRune(wordRunes, r) })
}

// digits reports whether the identifier is one or more ASCII digits.
func (i identifier) digits() bool {
	return i != "" && strings.Trim(string(i), "0123456789") == ""
}

// compareNumber orders two numbers written without a leading zero, however many digits they hold.
func (i identifier) compareNumber(other identifier) int {
	return cmp.Or(cmp.Compare(len(i), len(other)), strings.Compare(string(i), string(other)))
}

// compareWord orders two pre-release identifiers: numbers by value, before any word, words as text.
func (i identifier) compareWord(other identifier) int {
	numberA, numberB := i.digits(), other.digits()
	switch {
	case numberA && numberB:
		return i.compareNumber(other)
	case numberA:
		return -1
	case numberB:
		return 1
	}
	return strings.Compare(string(i), string(other))
}

// boolInt returns 1 for true and 0 for false.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
