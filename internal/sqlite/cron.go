// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// CronStore meets the cron job store contract.
var _ cron.Store = (*CronStore)(nil)

// CronStore keeps the site's cron jobs and their leases in the core tables of its SQLite file.
type CronStore struct {
	queries *db.Queries
}

// NewCronStore returns a [CronStore] on the site's one SQLite handle.
func NewCronStore(handle *sql.DB) *CronStore {
	return &CronStore{queries: db.New(handle)}
}

// Slots returns where every stored job stands.
func (s *CronStore) Slots(ctx context.Context) ([]cron.Slot, error) {
	rows, err := s.queries.ListCronSlots(ctx)
	if err != nil {
		return nil, failed("reading the cron jobs", err)
	}
	slots := make([]cron.Slot, 0, len(rows))
	for _, row := range rows {
		slots = append(slots, cron.Slot{
			Name: row.Name, NextRun: row.NextRunAt.Time, LeasedUntil: cmp.Or(row.LeasedUntil, &dbkit.Time{}).UTC(),
		})
	}
	return slots, nil
}

// Take claims the job for the lease when it is due and free, reporting whether it did.
func (s *CronStore) Take(ctx context.Context, lease cron.Lease) (cron.Lease, bool, error) {
	nextRun, err := s.queries.TakeCronJob(ctx, db.TakeCronJobParams{
		Name: lease.Name, TakenAt: dbkit.Time{Time: lease.Taken}, LeasedUntil: &dbkit.Time{Time: lease.Until},
		LeaseToken: &lease.Token,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return cron.Lease{}, false, nil
	}
	if err != nil {
		return cron.Lease{}, false, failed(fmt.Sprintf("taking the cron job %q", lease.Name), err)
	}
	lease.NextRun = nextRun.Time
	return lease, true, nil
}

// Settle frees the job and records how its run ended, refusing a lease another run took over.
func (s *CronStore) Settle(ctx context.Context, lease cron.Lease, outcome cron.Outcome) error {
	settled, err := s.queries.SettleCronJob(ctx, db.SettleCronJobParams{
		NextRunAt: dbkit.Time{Time: outcome.NextRun}, FinishedAt: &dbkit.Time{Time: outcome.Finished},
		LastError: failureOf(outcome), Name: lease.Name, LeaseToken: &lease.Token,
	})
	if err != nil {
		return failed(fmt.Sprintf("settling the cron job %q", lease.Name), err)
	}
	if settled == 0 {
		return cron.ErrLeaseLost
	}
	return nil
}

// failureOf returns the failure an outcome records, nothing for a run that ended well.
func failureOf(outcome cron.Outcome) *string {
	if outcome.Failure == "" {
		return nil
	}
	return &outcome.Failure
}
