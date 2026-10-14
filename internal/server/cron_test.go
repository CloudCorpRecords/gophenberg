// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/server"
)

// cronSecret is the synthetic cron secret the tests hand the server.
const cronSecret = "0123456789abcdef0123456789abcdef"

// recordedJobs stands in for the site's cron jobs, counting runs and holding each until it is let go.
type recordedJobs struct {
	mu      sync.Mutex
	calls   int
	release chan struct{}
	stopped bool
	err     error
}

// RunDue counts one run, waits for its release when one is set, and notes whether its context ended.
func (j *recordedJobs) RunDue(ctx context.Context) (cron.Report, error) {
	j.mu.Lock()
	j.calls++
	release := j.release
	j.mu.Unlock()
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
	j.mu.Lock()
	j.stopped = j.stopped || ctx.Err() != nil
	j.mu.Unlock()
	return cron.Report{Ran: 1}, j.err
}

// count returns how many runs started.
func (j *recordedJobs) count() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.calls
}

// wasStopped reports whether a run saw its context end.
func (j *recordedJobs) wasStopped() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.stopped
}

// cronServer returns a server whose cron jobs are jobs, its visits on as asked, its secret secret, logging to logs.
func cronServer(jobs *recordedJobs, onVisits bool, secret string, logs *bytes.Buffer) http.Handler {
	return server.NewServer(server.Config{
		Users:        newFakeUserStore(),
		Cron:         server.NewCronRuns(jobs, time.Minute, time.Now),
		CronSecret:   secret,
		CronOnVisits: onVisits,
		Logger:       slog.New(slog.NewTextHandler(logs, nil)),
	})
}

// bearer is the authorization header that carries the test secret.
const bearer = "Bearer " + cronSecret

// cronCall sends a cron call carrying the authorization header.
func cronCall(handler http.Handler, method, authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/api/cron", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// visit sends one visitor's page request through handler.
func visit(handler http.Handler) {
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestAfterVisitsRunsTheCronJobsAfterTheAnswer(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := &recordedJobs{}
		runsBeforeAnswer := -1
		page := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			synctest.Wait()
			runsBeforeAnswer = jobs.count()
			w.WriteHeader(http.StatusOK)
		})
		handler := server.NewServer(server.Config{
			Users: newFakeUserStore(), Plugins: map[string]http.Handler{"page": page},
			PluginPublicPaths: map[string][]string{"page": {"/"}},
			Cron:              server.NewCronRuns(jobs, time.Minute, time.Now), CronOnVisits: true,
		})

		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/plugins/page/", nil))
		synctest.Wait()

		if runsBeforeAnswer != 0 || jobs.count() != 1 {
			t.Errorf("the jobs ran %d times before the answer and %d after, want none before and one after",
				runsBeforeAnswer, jobs.count())
		}
	})
}

func TestAfterVisitsPassesRequestsThroughWhenVisitsAreOff(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := &recordedJobs{}
		handler := cronServer(jobs, false, cronSecret, &bytes.Buffer{})

		visit(handler)
		synctest.Wait()

		if jobs.count() != 0 {
			t.Errorf("a visit with visits off ran the jobs %d times, want none", jobs.count())
		}
	})
}

func TestAfterVisitsPassesRequestsThroughWithoutCronRuns(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	server.NewServer(server.Config{Users: newFakeUserStore(), CronOnVisits: true}).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/nowhere", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want the request answered as without cron runs", recorder.Code)
	}
}

func TestVisitedChecksAtMostOncePerPace(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := &recordedJobs{}
		runs := server.NewCronRuns(jobs, time.Minute, time.Now)

		runs.Visited()
		synctest.Wait()
		time.Sleep(30 * time.Second)
		runs.Visited()
		synctest.Wait()
		if jobs.count() != 1 {
			t.Fatalf("two visits inside the pace ran the jobs %d times, want once", jobs.count())
		}

		time.Sleep(30 * time.Second)
		runs.Visited()
		synctest.Wait()
		if jobs.count() != 2 {
			t.Errorf("a visit once the pace passed ran the jobs %d times in all, want twice", jobs.count())
		}
	})
}

func TestVisitedStartsNoSecondRunWhileOneRuns(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := &recordedJobs{release: make(chan struct{})}
		runs := server.NewCronRuns(jobs, time.Minute, time.Now)

		runs.Visited()
		synctest.Wait()
		time.Sleep(2 * time.Minute)
		runs.Visited()
		synctest.Wait()
		if jobs.count() != 1 {
			t.Errorf("a visit while a run is going started %d runs, want one", jobs.count())
		}
		close(jobs.release)
	})
}

func TestVisitedStartsNothingAfterClose(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := &recordedJobs{}
		runs := server.NewCronRuns(jobs, time.Minute, time.Now)
		if err := runs.Close(t.Context()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		runs.Visited()
		synctest.Wait()
		if jobs.count() != 0 {
			t.Errorf("a visit after Close ran the jobs %d times, want none", jobs.count())
		}
	})
}

func TestRunNowRefusesOnceCloseHasBegun(t *testing.T) {
	t.Parallel()

	jobs := &recordedJobs{}
	runs := server.NewCronRuns(jobs, time.Minute, time.Now)
	if err := runs.Close(t.Context()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := runs.RunNow(); !errors.Is(err, server.ErrCronClosed) || jobs.count() != 0 {
		t.Errorf("RunNow() after Close error = %v after %d runs, want ErrCronClosed and no run", err, jobs.count())
	}
}

func TestRunNowCountsAsTheVisitCheck(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := &recordedJobs{}
		runs := server.NewCronRuns(jobs, time.Minute, time.Now)

		if _, err := runs.RunNow(); err != nil {
			t.Fatalf("RunNow() error = %v", err)
		}
		runs.Visited()
		synctest.Wait()
		if jobs.count() != 1 {
			t.Errorf("a visit right after a cron call ran the jobs %d times in all, want once", jobs.count())
		}
	})
}

func TestCloseCancelsARunningRunAndWaitsForIt(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := &recordedJobs{release: make(chan struct{})}
		runs := server.NewCronRuns(jobs, time.Minute, time.Now)
		runs.Visited()
		synctest.Wait()

		if err := runs.Close(t.Context()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if !jobs.wasStopped() {
			t.Errorf("Close returned before the running run saw its stop")
		}
	})
}

func TestCutCancelsARunNowInFlightAndRefusesLaterRuns(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := &recordedJobs{release: make(chan struct{})}
		runs := server.NewCronRuns(jobs, time.Minute, time.Now)
		finished := make(chan error, 1)
		go func() {
			_, err := runs.RunNow()
			finished <- err
		}()
		synctest.Wait()

		runs.Cut()
		synctest.Wait()
		if err := <-finished; err != nil || !jobs.wasStopped() {
			t.Fatalf("the cron run ended with %v, stopped %v, want it cut at once", err, jobs.wasStopped())
		}
		time.Sleep(2 * time.Minute)
		runs.Visited()
		synctest.Wait()
		if _, err := runs.RunNow(); !errors.Is(err, server.ErrCronClosed) || jobs.count() != 1 {
			t.Errorf("after Cut RunNow() error = %v after %d runs, want ErrCronClosed and no new run", err, jobs.count())
		}
	})
}

func TestCloseGivesUpWhenItsContextEnds(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		stubborn := make(chan struct{})
		jobs := &stubbornJobs{until: stubborn}
		runs := server.NewCronRuns(jobs, time.Minute, time.Now)
		runs.Visited()
		synctest.Wait()
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		if err := runs.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Close() error = %v, want it to give up at its deadline", err)
		}
		close(stubborn)
	})
}

// stubbornJobs stands in for cron jobs that ignore their stop until let go.
type stubbornJobs struct {
	until chan struct{}
}

// RunDue waits until it is let go, whatever its context says.
func (j *stubbornJobs) RunDue(context.Context) (cron.Report, error) {
	<-j.until
	return cron.Report{}, nil
}

func TestCronCallRunsTheJobsAndCountsThem(t *testing.T) {
	t.Parallel()

	jobs := &recordedJobs{}
	recorder := cronCall(cronServer(jobs, true, cronSecret, &bytes.Buffer{}), http.MethodPost, bearer)

	if recorder.Code != http.StatusOK || jobs.count() != 1 {
		t.Fatalf("status = %d after %d runs, want 200 and one run", recorder.Code, jobs.count())
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != `{"ran":1,"failed":0,"held":0}` {
		t.Errorf("body = %s, want the counts alone", body)
	}
}

func TestCronCallKeepsRunningWhenTheCallerHangsUp(t *testing.T) {
	t.Parallel()

	jobs := &recordedJobs{}
	handler := cronServer(jobs, true, cronSecret, &bytes.Buffer{})
	ctx, hangUp := context.WithCancel(t.Context())
	hangUp()
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/cron", nil)
	request.Header.Set("Authorization", bearer)

	handler.ServeHTTP(httptest.NewRecorder(), request)
	if jobs.count() != 1 || jobs.wasStopped() {
		t.Errorf("a call whose caller hung up ran %d times, stopped %v, want one run left to finish",
			jobs.count(), jobs.wasStopped())
	}
}

func TestCronCallAnswersInternalWhenTheJobsCannotBeRead(t *testing.T) {
	t.Parallel()

	jobs := &recordedJobs{err: errors.New("the store is unreachable")}
	recorder := cronCall(cronServer(jobs, true, cronSecret, &bytes.Buffer{}), http.MethodPost, bearer)

	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"internal"`) {
		t.Errorf("status = %d body %s, want 500 with the internal code", recorder.Code, recorder.Body.String())
	}
}

func TestCronCallRefusalMatchesAnUnknownPath(t *testing.T) {
	t.Parallel()

	for name, authorization := range map[string]string{
		"a wrong secret":         "Bearer " + strings.Repeat("f", len(cronSecret)),
		"a part of the secret":   "Bearer " + cronSecret[:10],
		"no secret":              "",
		"the secret, not bearer": cronSecret,
	} {
		jobs := &recordedJobs{}
		handler := cronServer(jobs, false, cronSecret, &bytes.Buffer{})
		refused := cronCall(handler, http.MethodPost, authorization)
		unknown := httptest.NewRecorder()
		handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodPost, "/api/nowhere", nil))

		if refused.Code != http.StatusNotFound || refused.Body.String() != unknown.Body.String() ||
			refused.Header().Get("Cache-Control") != unknown.Header().Get("Cache-Control") || jobs.count() != 0 {
			t.Errorf("with %s the call answered %d %q, want %d %q like an unknown path and no run",
				name, refused.Code, refused.Body.String(), unknown.Code, unknown.Body.String())
		}
	}
}

func TestCronCallAnswersAnotherMethodNotAllowed(t *testing.T) {
	t.Parallel()

	recorder := cronCall(cronServer(&recordedJobs{}, true, cronSecret, &bytes.Buffer{}), http.MethodGet, "")

	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodPost {
		t.Errorf("GET answered %d with Allow %q, want 405 and Allow POST", recorder.Code, recorder.Header().Get("Allow"))
	}
}

func TestCronCallIsNotServedWithoutASecret(t *testing.T) {
	t.Parallel()

	jobs := &recordedJobs{}
	recorder := cronCall(cronServer(jobs, false, "", &bytes.Buffer{}), http.MethodPost, "Bearer ")

	if recorder.Code != http.StatusNotFound || jobs.count() != 0 {
		t.Errorf("with no secret set the call answered %d after %d runs, want 404 and no run", recorder.Code, jobs.count())
	}
}

func TestCronCallIsNotServedWithoutCronRuns(t *testing.T) {
	t.Parallel()

	recorder := cronCall(server.NewServer(server.Config{Users: newFakeUserStore(), CronSecret: cronSecret}),
		http.MethodPost, bearer)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("with no cron runs the call answered %d, want 404", recorder.Code)
	}
}

func TestCronCallIsNeverCached(t *testing.T) {
	t.Parallel()

	recorder := cronCall(cronServer(&recordedJobs{}, true, cronSecret, &bytes.Buffer{}), http.MethodPost, bearer)

	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestCronCallLogsARefusalWithoutTheSecret(t *testing.T) {
	t.Parallel()

	logs := &bytes.Buffer{}
	presented := "Bearer " + strings.Repeat("9", len(cronSecret))
	cronCall(cronServer(&recordedJobs{}, true, cronSecret, logs), http.MethodPost, presented)

	if !strings.Contains(logs.String(), `msg="cron call refused"`) {
		t.Errorf("the log reads %q, want the refusal logged", logs.String())
	}
	if strings.Contains(logs.String(), cronSecret) || strings.Contains(logs.String(), strings.Repeat("9", 10)) {
		t.Errorf("the log reads %q, want neither secret in it", logs.String())
	}
}
