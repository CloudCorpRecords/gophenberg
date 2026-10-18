// SPDX-License-Identifier: Apache-2.0

package update

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/gopherium/gophenberg/internal/signature"
)

// rootMultikeys are the two root keys that sign the key list of the official update site.
var rootMultikeys = []string{
	"zDnaesSvh9HYr1uiuwM9WMLyRiwQ9mu1jogeggWXieTyN9ypm",
	"zDnaeUB7EZwbeTsHgsphuWYx1ix2c3GHmAE68KX21NaLEMRca",
}

// Roots returns the root keys built into this binary.
func Roots() []*ecdsa.PublicKey {
	roots, _ := ParseRoots(strings.Join(rootMultikeys, ","))
	return roots
}

// ParseRoots returns the P-256 root keys a comma-separated list of Multikeys names, none for a blank text.
func ParseRoots(text string) ([]*ecdsa.PublicKey, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	var roots []*ecdsa.PublicKey
	for item := range strings.SplitSeq(text, ",") {
		name := strings.TrimSpace(item)
		key, err := signature.ParseMultikey(name)
		if err != nil {
			return nil, fmt.Errorf("update: refused the root %q: %w", name, err)
		}
		root, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("update: refused the root %q, which is not a P-256 key", name)
		}
		roots = append(roots, root)
	}
	return roots, nil
}
