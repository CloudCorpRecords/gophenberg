// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/cron/crontest"
	"github.com/gopherium/gophenberg/internal/server"
)

// cronSecret is the synthetic secret the host's cron holds in the scenarios.
const cronSecret = "0123456789abcdef0123456789abcdef"

// cronTimeLimit is how long a scenario's job may run before it is cut.
const cronTimeLimit = 50 * time.Millisecond

// quietWindow is how long a step waits to be sure nothing more starts.
const quietWindow = 100 * time.Millisecond

// stepClock is the clock the scenarios move forward by hand.
type stepClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now returns the time the clock reads.
func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// set moves the clock to at.
func (c *stepClock) set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = at
}

// advance moves the clock forward by d.
func (c *stepClock) advance(d time.Duration) {
	c.set(c.Now().Add(d))
}

// checkedStore keeps cron jobs in memory and counts every check of them.
type checkedStore struct {
	*crontest.MemoryStore
	checks atomic.Int64
}

// Slots counts one check and reads the stored jobs.
func (s *checkedStore) Slots(ctx context.Context) ([]cron.Slot, error) {
	s.checks.Add(1)
	return s.MemoryStore.Slots(ctx)
}

// sceneJob is one job a scenario gives the site, behaving as the scenario says.
type sceneJob struct {
	name         string
	mu           sync.Mutex
	runs         int
	finished     int
	release      chan struct{}
	releaseOnce  sync.Once
	failWith     string
	untilStopped bool
	panics       bool
}

// Run counts one run and behaves as the scenario set it to.
func (j *sceneJob) Run(ctx context.Context, _ time.Time) error {
	j.mu.Lock()
	j.runs++
	j.mu.Unlock()
	defer j.end()
	switch {
	case j.panics:
		panic("the archive is torn")
	case j.release != nil:
		<-j.release
	case j.untilStopped:
		<-ctx.Done()
		return ctx.Err()
	case j.failWith != "":
		return errors.New(j.failWith)
	}
	return nil
}

// letGo lets a job that waits until it is let go finish, closing its wait at most once.
func (j *sceneJob) letGo() {
	j.releaseOnce.Do(func() {
		if j.release != nil {
			close(j.release)
		}
	})
}

// end counts one finished run.
func (j *sceneJob) end() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.finished++
}

// counts returns how many runs started and how many finished.
func (j *sceneJob) counts() (int, int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.runs, j.finished
}

// cronWorld is a scenario's cron jobs: their store, clock, jobs and the site's trigger settings.
type cronWorld struct {
	clock    *stepClock
	store    *checkedStore
	mu       sync.Mutex
	jobs     []*sceneJob
	secret   string
	onVisits bool
	runs     *server.CronRuns
	passes   atomic.Int64
}

// newCronWorld returns a scenario's cron jobs holding one hourly job of the name, the cron secret set and visits on.
func newCronWorld(name string) *cronWorld {
	cw := &cronWorld{
		clock:    &stepClock{now: time.Date(2026, time.October, 13, 9, 0, 0, 0, time.UTC)},
		store:    &checkedStore{MemoryStore: crontest.NewMemoryStore()},
		jobs:     []*sceneJob{{name: name}},
		secret:   cronSecret,
		onVisits: true,
	}
	cw.runs = server.NewCronRuns(cw, time.Minute, cw.clock.Now)
	return cw
}

// RunDue runs the scenario's jobs once over its store and clock, counting the finished pass.
func (cw *cronWorld) RunDue(ctx context.Context) (cron.Report, error) {
	defer cw.passes.Add(1)
	cw.mu.Lock()
	jobs := make([]cron.Job, 0, len(cw.jobs))
	for _, job := range cw.jobs {
		jobs = append(jobs, cron.Job{Name: job.name, Every: time.Hour, Run: job.Run})
	}
	cw.mu.Unlock()
	return cron.New(cron.Config{
		Jobs: jobs, Store: cw.store, Clock: cw.clock.Now, TimeLimit: cronTimeLimit, Lease: time.Minute,
	}).RunDue(ctx)
}

// into hands the server the scenario's cron runs and trigger settings.
func (cw *cronWorld) into(cfg *server.Config) {
	cfg.Cron, cfg.CronSecret, cfg.CronOnVisits = cw.runs, cw.secret, cw.onVisits
}

// stop lets every held job go, then closes the scenario's cron runs, waiting at most readyWait.
func (cw *cronWorld) stop(ctx context.Context) error {
	cw.mu.Lock()
	for _, job := range cw.jobs {
		job.letGo()
	}
	cw.mu.Unlock()
	waiting, cancel := context.WithTimeout(context.WithoutCancel(ctx), readyWait)
	defer cancel()
	return cw.runs.Close(waiting)
}

// job returns the scenario's job of the name.
func (cw *cronWorld) job(name string) (*sceneJob, error) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	for _, job := range cw.jobs {
		if job.name == name {
			return job, nil
		}
	}
	return nil, fmt.Errorf("the site has no cron job %q", name)
}

// add gives the site one more hourly job of the name.
func (cw *cronWorld) add(job *sceneJob) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.jobs = append(cw.jobs, job)
}

// cronWorldOf returns the scenario's cron jobs, or the error that no step gave the site any.
func cronWorldOf(ctx context.Context) (*world, *cronWorld, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, nil, err
	}
	if w.cron == nil {
		return nil, nil, errors.New("the scenario gave the site no cron jobs")
	}
	return w, w.cron, nil
}

// eventually waits until held reports true, failing with what it last said at the deadline.
func eventually(held func() (bool, string)) error {
	deadline := time.Now().Add(readyWait)
	for {
		ok, said := held()
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New(said)
		}
		time.Sleep(readyPoll)
	}
}

// aRunningGophenbergWithTheCronJob starts a site with one hourly cron job and the cron secret set.
func aRunningGophenbergWithTheCronJob(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	w.cron = newCronWorld(name)
	return w.start(ctx)
}

// theCronJobEveryHourThatPanics gives the site one more hourly job that panics.
func theCronJobEveryHourThatPanics(ctx context.Context, name string) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	cw.add(&sceneJob{name: name, panics: true})
	return nil
}

// theJobBehaves sets the named job to behave as change says.
func theJobBehaves(ctx context.Context, name string, change func(*sceneJob)) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	job, err := cw.job(name)
	if err != nil {
		return err
	}
	change(job)
	return nil
}

// theJobWaitsUntilItIsLetGo makes the job hold its run until the scenario lets it go.
func theJobWaitsUntilItIsLetGo(ctx context.Context, name string) error {
	return theJobBehaves(ctx, name, func(job *sceneJob) { job.release = make(chan struct{}) })
}

// theJobFailsWith makes the job fail with the message.
func theJobFailsWith(ctx context.Context, name, message string) error {
	return theJobBehaves(ctx, name, func(job *sceneJob) { job.failWith = message })
}

// theJobRunsUntilItIsToldToStop makes the job run until its context ends.
func theJobRunsUntilItIsToldToStop(ctx context.Context, name string) error {
	return theJobBehaves(ctx, name, func(job *sceneJob) { job.untilStopped = true })
}

// aVisitorOpensTheHomePage sends one visitor's request for the home page.
func aVisitorOpensTheHomePage(ctx context.Context) error {
	w, _, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	return w.get("/")
}

// aVisitorOpenedTheHomePage sends one visitor's request and waits until the check it started is over.
func aVisitorOpenedTheHomePage(ctx context.Context) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	before := cw.passes.Load()
	if err := aVisitorOpensTheHomePage(ctx); err != nil {
		return err
	}
	return eventually(func() (bool, string) {
		return cw.passes.Load() > before, "the visit's check never finished"
	})
}

// aVisitorOpenedTheHomePageAt moves the clock to the moment and sends one visitor's request, waiting for its check.
func aVisitorOpenedTheHomePageAt(ctx context.Context, at string) error {
	if err := theClockReads(ctx, at); err != nil {
		return err
	}
	return aVisitorOpenedTheHomePage(ctx)
}

// twoVisitorsOpenTheHomePageAtTheSameMoment sends two visitors' requests at once.
func twoVisitorsOpenTheHomePageAtTheSameMoment(ctx context.Context) error {
	w, _, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	failed := make([]error, 2)
	for visitor := range failed {
		wg.Go(func() {
			_, failed[visitor] = w.send(http.MethodGet, "/", "", nil)
		})
	}
	wg.Wait()
	return errors.Join(failed...)
}

// timePasses moves the clock forward by the amount and unit.
func timePasses(ctx context.Context, amount int, unit string) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	step := time.Second
	if strings.HasPrefix(unit, "minute") {
		step = time.Minute
	}
	cw.clock.advance(time.Duration(amount) * step)
	return nil
}

// theClockReads moves the clock to the moment.
func theClockReads(ctx context.Context, at string) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	moment, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return err
	}
	cw.clock.set(moment)
	return nil
}

// theJobHasRun asserts how many finished runs the job has, waiting for the count and then for quiet.
func theJobHasRun(ctx context.Context, name string, want int) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	job, err := cw.job(name)
	if err != nil {
		return err
	}
	ran := func() (bool, string) {
		runs, finished := job.counts()
		return runs == want && finished == want, fmt.Sprintf("the job %q ran %d times, want %d", name, runs, want)
	}
	if err := eventually(ran); err != nil {
		return err
	}
	time.Sleep(quietWindow)
	if ok, said := ran(); !ok {
		return errors.New(said)
	}
	return nil
}

// theVisitorGotThePage asserts the visitor's request was answered.
func theVisitorGotThePage(ctx context.Context) error {
	w, _, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	if w.answer == nil || w.answer.status == 0 {
		return errors.New("the visitor got no answer")
	}
	return nil
}

// theJobIsStillRunning asserts the job started a run that has not finished.
func theJobIsStillRunning(ctx context.Context, name string) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	job, err := cw.job(name)
	if err != nil {
		return err
	}
	return eventually(func() (bool, string) {
		runs, finished := job.counts()
		return runs == 1 && finished == 0, fmt.Sprintf("the job %q started %d runs and finished %d", name, runs, finished)
	})
}

// theJobFinishesOnceItIsLetGo lets the held job go and waits for its run to finish.
func theJobFinishesOnceItIsLetGo(ctx context.Context, name string) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	job, err := cw.job(name)
	if err != nil {
		return err
	}
	job.letGo()
	return theJobHasRun(ctx, name, 1)
}

// theSiteCheckedForCronJobs asserts how many checks of the cron jobs the site made.
func theSiteCheckedForCronJobs(ctx context.Context, want int) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	time.Sleep(quietWindow)
	if got := cw.store.checks.Load(); got != int64(want) {
		return fmt.Errorf("the site checked for cron jobs %d times, want %d", got, want)
	}
	return nil
}

// theJobFallsDueNextAt asserts when the store says the job falls due next.
func theJobFallsDueNextAt(ctx context.Context, name, at string) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	want, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return err
	}
	return eventually(func() (bool, string) {
		slots, _ := cw.store.MemoryStore.Slots(ctx)
		for _, slot := range slots {
			if slot.Name == name && slot.NextRun.Equal(want) && slot.LeasedUntil.IsZero() {
				return true, ""
			}
		}
		return false, fmt.Sprintf("the stored jobs are %+v, want %q free and due at %s", slots, name, at)
	})
}

// theJobLastFailedWith asserts the failure the job's last run recorded holds the text.
func theJobLastFailedWith(ctx context.Context, name, text string) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	return eventually(func() (bool, string) {
		failed, _ := cw.store.LastFailure(ctx, name)
		return strings.Contains(failed, text), fmt.Sprintf("the job %q last recorded %q, want %q in it", name, failed, text)
	})
}

// theJobIsCutShortAtItsTimeLimit asserts the job's run ended although it would never end alone.
func theJobIsCutShortAtItsTimeLimit(ctx context.Context, name string) error {
	return theJobHasRun(ctx, name, 1)
}

// anotherServerHoldsTheJob takes a lease on the job as another server would.
func anotherServerHoldsTheJob(ctx context.Context, name string) error {
	_, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	now := cw.clock.Now()
	held := cron.Lease{Name: name, Token: "elsewhere", Taken: now, Until: now.Add(time.Minute)}
	_, ok, err := cw.store.Take(ctx, held)
	if err != nil || !ok {
		return fmt.Errorf("another server could not take %q: %v, %w", name, ok, err)
	}
	return nil
}

// theHostsCronCalls posts to the cron address with the authorization header.
func theHostsCronCalls(ctx context.Context, authorization string) error {
	w, _, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.running(); err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, w.site.URL+"/api/cron", nil)
	if err != nil {
		return fmt.Errorf("building the cron call: %w", err)
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	got, err := w.answerFrom(w.browserOf(), request)
	if err != nil {
		return err
	}
	w.answer = got
	return nil
}

// theHostsCronCallsWithTheSecret calls the cron address holding the secret.
func theHostsCronCallsWithTheSecret(ctx context.Context) error {
	return theHostsCronCalls(ctx, "Bearer "+cronSecret)
}

// theHostsCronCallsWithAWrongSecret calls the cron address holding another secret of the same length.
func theHostsCronCallsWithAWrongSecret(ctx context.Context) error {
	return theHostsCronCalls(ctx, "Bearer "+strings.Repeat("f", len(cronSecret)))
}

// theHostsCronCallsWithNoSecret calls the cron address holding no secret.
func theHostsCronCallsWithNoSecret(ctx context.Context) error {
	return theHostsCronCalls(ctx, "")
}

// theCallAnswersThatJobsRan asserts the cron call answered with how many jobs ran.
func theCallAnswersThatJobsRan(ctx context.Context, want int) error {
	w, _, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	var counts struct {
		Ran int `json:"ran"`
	}
	if w.answer.status != http.StatusOK {
		return fmt.Errorf("the call answered %d %s, want 200", w.answer.status, w.answer.body)
	}
	if err := w.answer.decode(&counts); err != nil || counts.Ran != want {
		return fmt.Errorf("the call answered %s (%v), want %d jobs ran", w.answer.body, err, want)
	}
	return nil
}

// theCallIsAnsweredLikeAMissingPage asserts the cron call was answered as an unknown address is.
func theCallIsAnsweredLikeAMissingPage(ctx context.Context) error {
	w, _, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	if w.answer.status != http.StatusNotFound || w.answer.errorCode() != "route_not_found" {
		return fmt.Errorf("the call answered %d %s, want 404 route_not_found", w.answer.status, w.answer.body)
	}
	return nil
}

// theSiteRunsWith serves the site again after change sets its trigger settings.
func theSiteRunsWith(ctx context.Context, change func(*cronWorld)) error {
	w, cw, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	change(cw)
	return w.serveAgain()
}

// theSiteRunsWithoutACronSecret serves the site again with no cron secret.
func theSiteRunsWithoutACronSecret(ctx context.Context) error {
	return theSiteRunsWith(ctx, func(cw *cronWorld) { cw.secret = "" })
}

// theSiteRunsWithVisitsTurnedOff serves the site again with visits starting no cron jobs.
func theSiteRunsWithVisitsTurnedOff(ctx context.Context) error {
	return theSiteRunsWith(ctx, func(cw *cronWorld) { cw.onVisits = false })
}

// theAnswerIsKeptOutOfEveryCache asserts the answer forbids every cache from keeping it.
func theAnswerIsKeptOutOfEveryCache(ctx context.Context) error {
	w, _, err := cronWorldOf(ctx)
	if err != nil {
		return err
	}
	if got := w.answer.header.Get("Cache-Control"); got != "no-store" {
		return fmt.Errorf("Cache-Control = %q, want no-store", got)
	}
	return nil
}

// initializeCron binds the steps the cron feature uses.
func initializeCron(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^a running Gophenberg with the cron job "([^"]*)" every hour and the cron secret set$`,
		aRunningGophenbergWithTheCronJob)
	sc.Given(`^the cron job "([^"]*)" every hour that panics$`, theCronJobEveryHourThatPanics)
	sc.Given(`^the job "([^"]*)" waits until it is let go$`, theJobWaitsUntilItIsLetGo)
	sc.Given(`^the job "([^"]*)" fails with "([^"]*)"$`, theJobFailsWith)
	sc.Given(`^the job "([^"]*)" runs until it is told to stop$`, theJobRunsUntilItIsToldToStop)
	sc.Given(`^a visitor opened the home page$`, aVisitorOpenedTheHomePage)
	sc.Given(`^a visitor opened the home page at "([^"]*)"$`, aVisitorOpenedTheHomePageAt)
	sc.Given(`^another server holds the job "([^"]*)"$`, anotherServerHoldsTheJob)
	sc.Given(`^the site runs without a cron secret$`, theSiteRunsWithoutACronSecret)
	sc.Given(`^the site runs with visits turned off$`, theSiteRunsWithVisitsTurnedOff)
	sc.When(`^a visitor opens the home page$`, aVisitorOpensTheHomePage)
	sc.When(`^two visitors open the home page at the same moment$`, twoVisitorsOpenTheHomePageAtTheSameMoment)
	sc.When(`^(\d+) (seconds?|minutes?) pass(?:es)?$`, timePasses)
	sc.When(`^the clock reads "([^"]*)"$`, theClockReads)
	sc.When(`^the host's cron calls the cron address with the secret$`, theHostsCronCallsWithTheSecret)
	sc.When(`^the host's cron calls the cron address with a wrong secret$`, theHostsCronCallsWithAWrongSecret)
	sc.When(`^the host's cron calls the cron address with no secret$`, theHostsCronCallsWithNoSecret)
	sc.Then(`^the job "([^"]*)" has run (\d+) times?$`, theJobHasRun)
	sc.Then(`^the visitor got the page$`, theVisitorGotThePage)
	sc.Then(`^the job "([^"]*)" is still running$`, theJobIsStillRunning)
	sc.Then(`^the job "([^"]*)" finishes once it is let go$`, theJobFinishesOnceItIsLetGo)
	sc.Then(`^the site checked for cron jobs (\d+) times?$`, theSiteCheckedForCronJobs)
	sc.Then(`^the job "([^"]*)" falls due next at "([^"]*)"$`, theJobFallsDueNextAt)
	sc.Then(`^the job "([^"]*)" last failed with "([^"]*)"$`, theJobLastFailedWith)
	sc.Then(`^the job "([^"]*)" is cut short at its time limit$`, theJobIsCutShortAtItsTimeLimit)
	sc.Then(`^the call answers that (\d+) jobs? ran$`, theCallAnswersThatJobsRan)
	sc.Then(`^the call is answered like a missing page$`, theCallIsAnsweredLikeAMissingPage)
	sc.Then(`^the answer is kept out of every cache$`, theAnswerIsKeptOutOfEveryCache)
}
