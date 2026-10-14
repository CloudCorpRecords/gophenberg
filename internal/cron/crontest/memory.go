// SPDX-License-Identifier: Apache-2.0

package crontest

import (
	"context"
	"sync"
	"time"

	"github.com/gopherium/gophenberg/internal/cron"
)

var _ cron.Store = (*MemoryStore)(nil)

// memoryRow is one stored job in a MemoryStore.
type memoryRow struct {
	nextRun     time.Time
	leasedUntil time.Time
	token       string
	lastError   string
}

// MemoryStore keeps cron jobs and their leases in memory.
type MemoryStore struct {
	mu   sync.Mutex
	rows map[string]memoryRow
}

// NewMemoryStore returns an empty in-memory cron job store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{rows: make(map[string]memoryRow)}
}

// Slots returns where every stored job stands.
func (m *MemoryStore) Slots(context.Context) ([]cron.Slot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	slots := make([]cron.Slot, 0, len(m.rows))
	for name, row := range m.rows {
		slots = append(slots, cron.Slot{Name: name, NextRun: row.nextRun, LeasedUntil: row.leasedUntil})
	}
	return slots, nil
}

// Take claims the job for the lease when it is due and free, reporting whether it did.
func (m *MemoryStore) Take(_ context.Context, lease cron.Lease) (cron.Lease, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, stored := m.rows[lease.Name]
	if !stored {
		row.nextRun = lease.Taken
	}
	if row.nextRun.After(lease.Taken) || row.leasedUntil.After(lease.Taken) {
		return cron.Lease{}, false, nil
	}
	row.leasedUntil, row.token = lease.Until, lease.Token
	m.rows[lease.Name] = row
	lease.NextRun = row.nextRun
	return lease, true, nil
}

// Settle frees the job and records how its run ended, refusing a lease another run took over.
func (m *MemoryStore) Settle(_ context.Context, lease cron.Lease, outcome cron.Outcome) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, stored := m.rows[lease.Name]
	if !stored || row.token != lease.Token {
		return cron.ErrLeaseLost
	}
	m.rows[lease.Name] = memoryRow{nextRun: outcome.NextRun, lastError: outcome.Failure}
	return nil
}

// LastFailure returns the failure the job's last run recorded, empty when it ran well.
func (m *MemoryStore) LastFailure(_ context.Context, name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rows[name].lastError, nil
}
