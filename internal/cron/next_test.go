// SPDX-License-Identifier: Apache-2.0

package cron_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/cron/crontest"
)

func TestNextRunNamesWhenAStoredJobFallsDue(t *testing.T) {
	t.Parallel()

	store := crontest.NewMemoryStore()
	lease, taken, err := store.Take(t.Context(), cron.Lease{
		Name: "tidy", Token: "first", Taken: start, Until: start.Add(time.Minute),
	})
	if err != nil || !taken {
		t.Fatalf("Take() = %v, %v, want the job taken", taken, err)
	}
	if err := store.Settle(t.Context(), lease, cron.Outcome{NextRun: start.Add(time.Hour), Finished: start}); err != nil {
		t.Fatalf("Settle() = %v, want nil", err)
	}

	next, err := cron.NextRun(t.Context(), store, "tidy")

	if err != nil || !next.Equal(start.Add(time.Hour)) {
		t.Errorf("NextRun() = %v, %v, want %v", next, err, start.Add(time.Hour))
	}
}

func TestNextRunOfAJobTheStoreHoldsNoSlotOfIsTheZeroTime(t *testing.T) {
	t.Parallel()

	next, err := cron.NextRun(t.Context(), crontest.NewMemoryStore(), "tidy")

	if err != nil || !next.IsZero() {
		t.Errorf("NextRun() of a job never stored = %v, %v, want the zero time", next, err)
	}
}

func TestNextRunReportsSlotsItCannotRead(t *testing.T) {
	t.Parallel()

	unread := errors.New("the store is unreachable")

	_, err := cron.NextRun(t.Context(), &failingStore{Store: crontest.NewMemoryStore(), slotsErr: unread}, "tidy")

	if !errors.Is(err, unread) {
		t.Errorf("NextRun() over a store it cannot read = %v, want %v", err, unread)
	}
}
