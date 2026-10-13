// SPDX-License-Identifier: Apache-2.0

package cron_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/cron/crontest"
)

// failingStore wraps a store, refusing the calls its errors name and recording each token taken.
type failingStore struct {
	cron.Store
	slotsErr  error
	takeErr   error
	settleErr error
	takenAway bool
	onTake    func()
	mu        sync.Mutex
	tokens    []string
}

// Slots refuses with slotsErr, or reads the wrapped store.
func (f *failingStore) Slots(ctx context.Context) ([]cron.Slot, error) {
	if f.slotsErr != nil {
		return nil, f.slotsErr
	}
	return f.Store.Slots(ctx)
}

// Take records the token, then refuses, hands nothing over or takes through the wrapped store.
func (f *failingStore) Take(ctx context.Context, lease cron.Lease) (cron.Lease, bool, error) {
	f.mu.Lock()
	f.tokens = append(f.tokens, lease.Token)
	f.mu.Unlock()
	if f.onTake != nil {
		f.onTake()
	}
	if f.takeErr != nil || f.takenAway {
		return cron.Lease{}, false, f.takeErr
	}
	return f.Store.Take(ctx, lease)
}

// Settle refuses with settleErr, or settles through the wrapped store.
func (f *failingStore) Settle(ctx context.Context, lease cron.Lease, outcome cron.Outcome) error {
	if f.settleErr != nil {
		return f.settleErr
	}
	return f.Store.Settle(ctx, lease, outcome)
}

// rigOver returns a runner over store with a fresh clock, limited to a second and leasing for two.
func rigOver(store cron.Store, jobs ...cron.Job) rig {
	clock, logs := newClock(), &syncBuffer{}
	return rig{clock: clock, logs: logs, runner: cron.New(cron.Config{
		Jobs: jobs, Store: store, Clock: clock.Now, TimeLimit: time.Second, Lease: 2 * time.Second,
		Logger: slog.New(slog.NewTextHandler(logs, nil)),
	})}
}

// slot returns the stored slot of the tidy job, failing the test when none is stored.
func (r rig) slot(t *testing.T) cron.Slot {
	t.Helper()
	slots, err := r.store.Slots(t.Context())
	if err != nil {
		t.Fatalf("Slots() error = %v", err)
	}
	for _, slot := range slots {
		if slot.Name == "tidy" {
			return slot
		}
	}
	t.Fatalf("Slots() = %+v, want tidy among them", slots)
	return cron.Slot{}
}

// failure returns the failure the tidy job's last run recorded.
func (r rig) failure(t *testing.T) string {
	t.Helper()
	failed, err := r.store.LastFailure(t.Context(), "tidy")
	if err != nil {
		t.Fatalf("LastFailure() error = %v", err)
	}
	return failed
}

// failing returns a job body that fails with message.
func failing(message string) func(context.Context, time.Time) error {
	return func(context.Context, time.Time) error {
		return errors.New(message)
	}
}

func TestRunDueRunsAJobNeverStoredAtOnce(t *testing.T) {
	t.Parallel()

	job := &counter{}
	r := newRig(hourly("tidy", job.Run))

	if report := r.runDue(t); report.Ran != 1 || job.count() != 1 {
		t.Errorf("RunDue() = %+v with %d runs, want the new job run once", report, job.count())
	}
	r.logged(t, "cron job ran")
}

func TestRunDueSkipsAJobWhoseTimeHasNotCome(t *testing.T) {
	t.Parallel()

	job := &counter{}
	r := newRig(hourly("tidy", job.Run))
	r.runDue(t)
	r.clock.Advance(30 * time.Minute)

	if report := r.runDue(t); report != (cron.Report{}) || job.count() != 1 {
		t.Errorf("RunDue() half an hour on = %+v with %d runs, want nothing run", report, job.count())
	}
}

func TestRunDueCountsAJobAnotherRunHolds(t *testing.T) {
	t.Parallel()

	job := &counter{}
	r := newRig(hourly("tidy", job.Run))
	if _, ok, err := r.store.Take(t.Context(), cron.Lease{
		Name: "tidy", Token: "elsewhere", Taken: start, Until: start.Add(time.Minute),
	}); err != nil || !ok {
		t.Fatalf("Take() = %v, %v, want another run holding the job", ok, err)
	}

	if report := r.runDue(t); report != (cron.Report{Held: 1}) || job.count() != 0 {
		t.Errorf("RunDue() = %+v with %d runs, want the job counted as held and not run", report, job.count())
	}
}

func TestRunDueCountsAJobAnotherRunTookAfterTheRead(t *testing.T) {
	t.Parallel()

	job := &counter{}
	r := rigOver(&failingStore{Store: crontest.NewMemoryStore(), takenAway: true}, hourly("tidy", job.Run))

	if report := r.runDue(t); report != (cron.Report{Held: 1}) || job.count() != 0 {
		t.Errorf("RunDue() = %+v with %d runs, want the job counted as held and not run", report, job.count())
	}
}

func TestRunDueRunsJobsByNextRunThenByName(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var order []string
	recording := func(name string, every time.Duration) cron.Job {
		return cron.Job{Name: name, Every: every, Run: func(context.Context, time.Time) error {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, name)
			return nil
		}}
	}
	r := newRig(recording("zeta", time.Hour), recording("alpha", 2*time.Hour), recording("beta", 2*time.Hour))
	r.runDue(t)
	r.clock.Advance(3 * time.Hour)
	r.runDue(t)

	want := []string{"alpha", "beta", "zeta", "zeta", "alpha", "beta"}
	if !slices.Equal(order, want) {
		t.Errorf("the jobs ran in the order %v, want %v", order, want)
	}
}

func TestRunDueHandsTheJobTheTimeItWasTaken(t *testing.T) {
	t.Parallel()

	job := &counter{}
	r := newRig(hourly("tidy", job.Run))
	r.runDue(t)
	r.clock.Advance(72*time.Hour + 15*time.Minute)

	r.runDue(t)
	want := start.Add(72*time.Hour + 15*time.Minute)
	if len(job.at) != 2 || !job.at[1].Equal(want) {
		t.Errorf("the job ran at %v, want its late run at %s, the time it was taken", job.at, want)
	}
}

func TestRunDueEndsAJobWithItsLeaseWhenTheTakeWasSlow(t *testing.T) {
	t.Parallel()

	var left time.Duration
	store := &failingStore{Store: crontest.NewMemoryStore()}
	r := rigOver(store, hourly("tidy", func(ctx context.Context, _ time.Time) error {
		deadline, _ := ctx.Deadline()
		left = time.Until(deadline)
		return nil
	}))
	store.onTake = func() { r.clock.Advance(1500 * time.Millisecond) }

	r.runDue(t)
	if left > 750*time.Millisecond {
		t.Errorf("the job had %s to run after a slow take, want no more than the half second its lease had left", left)
	}
}

// endedStore wraps a memory store, refusing a settle whose context has ended as a database pool does.
type endedStore struct {
	*crontest.MemoryStore
}

// Settle refuses an ended context, or settles through the memory store.
func (e endedStore) Settle(ctx context.Context, lease cron.Lease, outcome cron.Outcome) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return e.MemoryStore.Settle(ctx, lease, outcome)
}

func TestRunDueSettlesACutJobThroughAStoreThatRefusesEndedContexts(t *testing.T) {
	t.Parallel()

	store := endedStore{crontest.NewMemoryStore()}
	ctx, cancel := context.WithCancel(t.Context())
	r := rigOver(store, hourly("tidy", func(cut context.Context, _ time.Time) error {
		cancel()
		<-cut.Done()
		return cut.Err()
	}))

	if _, err := r.runner.RunDue(ctx); err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	failed, err := store.LastFailure(t.Context(), "tidy")
	if err != nil || failed != context.Canceled.Error() {
		t.Errorf("the cut job recorded %q (%v), want its stop recorded although its context had ended", failed, err)
	}
	if strings.Contains(r.logs.String(), `msg="cron job unsettled"`) {
		t.Errorf("the log reads %q, want the cut job settled", r.logs.String())
	}
}

func TestRunDueMovesAnEarlyFinishOneIntervalOn(t *testing.T) {
	t.Parallel()

	r := newRig(hourly("tidy", (&counter{}).Run))
	r.runDue(t)

	if slot := r.slot(t); !slot.NextRun.Equal(start.Add(time.Hour)) || !slot.LeasedUntil.IsZero() {
		t.Errorf("slot = %+v, want it free and due an hour after its first run", slot)
	}
}

func TestRunDueKeepsTheRhythmOfALateJob(t *testing.T) {
	t.Parallel()

	job := &counter{}
	r := newRig(hourly("tidy", job.Run))
	r.runDue(t)
	r.clock.Advance(72*time.Hour + 15*time.Minute)

	r.runDue(t)
	want := start.Add(73 * time.Hour)
	if slot := r.slot(t); job.count() != 2 || !slot.NextRun.Equal(want) {
		t.Errorf("after %d runs the job falls due at %s, want two runs and %s", job.count(), slot.NextRun, want)
	}
}

func TestRunDueRecordsAFailureAndMovesToTheNextSlot(t *testing.T) {
	t.Parallel()

	r := newRig(hourly("tidy", failing("the disk is full")))

	if report := r.runDue(t); report != (cron.Report{Failed: 1}) {
		t.Errorf("RunDue() = %+v, want one failure", report)
	}
	if r.failure(t) != "the disk is full" || !r.slot(t).NextRun.Equal(start.Add(time.Hour)) {
		t.Errorf("the job recorded %q and falls due at %s, want the failure and the next slot",
			r.failure(t), r.slot(t).NextRun)
	}
	r.logged(t, "cron job failed")
}

func TestRunDueClearsTheFailureOfAJobThatRanWell(t *testing.T) {
	t.Parallel()

	fails := true
	r := newRig(hourly("tidy", func(context.Context, time.Time) error {
		if fails {
			return errors.New("the disk is full")
		}
		return nil
	}))
	r.runDue(t)
	fails = false
	r.clock.Advance(time.Hour)

	r.runDue(t)
	if failed := r.failure(t); failed != "" {
		t.Errorf("after a good run the job records %q, want no failure", failed)
	}
}

func TestRunDueTurnsAPanicIntoAFailure(t *testing.T) {
	t.Parallel()

	r := newRig(hourly("tidy", func(context.Context, time.Time) error { panic("the index is torn") }))

	if report := r.runDue(t); report != (cron.Report{Failed: 1}) {
		t.Errorf("RunDue() = %+v, want one failure", report)
	}
	failed := r.failure(t)
	if !strings.Contains(failed, "panic") || !strings.Contains(failed, "the index is torn") {
		t.Errorf("the job recorded %q, want the panic and its value", failed)
	}
}

func TestRunDueCutsAJobAtTheTimeLimit(t *testing.T) {
	t.Parallel()

	r := newRig(hourly("tidy", func(ctx context.Context, _ time.Time) error {
		<-ctx.Done()
		return ctx.Err()
	}))

	r.runDue(t)
	if failed := r.failure(t); failed != context.DeadlineExceeded.Error() {
		t.Errorf("the job recorded %q, want it cut at its time limit", failed)
	}
}

func TestRunDueRecordsAJobThatReturnsPastItsTimeLimit(t *testing.T) {
	t.Parallel()

	r := newRig(hourly("tidy", func(ctx context.Context, _ time.Time) error {
		<-ctx.Done()
		time.Sleep(time.Millisecond)
		return nil
	}))

	if report := r.runDue(t); report != (cron.Report{Failed: 1}) {
		t.Errorf("RunDue() = %+v, want the overrun counted as a failure", report)
	}
	if failed := r.failure(t); failed != context.DeadlineExceeded.Error() {
		t.Errorf("the job recorded %q, want the overrun recorded", failed)
	}
}

func TestRunDueStartsNoJobOnceItsContextEnds(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	later := &counter{}
	r := newRig(
		hourly("alpha", func(context.Context, time.Time) error { cancel(); return nil }),
		hourly("beta", later.Run),
	)

	if _, err := r.runner.RunDue(ctx); err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if later.count() != 0 {
		t.Errorf("the job after the stop ran %d times, want none", later.count())
	}
}

func TestRunDueKeepsAJobItsContextCutDue(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	r := newRig(hourly("tidy", func(cut context.Context, _ time.Time) error {
		cancel()
		<-cut.Done()
		return cut.Err()
	}))

	if _, err := r.runner.RunDue(ctx); err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	slot := r.slot(t)
	if !slot.NextRun.Equal(start) || !slot.LeasedUntil.IsZero() {
		t.Errorf("slot after a stop = %+v, want it free and still due at %s", slot, start)
	}
	if failed := r.failure(t); failed != context.Canceled.Error() {
		t.Errorf("the cut job recorded %q, want the stop recorded", failed)
	}
}

func TestRunDueReturnsTheErrorOfSlotsItCannotRead(t *testing.T) {
	t.Parallel()

	unread := errors.New("the store is unreachable")
	r := rigOver(&failingStore{Store: crontest.NewMemoryStore(), slotsErr: unread}, hourly("tidy", (&counter{}).Run))

	if _, err := r.runner.RunDue(t.Context()); !errors.Is(err, unread) {
		t.Errorf("RunDue() error = %v, want the store's error wrapped", err)
	}
	r.logged(t, "cron jobs unread")
}

func TestRunDueReportsATakeTheStoreRefuses(t *testing.T) {
	t.Parallel()

	job := &counter{}
	r := rigOver(&failingStore{Store: crontest.NewMemoryStore(), takeErr: errors.New("the store is unreachable")},
		hourly("tidy", job.Run))

	if report := r.runDue(t); report != (cron.Report{Failed: 1}) || job.count() != 0 {
		t.Errorf("RunDue() = %+v with %d runs, want one failure and no run", report, job.count())
	}
	r.logged(t, "cron job failed")
}

func TestRunDueReportsASettleTheStoreRefuses(t *testing.T) {
	t.Parallel()

	r := rigOver(&failingStore{Store: crontest.NewMemoryStore(), settleErr: errors.New("the store is unreachable")},
		hourly("tidy", (&counter{}).Run))

	if report := r.runDue(t); report != (cron.Report{Failed: 1}) {
		t.Errorf("RunDue() = %+v, want the run whose outcome went unsaved counted as failed", report)
	}
	r.logged(t, "cron job unsettled")
	if strings.Contains(r.logs.String(), `msg="cron job ran"`) {
		t.Errorf("the log reads %q, want no success logged for a run whose outcome went unsaved", r.logs.String())
	}
}

func TestRunDueStopsAtTheFirstSettleTheStoreRefuses(t *testing.T) {
	t.Parallel()

	tidy, vacuum := &counter{}, &counter{}
	r := rigOver(&failingStore{Store: crontest.NewMemoryStore(), settleErr: errors.New("the store is unreachable")},
		hourly("tidy", tidy.Run), hourly("vacuum", vacuum.Run))

	if report := r.runDue(t); report != (cron.Report{Failed: 1}) || vacuum.count() != 0 {
		t.Errorf("RunDue() = %+v with %d later runs, want the pass stopped at the refused settle",
			report, vacuum.count())
	}
}

func TestRunDueGoesOnAfterALostLease(t *testing.T) {
	t.Parallel()

	tidy, vacuum := &counter{}, &counter{}
	r := rigOver(&failingStore{Store: crontest.NewMemoryStore(), settleErr: cron.ErrLeaseLost},
		hourly("tidy", tidy.Run), hourly("vacuum", vacuum.Run))

	if report := r.runDue(t); report != (cron.Report{Failed: 2}) || vacuum.count() != 1 {
		t.Errorf("RunDue() = %+v with %d later runs, want the pass to go on past a lost lease", report, vacuum.count())
	}
}

// stalledSlots wraps a store, holding every read of the slots until its context ends.
type stalledSlots struct {
	cron.Store
}

// Slots waits for ctx to end and returns why it ended.
func (stalledSlots) Slots(ctx context.Context) ([]cron.Slot, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRunDueGivesUpOnASlotsReadThatStallsPastTheLease(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		r := rigOver(stalledSlots{crontest.NewMemoryStore()}, hourly("tidy", (&counter{}).Run))
		began := time.Now()

		if _, err := r.runner.RunDue(t.Context()); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("RunDue() error = %v, want the stalled read given up", err)
		}
		if waited := time.Since(began); waited != 2*time.Second {
			t.Errorf("RunDue() gave up after %s, want the two second lease", waited)
		}
	})
}

// stalledTake wraps a store, holding every take until its context ends.
type stalledTake struct {
	cron.Store
}

// Take waits for ctx to end and returns why it ended.
func (stalledTake) Take(ctx context.Context, _ cron.Lease) (cron.Lease, bool, error) {
	<-ctx.Done()
	return cron.Lease{}, false, ctx.Err()
}

func TestRunDueGivesUpOnATakeThatStallsPastTheLease(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		tidy, vacuum := &counter{}, &counter{}
		r := rigOver(stalledTake{crontest.NewMemoryStore()}, hourly("tidy", tidy.Run), hourly("vacuum", vacuum.Run))
		began := time.Now()

		if report := r.runDue(t); report != (cron.Report{Failed: 1}) || tidy.count()+vacuum.count() != 0 {
			t.Errorf("RunDue() = %+v with %d runs, want the pass stopped at the stalled take and no run",
				report, tidy.count()+vacuum.count())
		}
		if waited := time.Since(began); waited != 2*time.Second {
			t.Errorf("RunDue() gave up after %s, want one two second lease for the whole pass", waited)
		}
	})
}

func TestRunDueReportsTheLostLeaseOfAJobThatOverranIt(t *testing.T) {
	t.Parallel()

	store, clock := crontest.NewMemoryStore(), newClock()
	logs := &syncBuffer{}
	var second *cron.Runner
	overran := false
	body := func(ctx context.Context, _ time.Time) error {
		if overran {
			return nil
		}
		overran = true
		clock.Advance(time.Minute)
		_, err := second.RunDue(ctx)
		return err
	}
	build := func(log *syncBuffer) *cron.Runner {
		return cron.New(cron.Config{
			Jobs: []cron.Job{hourly("tidy", body)}, Store: store, Clock: clock.Now,
			TimeLimit: time.Second, Lease: 2 * time.Second, Logger: slog.New(slog.NewTextHandler(log, nil)),
		})
	}
	first := build(logs)
	second = build(&syncBuffer{})

	if _, err := first.RunDue(t.Context()); err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if !strings.Contains(logs.String(), `msg="cron job lease lost"`) {
		t.Errorf("the log reads %q, want the overrun's settle refused as a lost lease", logs.String())
	}
}

func TestRunDueGivesEachTakeAFreshToken(t *testing.T) {
	t.Parallel()

	store := &failingStore{Store: crontest.NewMemoryStore()}
	r := rigOver(store, hourly("tidy", (&counter{}).Run))
	r.runDue(t)
	r.clock.Advance(time.Hour)
	r.runDue(t)

	if len(store.tokens) != 2 || store.tokens[0] == "" || store.tokens[0] == store.tokens[1] {
		t.Errorf("the takes carried the tokens %q, want two different ones", store.tokens)
	}
}
