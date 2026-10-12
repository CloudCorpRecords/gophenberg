// SPDX-License-Identifier: Apache-2.0

// Package mediatest runs one set of behaviour cases against any media.Store.
package mediatest

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
)

// Stores is one fresh, empty media library a case runs against, with a way to add the accounts its items name.
type Stores struct {
	Media     media.Store
	Saved     func(ctx context.Context, files []string) (map[string]bool, error)
	AddAuthor func(t *testing.T, name string) uuid.UUID
}

// Factory returns fresh stores for one case, skipping the case when its engine is out of reach.
type Factory func(t *testing.T) Stores

// Case is one named behaviour every media store shares.
type Case struct {
	Name string
	Run  func(t *testing.T, s Stores)
}

// Cases returns every media store case in a fixed order.
func Cases() []Case {
	return slices.Concat(mediaCases, listCases)
}

// Run runs every media store case in parallel, each on stores of its own.
func Run(t *testing.T, fresh Factory) {
	t.Helper()
	for _, c := range Cases() {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			c.Run(t, fresh(t))
		})
	}
}
