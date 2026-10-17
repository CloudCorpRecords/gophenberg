// SPDX-License-Identifier: Apache-2.0

package crontest

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/cron"
)

// storeCases are the checks over taking, settling and listing cron jobs.
var storeCases = []Case{
	{"TakeClaimsANewJobFromItsTakenTime", takeClaimsANewJobFromItsTakenTime},
	{"TakeRefusesAJobNotYetDue", takeRefusesAJobNotYetDue},
	{"TakeRefusesAJobAnotherLeaseHolds", takeRefusesAJobAnotherLeaseHolds},
	{"TakeClaimsAJobWhoseLeaseRanOut", takeClaimsAJobWhoseLeaseRanOut},
	{"TwoTakesAtOnceHandOutOneLease", twoTakesAtOnceHandOutOneLease},
	{"SettleMovesTheJobOnAndFreesIt", settleMovesTheJobOnAndFreesIt},
	{"SettleRefusesATokenThatNoLongerHolds", settleRefusesATokenThatNoLongerHolds},
	{"SettleRecordsAndThenClearsAFailure", settleRecordsAndThenClearsAFailure},
	{"SlotsListEveryStoredJob", slotsListEveryStoredJob},
	{"SlotsShowWhenAHeldLeaseEnds", slotsShowWhenAHeldLeaseEnds},
	{"TakeAndSettleKeepTimesToTheMicrosecond", takeAndSettleKeepTimesToTheMicrosecond},
}

// start is the moment every case's first take happens.
var start = time.Date(2026, time.October, 13, 10, 0, 0, 0, time.UTC)

// leaseOf returns a lease on the job proven by token, taken at taken and held for a minute.
func leaseOf(name, token string, taken time.Time) cron.Lease {
	return cron.Lease{Name: name, Token: token, Taken: taken, Until: taken.Add(time.Minute)}
}

// mustTake takes the lease, failing the case unless the store hands it over.
func mustTake(t *testing.T, s Stores, lease cron.Lease) cron.Lease {
	t.Helper()
	taken, ok, err := s.Store.Take(t.Context(), lease)
	if err != nil || !ok {
		t.Fatalf("Take(%s at %s) = %v, %v, want the lease handed over", lease.Name, lease.Taken, ok, err)
	}
	return taken
}

// mustSettle settles the lease with the outcome, failing the case on any error.
func mustSettle(t *testing.T, s Stores, lease cron.Lease, outcome cron.Outcome) {
	t.Helper()
	if err := s.Store.Settle(t.Context(), lease, outcome); err != nil {
		t.Fatalf("Settle(%s) error = %v", lease.Name, err)
	}
}

// refused fails the case unless the store refuses the lease.
func refused(t *testing.T, s Stores, lease cron.Lease) {
	t.Helper()
	if _, ok, err := s.Store.Take(t.Context(), lease); err != nil || ok {
		t.Errorf("Take(%s at %s) = %v, %v, want it refused", lease.Name, lease.Taken, ok, err)
	}
}

// slotOf returns the stored slot of the job, failing the case when none is stored.
func slotOf(t *testing.T, s Stores, name string) cron.Slot {
	t.Helper()
	slots, err := s.Store.Slots(t.Context())
	if err != nil {
		t.Fatalf("Slots() error = %v", err)
	}
	for _, slot := range slots {
		if slot.Name == name {
			return slot
		}
	}
	t.Fatalf("Slots() = %+v, want %s among them", slots, name)
	return cron.Slot{}
}

// takeClaimsANewJobFromItsTakenTime hands over a job never stored, its rhythm starting at the take.
func takeClaimsANewJobFromItsTakenTime(t *testing.T, s Stores) {
	taken := mustTake(t, s, leaseOf("tidy", "first", start))

	if !taken.NextRun.Equal(start) || taken.Token != "first" {
		t.Errorf("Take() = %+v, want the lease back with its rhythm starting at %s", taken, start)
	}
}

// takeRefusesAJobNotYetDue refuses a job whose next run lies ahead.
func takeRefusesAJobNotYetDue(t *testing.T, s Stores) {
	taken := mustTake(t, s, leaseOf("tidy", "first", start))
	mustSettle(t, s, taken, cron.Outcome{NextRun: start.Add(time.Hour), Finished: start})

	refused(t, s, leaseOf("tidy", "second", start.Add(30*time.Minute)))
}

// takeRefusesAJobAnotherLeaseHolds refuses a job another run holds.
func takeRefusesAJobAnotherLeaseHolds(t *testing.T, s Stores) {
	mustTake(t, s, leaseOf("tidy", "first", start))

	refused(t, s, leaseOf("tidy", "second", start.Add(30*time.Second)))
}

// takeClaimsAJobWhoseLeaseRanOut hands over a job once the lease holding it has run out.
func takeClaimsAJobWhoseLeaseRanOut(t *testing.T, s Stores) {
	mustTake(t, s, leaseOf("tidy", "first", start))

	again := mustTake(t, s, leaseOf("tidy", "second", start.Add(time.Minute)))
	if !again.NextRun.Equal(start) {
		t.Errorf("Take() after the lease ran out = %+v, want the job still due from %s", again, start)
	}
}

// twoTakesAtOnceHandOutOneLease hands a job to exactly one of several takes at the same moment.
func twoTakesAtOnceHandOutOneLease(t *testing.T, s Stores) {
	var handed atomic.Int64
	var wg sync.WaitGroup
	for _, token := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		wg.Go(func() {
			_, ok, err := s.Store.Take(t.Context(), leaseOf("tidy", token, start))
			if err != nil {
				t.Errorf("Take(%s) error = %v", token, err)
			}
			if ok {
				handed.Add(1)
			}
		})
	}
	wg.Wait()

	if got := handed.Load(); got != 1 {
		t.Errorf("eight takes at once handed out %d leases, want 1", got)
	}
}

// settleMovesTheJobOnAndFreesIt moves a settled job to its next run and frees its lease.
func settleMovesTheJobOnAndFreesIt(t *testing.T, s Stores) {
	taken := mustTake(t, s, leaseOf("tidy", "first", start))
	mustSettle(t, s, taken, cron.Outcome{NextRun: start.Add(time.Hour), Finished: start})

	slot := slotOf(t, s, "tidy")
	if !slot.NextRun.Equal(start.Add(time.Hour)) || !slot.LeasedUntil.IsZero() {
		t.Errorf("slot = %+v, want it due an hour on and free", slot)
	}
}

// settleRefusesATokenThatNoLongerHolds refuses the settle of a run another run took the job from.
func settleRefusesATokenThatNoLongerHolds(t *testing.T, s Stores) {
	first := mustTake(t, s, leaseOf("tidy", "first", start))
	mustTake(t, s, leaseOf("tidy", "second", start.Add(2*time.Minute)))

	err := s.Store.Settle(t.Context(), first, cron.Outcome{NextRun: start.Add(time.Hour), Finished: start})
	if !errors.Is(err, cron.ErrLeaseLost) {
		t.Errorf("Settle() with a token that no longer holds error = %v, want ErrLeaseLost", err)
	}
}

// settleRecordsAndThenClearsAFailure keeps a failed run's failure until a good run clears it.
func settleRecordsAndThenClearsAFailure(t *testing.T, s Stores) {
	taken := mustTake(t, s, leaseOf("tidy", "first", start))
	mustSettle(t, s, taken, cron.Outcome{NextRun: start.Add(time.Hour), Finished: start, Failure: "the disk is full"})
	failed, err := s.LastFailure(t.Context(), "tidy")
	if err != nil || failed != "the disk is full" {
		t.Fatalf("LastFailure() = %q, %v, want the recorded failure", failed, err)
	}

	again := mustTake(t, s, leaseOf("tidy", "second", start.Add(time.Hour)))
	mustSettle(t, s, again, cron.Outcome{NextRun: start.Add(2 * time.Hour), Finished: start.Add(time.Hour)})
	cleared, err := s.LastFailure(t.Context(), "tidy")
	if err != nil || cleared != "" {
		t.Errorf("LastFailure() after a good run = %q, %v, want it cleared", cleared, err)
	}
}

// slotsListEveryStoredJob lists each job the store holds.
func slotsListEveryStoredJob(t *testing.T, s Stores) {
	mustTake(t, s, leaseOf("tidy", "first", start))
	mustTake(t, s, leaseOf("archive", "second", start))

	slots, err := s.Store.Slots(t.Context())
	if err != nil {
		t.Fatalf("Slots() error = %v", err)
	}
	var names []string
	for _, slot := range slots {
		names = append(names, slot.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"archive", "tidy"}) {
		t.Errorf("Slots() names %v, want archive and tidy", names)
	}
}

// slotsShowWhenAHeldLeaseEnds lists each held job with the end of the lease that holds it.
func slotsShowWhenAHeldLeaseEnds(t *testing.T, s Stores) {
	leases := []cron.Lease{leaseOf("tidy", "first", start), leaseOf("archive", "second", start.Add(30*time.Second))}
	for _, lease := range leases {
		mustTake(t, s, lease)
	}

	for _, lease := range leases {
		if slot := slotOf(t, s, lease.Name); !slot.LeasedUntil.Equal(lease.Until) {
			t.Errorf("slot = %+v, want it held until %s", slot, lease.Until)
		}
	}
}

// takeAndSettleKeepTimesToTheMicrosecond keeps a take's start, its lease end and the next run to the microsecond.
func takeAndSettleKeepTimesToTheMicrosecond(t *testing.T, s Stores) {
	lease := leaseOf("tidy", "first", start.Add(123456*time.Microsecond))
	taken := mustTake(t, s, lease)
	if !taken.NextRun.Equal(lease.Taken) {
		t.Errorf("Take() = %+v, want its rhythm starting at %s", taken, lease.Taken)
	}
	if slot := slotOf(t, s, "tidy"); !slot.LeasedUntil.Equal(lease.Until) {
		t.Errorf("slot after the take = %+v, want it held until %s", slot, lease.Until)
	}

	next := lease.Taken.Add(time.Hour + 654321*time.Microsecond)
	mustSettle(t, s, taken, cron.Outcome{NextRun: next, Finished: lease.Taken})
	if slot := slotOf(t, s, "tidy"); !slot.NextRun.Equal(next) {
		t.Errorf("slot after the settle = %+v, want it due at %s", slot, next)
	}
}
