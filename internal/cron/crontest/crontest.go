// SPDX-License-Identifier: Apache-2.0

// Package crontest holds the cases every cron job store passes and an in-memory store that passes them.
package crontest

import (
	"context"
	"testing"

	"github.com/gopherium/gophenberg/internal/cron"
)

// Stores is one fresh store under test and a way to read the failure its last run recorded.
type Stores struct {
	Store       cron.Store
	LastFailure func(ctx context.Context, name string) (string, error)
}

// Factory returns fresh stores for one case.
type Factory func(t *testing.T) Stores

// Case is one named check every cron job store passes.
type Case struct {
	Name string
	Run  func(t *testing.T, s Stores)
}

// Cases returns every check a cron job store passes.
func Cases() []Case {
	return storeCases
}

// Run runs every case against fresh stores from fresh, each in parallel.
func Run(t *testing.T, fresh Factory) {
	t.Helper()
	for _, c := range Cases() {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			c.Run(t, fresh(t))
		})
	}
}
