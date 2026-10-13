// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/cron"
)

// cronPath is the address the host's cron calls to run the cron jobs.
const cronPath = "/api/cron"

// ErrCronClosed reports a run asked for once the cron runs began to close.
var ErrCronClosed = errors.New("server: cron runs closed")

// CronRunner runs the site's cron jobs once and reports what it did.
type CronRunner interface {
	RunDue(ctx context.Context) (cron.Report, error)
}

// CronRuns owns every cron run of one server, starting visit runs in the background and cutting all at Close.
type CronRuns struct {
	runner    CronRunner
	pace      time.Duration
	clock     func() time.Time
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	nextCheck time.Time
	going     bool
	closed    bool
	runs      sync.WaitGroup
}

// NewCronRuns returns the runs over runner, a visit checking at most once per pace on clock.
func NewCronRuns(runner CronRunner, pace time.Duration, clock func() time.Time) *CronRuns {
	ctx, cancel := context.WithCancel(context.Background())
	return &CronRuns{runner: runner, pace: pace, clock: clock, ctx: ctx, cancel: cancel}
}

// Visited starts a background run when the pace allows and no visit run is going, and returns at once.
func (c *CronRuns) Visited() {
	now := c.clock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.going {
		return
	}
	if now.Before(c.nextCheck) {
		return
	}
	c.going = true
	c.nextCheck = now.Add(c.pace)
	c.runs.Go(func() {
		_, _ = c.runner.RunDue(c.ctx)
		c.mu.Lock()
		defer c.mu.Unlock()
		c.going = false
	})
}

// RunNow runs the cron jobs in the caller's goroutine under the runs' own context, refusing once Close has begun.
func (c *CronRuns) RunNow() (cron.Report, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return cron.Report{}, ErrCronClosed
	}
	c.nextCheck = c.clock().Add(c.pace)
	c.runs.Add(1)
	c.mu.Unlock()
	defer c.runs.Done()
	return c.runner.RunDue(c.ctx)
}

// Cut refuses later runs and cancels every running one without waiting.
func (c *CronRuns) Cut() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.cancel()
}

// Close cuts every run and waits for them until ctx ends.
func (c *CronRuns) Close(ctx context.Context) error {
	c.Cut()
	done := make(chan struct{})
	go func() {
		c.runs.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("server: cron runs still running: %w", ctx.Err())
	}
}

// afterVisits calls Visited once each request has answered, passing requests through when visits cannot run.
func afterVisits(runs *CronRuns, on bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if runs == nil || !on {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			runs.Visited()
		})
	}
}

// cronAnswer is what a cron call learns: how many jobs ran, failed and were held by another run.
type cronAnswer struct {
	Ran    int `json:"ran"`
	Failed int `json:"failed"`
	Held   int `json:"held"`
}

// handleCronCall runs the cron jobs for a caller holding the secret and answers how many ran.
func handleCronCall(runs *CronRuns, secret string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		presented, bearer := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !bearer || !secretMatches(presented, secret) {
			logger.Warn("cron call refused", "method", r.Method, "path", r.URL.Path)
			respondNotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", uncachedControl)
		report, err := runs.RunNow()
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, cronAnswer{Ran: report.Ran, Failed: report.Failed, Held: report.Held})
	}
}

// secretMatches reports whether the presented secret equals the site's, comparing their hashes in constant time.
func secretMatches(presented, secret string) bool {
	presentedHash, secretHash := sha256.Sum256([]byte(presented)), sha256.Sum256([]byte(secret))
	return subtle.ConstantTimeCompare(presentedHash[:], secretHash[:]) == 1
}

// mountCron serves the cron address when the site has cron runs and a secret.
func mountCron(r chi.Router, cfg Config, logger *slog.Logger) {
	if cfg.Cron == nil || cfg.CronSecret == "" {
		return
	}
	r.Post(cronPath, handleCronCall(cfg.Cron, cfg.CronSecret, logger))
}
