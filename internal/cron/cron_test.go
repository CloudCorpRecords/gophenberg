// SPDX-License-Identifier: Apache-2.0

package cron_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/cron/crontest"
)

// start is the moment the fake clock reads first.
var start = time.Date(2026, time.October, 13, 10, 0, 0, 0, time.UTC)

// fakeClock is a clock the tests move forward by hand.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// newClock returns a clock reading start.
func newClock() *fakeClock {
	return &fakeClock{now: start}
}

// Now returns the time the clock reads.
func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// syncBuffer is a log buffer safe to write from several goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write keeps one log line.
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns every line kept so far.
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// counter is a job body that counts its runs.
type counter struct {
	mu   sync.Mutex
	runs int
	at   []time.Time
}

// Run counts one run at now.
func (c *counter) Run(_ context.Context, now time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runs++
	c.at = append(c.at, now)
	return nil
}

// count returns how many times the job ran.
func (c *counter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runs
}

// hourly returns a job of the name falling due every hour, running body.
func hourly(name string, body func(context.Context, time.Time) error) cron.Job {
	return cron.Job{Name: name, Every: time.Hour, Run: body}
}

// rig is one runner under test with its store, clock and log.
type rig struct {
	store  *crontest.MemoryStore
	clock  *fakeClock
	logs   *syncBuffer
	runner *cron.Runner
}

// newRig returns a runner over a fresh memory store and clock, limited to a second and leasing for two.
func newRig(jobs ...cron.Job) rig {
	store, clock, logs := crontest.NewMemoryStore(), newClock(), &syncBuffer{}
	return rig{store: store, clock: clock, logs: logs, runner: cron.New(cron.Config{
		Jobs: jobs, Store: store, Clock: clock.Now, TimeLimit: time.Second, Lease: 2 * time.Second,
		Logger: slog.New(slog.NewTextHandler(logs, nil)),
	})}
}

// runDue runs the cron jobs once, failing the test when the slots cannot be read.
func (r rig) runDue(t *testing.T) cron.Report {
	t.Helper()
	report, err := r.runner.RunDue(t.Context())
	if err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	return report
}

// logged fails the test unless the log holds the message.
func (r rig) logged(t *testing.T, message string) {
	t.Helper()
	if !strings.Contains(r.logs.String(), `msg="`+message+`"`) {
		t.Errorf("the log reads %q, want %q in it", r.logs.String(), message)
	}
}

func TestMemoryStoreRunsTheSharedSuite(t *testing.T) {
	t.Parallel()

	crontest.Run(t, func(*testing.T) crontest.Stores {
		store := crontest.NewMemoryStore()
		return crontest.Stores{Store: store, LastFailure: store.LastFailure}
	})
}

func TestNewLeavesOutAJobWithNoName(t *testing.T) {
	t.Parallel()

	job := &counter{}
	r := newRig(hourly("", job.Run))

	r.runDue(t)
	if job.count() != 0 {
		t.Errorf("the job with no name ran %d times, want it left out", job.count())
	}
	r.logged(t, "cron job left out")
}

func TestNewLeavesOutTheSecondJobOfAName(t *testing.T) {
	t.Parallel()

	first, second := &counter{}, &counter{}
	r := newRig(hourly("tidy", first.Run), hourly("tidy", second.Run))

	r.runDue(t)
	if first.count() != 1 || second.count() != 0 {
		t.Errorf("the two jobs named tidy ran %d and %d times, want only the first once", first.count(), second.count())
	}
	r.logged(t, "cron job left out")
}

func TestNewLeavesOutAJobWhoseIntervalIsNotAboveZero(t *testing.T) {
	t.Parallel()

	for _, every := range []time.Duration{0, -time.Hour} {
		job := &counter{}
		r := newRig(cron.Job{Name: "tidy", Every: every, Run: job.Run})

		r.runDue(t)
		if job.count() != 0 {
			t.Errorf("a job every %s ran %d times, want it left out", every, job.count())
		}
		r.logged(t, "cron job left out")
	}
}

func TestNewLeavesOutAJobWithNoRun(t *testing.T) {
	t.Parallel()

	r := newRig(cron.Job{Name: "tidy", Every: time.Hour})

	if report := r.runDue(t); report != (cron.Report{}) {
		t.Errorf("RunDue() = %+v, want nothing run for a job with no run", report)
	}
	r.logged(t, "cron job left out")
}

func TestNewRunsTheJobsItKeptBesideOneItLeftOut(t *testing.T) {
	t.Parallel()

	kept := &counter{}
	r := newRig(hourly("", (&counter{}).Run), hourly("tidy", kept.Run))

	if report := r.runDue(t); report.Ran != 1 || kept.count() != 1 {
		t.Errorf("RunDue() = %+v with tidy run %d times, want the kept job run once", report, kept.count())
	}
}

func TestNewRunsNothingUnderAConfigItCannotUse(t *testing.T) {
	t.Parallel()

	for name, cfg := range map[string]cron.Config{
		"no store":                         {TimeLimit: time.Second, Lease: 2 * time.Second},
		"a time limit not above zero":      {Store: crontest.NewMemoryStore(), Lease: time.Second},
		"a lease not above the time limit": {Store: crontest.NewMemoryStore(), TimeLimit: time.Second, Lease: time.Second},
	} {
		job, logs := &counter{}, &syncBuffer{}
		cfg.Jobs, cfg.Logger = []cron.Job{hourly("tidy", job.Run)}, slog.New(slog.NewTextHandler(logs, nil))

		report, err := cron.New(cfg).RunDue(t.Context())
		if err != nil || report != (cron.Report{}) || job.count() != 0 {
			t.Errorf("with %s RunDue() = %+v, %v after %d runs, want nothing run", name, report, err, job.count())
		}
		if !strings.Contains(logs.String(), `msg="cron config refused"`) || !strings.Contains(logs.String(), name) {
			t.Errorf("with %s the log reads %q, want the config refused for that reason", name, logs.String())
		}
	}
}

func TestNewReadsTheSystemClockWhenNoneIsGiven(t *testing.T) {
	t.Parallel()

	job := &counter{}
	before := time.Now()
	runner := cron.New(cron.Config{
		Jobs: []cron.Job{hourly("tidy", job.Run)}, Store: crontest.NewMemoryStore(),
		TimeLimit: time.Second, Lease: 2 * time.Second,
	})

	if _, err := runner.RunDue(t.Context()); err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if len(job.at) != 1 || job.at[0].Before(before) || job.at[0].After(time.Now()) {
		t.Errorf("the job ran at %v, want one run at the system clock's time", job.at)
	}
}
