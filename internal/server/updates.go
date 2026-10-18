// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/update"
)

// UpdateChecks is what the update routes ask of the site: what the checks stored, and a check run now.
type UpdateChecks interface {
	Result(ctx context.Context) (update.Result, error)
	Check(ctx context.Context) (update.Result, error)
}

// handleUpdatesGet answers what the checks for a newer version stored.
func (s *server) handleUpdatesGet(updates UpdateChecks) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := updates.Result(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, result)
	}
}

// handleUpdatesCheck runs a check now and answers its result, refusing one within age of the last.
func (s *server) handleUpdatesCheck(updates UpdateChecks, age time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		last, err := updates.Result(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if wait := age - time.Since(lastCheck(last)); wait > 0 {
			seconds := int(math.Ceil(wait.Seconds()))
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			authkit.RespondError(w, http.StatusTooManyRequests, authkit.ErrorResponse{
				Message: "the last check is too recent, so try again later", Code: "check_too_soon",
				Meta: map[string]any{"retry_after": seconds},
			})
			return
		}
		result, err := updates.Check(r.Context())
		if err != nil && result.FailedAt == nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, result)
	}
}

// lastCheck returns when the last check ended, a success or a failure, the zero time before any.
func lastCheck(result update.Result) time.Time {
	var last time.Time
	for _, at := range []*time.Time{result.CheckedAt, result.FailedAt} {
		if at != nil && at.After(last) {
			last = *at
		}
	}
	return last
}
