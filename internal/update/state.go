// SPDX-License-Identifier: Apache-2.0

package update

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	// siteIDKey is the site setting that holds the site's identity, which only the rollout bucket reads.
	siteIDKey = "site_id"
	// serialsKey is the site setting that holds the serials of the last key list and index taken.
	serialsKey = "update_serials"
	// resultKey is the site setting that holds what the checks found.
	resultKey = "update_result"
	// siteIDSize is how many random bytes a site's identity holds.
	siteIDSize = 16
)

// Settings is the site setting store a check keeps its state in.
type Settings interface {
	Lookup(ctx context.Context, key string) (string, bool, error)
	Save(ctx context.Context, values map[string]string) error
}

// Result is what the checks found: the last good check, and the last failure since it.
type Result struct {
	CheckedAt *time.Time `json:"checked_at"`
	Offer     *Found     `json:"offer"`
	FailedAt  *time.Time `json:"failed_at"`
	Error     string     `json:"error"`
}

// Run checks the update site for the site, keeping its identity, the serials taken and the result in store.
func Run(ctx context.Context, cfg Config, store Settings) (Result, error) {
	id, err := siteID(ctx, store)
	if err != nil {
		return Result{}, err
	}
	taken, previous, err := stateOf(ctx, store)
	if err != nil {
		return Result{}, err
	}
	cfg.Site.ID = id
	found, serials, checked := Check(ctx, cfg, taken)
	result := previous.after(found, checked, cfg.Now())
	values := map[string]string{resultKey: written(result)}
	if checked == nil {
		values[serialsKey] = written(serials)
	}
	if err := store.Save(ctx, values); err != nil {
		return result, errors.Join(fmt.Errorf("update: store the result: %w", err), checked)
	}
	return result, checked
}

// ResultOf returns what the checks stored in store, an empty result before the first check.
func ResultOf(ctx context.Context, store Settings) (Result, error) {
	held, found, err := store.Lookup(ctx, resultKey)
	if err != nil || !found {
		return Result{}, err
	}
	var result Result
	if err := json.Unmarshal([]byte(held), &result); err != nil {
		return Result{}, fmt.Errorf("update: read the stored result: %w", err)
	}
	return result, nil
}

// after returns the result once a check at the time at found an offer or failed with err.
func (r Result) after(found *Found, err error, at time.Time) Result {
	if err != nil {
		r.FailedAt, r.Error = &at, err.Error()
		return r
	}
	return Result{CheckedAt: &at, Offer: found}
}

// siteID returns the site's identity, making and storing one the first time.
func siteID(ctx context.Context, store Settings) (string, error) {
	held, found, err := store.Lookup(ctx, siteIDKey)
	if err != nil || found {
		return held, err
	}
	raw := make([]byte, siteIDSize)
	_, _ = rand.Read(raw)
	id := hex.EncodeToString(raw)
	return id, store.Save(ctx, map[string]string{siteIDKey: id})
}

// stateOf returns the serials taken and the result stored, starting again from any it cannot read.
func stateOf(ctx context.Context, store Settings) (Serials, Result, error) {
	var serials Serials
	var result Result
	for key, into := range map[string]any{serialsKey: &serials, resultKey: &result} {
		held, found, err := store.Lookup(ctx, key)
		if err != nil {
			return Serials{}, Result{}, err
		}
		if found {
			_ = json.Unmarshal([]byte(held), into)
		}
	}
	return serials, result, nil
}

// written returns v as the JSON a setting holds.
func written(v any) string {
	body, _ := json.Marshal(v)
	return string(body)
}
