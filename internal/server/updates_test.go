// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/update"
)

// errUpdates is the failure a test's update checks answer with.
var errUpdates = errors.New("update store failure")

// storedUpdates answers the update routes from the result it holds, counting the checks it runs.
type storedUpdates struct {
	mu        sync.Mutex
	result    update.Result
	resultErr error
	checked   update.Result
	checkErr  error
	checks    int
}

// Result returns the result the test stored.
func (s *storedUpdates) Result(context.Context) (update.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result, s.resultErr
}

// Check counts the check and returns what the test said it finds.
func (s *storedUpdates) Check(context.Context) (update.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks++
	return s.checked, s.checkErr
}

// updatesServer returns a server whose update routes held answers a minute apart at most, and an admin cookie.
func updatesServer(t *testing.T, held *storedUpdates) (http.Handler, *http.Cookie) {
	t.Helper()
	users := newFakeUserStore()
	addWithRole(t, users, "admin@example.com", "Maria Perez", role.Admin)
	handler := server.NewServer(server.Config{Users: users, Updates: held, UpdatesCheckAge: time.Minute})
	recorder := doLogin(t, handler, `{"email":"admin@example.com","password":"correct horse battery"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recorder.Code, http.StatusOK)
	}
	return handler, sessionCookie(t, recorder)
}

// answeredResult returns the result a reply carries.
func answeredResult(t *testing.T, recorder *httptest.ResponseRecorder) update.Result {
	t.Helper()
	var result update.Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("reading the result %q: %v", recorder.Body.String(), err)
	}
	return result
}

// at returns a pointer to the moment ago before now.
func at(ago time.Duration) *time.Time {
	moment := time.Now().Add(-ago).UTC()
	return &moment
}

func TestUpdatesGetAnswersTheStoredResult(t *testing.T) {
	t.Parallel()

	held := &storedUpdates{result: update.Result{CheckedAt: at(time.Hour), Offer: &update.Found{Version: "0.21.0"}}}
	handler, cookie := updatesServer(t, held)

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/updates", ""})

	if recorder.Code != http.StatusOK || answeredResult(t, recorder).Offer.Version != "0.21.0" {
		t.Errorf("GET /api/updates = %d %s, want 200 and the stored offer", recorder.Code, recorder.Body.String())
	}
}

func TestUpdatesGetMasksAStoreItCannotRead(t *testing.T) {
	t.Parallel()

	handler, cookie := updatesServer(t, &storedUpdates{resultErr: errUpdates})

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/updates", ""})

	if recorder.Code != http.StatusInternalServerError || refusalOf(t, recorder).Code != "internal" {
		t.Errorf("GET /api/updates = %d %s, want 500 and the internal code", recorder.Code, recorder.Body.String())
	}
}

func TestUpdatesCheckRunsACheckWhenTheLastIsOldEnough(t *testing.T) {
	t.Parallel()

	for name, last := range map[string]update.Result{
		"before any check":     {},
		"after an old check":   {CheckedAt: at(time.Hour)},
		"after an old failure": {CheckedAt: at(2 * time.Hour), FailedAt: at(time.Hour), Error: "update: fetch failed"},
	} {
		newer := update.Result{CheckedAt: at(0), Offer: &update.Found{Version: "0.22.0"}}
		held := &storedUpdates{result: last, checked: newer}
		handler, cookie := updatesServer(t, held)

		recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/check", ""})

		if recorder.Code != http.StatusOK || held.checks != 1 || answeredResult(t, recorder).Offer.Version != "0.22.0" {
			t.Errorf("POST /api/updates/check %s = %d %s after %d checks, want 200 and one check", name, recorder.Code,
				recorder.Body.String(), held.checks)
		}
	}
}

func TestUpdatesCheckRefusesACheckSoonAfterTheLast(t *testing.T) {
	t.Parallel()

	for name, last := range map[string]update.Result{
		"after a check":   {CheckedAt: at(10 * time.Second)},
		"after a failure": {CheckedAt: at(time.Hour), FailedAt: at(10 * time.Second), Error: "update: fetch failed"},
	} {
		held := &storedUpdates{result: last}
		handler, cookie := updatesServer(t, held)

		recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/check", ""})

		refused := recorder.Code == http.StatusTooManyRequests && refusalOf(t, recorder).Code == "check_too_soon"
		if !refused || held.checks != 0 {
			t.Errorf("POST /api/updates/check %s = %d %s, want 429 and check_too_soon with no check", name,
				recorder.Code, recorder.Body.String())
		}
		if wait, err := strconv.Atoi(recorder.Header().Get("Retry-After")); err != nil || wait < 1 || wait > 50 {
			t.Errorf("Retry-After = %q %s, want the seconds left, at most 50", recorder.Header().Get("Retry-After"), name)
		}
	}
}

func TestUpdatesCheckAnswersAFailedCheckWithItsStoredResult(t *testing.T) {
	t.Parallel()

	failed := update.Result{FailedAt: at(0), Error: "update: refused the index, which expired at 2026-10-01T00:00:00Z"}
	held := &storedUpdates{checked: failed, checkErr: errors.New(failed.Error)}
	handler, cookie := updatesServer(t, held)

	recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/check", ""})

	if recorder.Code != http.StatusOK || answeredResult(t, recorder).Error != failed.Error {
		t.Errorf("POST /api/updates/check = %d %s, want 200 and the failure beside the last result", recorder.Code,
			recorder.Body.String())
	}
}

func TestUpdatesCheckMasksAStoreItCannotUse(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]*storedUpdates{
		"a result it cannot read": {resultErr: errUpdates},
		"a check it cannot store": {checkErr: errUpdates},
	} {
		handler, cookie := updatesServer(t, held)

		recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/check", ""})

		if recorder.Code != http.StatusInternalServerError || refusalOf(t, recorder).Code != "internal" {
			t.Errorf("POST /api/updates/check with %s = %d %s, want 500 and the internal code", name, recorder.Code,
				recorder.Body.String())
		}
	}
}
