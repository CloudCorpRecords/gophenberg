// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/update"
)

// builtInRoots are the Multikeys of the two root keys the maintainer made for the official update site.
var builtInRoots = []string{
	"zDnaesSvh9HYr1uiuwM9WMLyRiwQ9mu1jogeggWXieTyN9ypm",
	"zDnaeUB7EZwbeTsHgsphuWYx1ix2c3GHmAE68KX21NaLEMRca",
}

func TestRootsAreTheTwoKeysOfTheOfficialSite(t *testing.T) {
	t.Parallel()

	roots := update.Roots()

	names := make([]string, len(roots))
	for i, root := range roots {
		names[i] = multikey(t, root)
	}
	if !slices.Equal(names, builtInRoots) {
		t.Errorf("Roots() = %v, want %v", names, builtInRoots)
	}
}

func TestParseRootsReadsAListOfP256Multikeys(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	text := " " + multikey(t, &k.root1.PublicKey) + " , " + multikey(t, &k.root2.PublicKey) + " "

	roots, err := update.ParseRoots(text)

	if err != nil || len(roots) != 2 || !roots[0].Equal(&k.root1.PublicKey) || !roots[1].Equal(&k.root2.PublicKey) {
		t.Errorf("ParseRoots(%q) = %v, %v, want the two roots in order", text, roots, err)
	}
	if empty, err := update.ParseRoots("  "); err != nil || empty != nil {
		t.Errorf("ParseRoots(blank) = %v, %v, want no root and nil", empty, err)
	}
}

func TestParseRootsRefusesAnythingButAP256Multikey(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	for name, text := range map[string]string{
		"an Ed25519 key": multikey(t, k.release.Public()),
		"a broken key":   "zDnaes",
		"an empty item":  multikey(t, &k.root1.PublicKey) + ",,",
	} {
		if _, err := update.ParseRoots(text); err == nil {
			t.Errorf("ParseRoots(%s) = nil, want it refused", name)
		}
	}
}
