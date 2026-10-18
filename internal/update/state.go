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
	// serialsKey is the site setting that holds the serials taken of each key list and index.
	serialsKey = "update_serials"
	// resultKey is the site setting that holds what the checks found.
	resultKey = "update_result"
	// siteIDSize is how many random bytes a site's identity holds.
	siteIDSize = 16
)

// ErrNotStored marks a check whose result the site setting store could not keep.
var ErrNotStored = errors.New("update: the result of the check could not be stored")

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

// outcome is what one check found or why it failed, and when it ended.
type outcome struct {
	found   *Found
	serials Serials
	err     error
	at      time.Time
}

// taken holds the serials a site took: of each update site's key list, and of each channel's index there.
type taken struct {
	Keys  map[string]int64 `json:"keys"`
	Index map[string]int64 `json:"index"`
}

// source names what a check reads: the key list of an update site, and the index of one of its channels.
type source struct {
	site  string
	index string
}

// Run checks the update site for the site, keeping its identity, the serials taken and the result in store.
func Run(ctx context.Context, cfg Config, store Settings) (Result, error) {
	id, err := siteID(ctx, store)
	if err != nil {
		return Result{}, err
	}
	held, _, err := stateOf(ctx, store)
	if err != nil {
		return Result{}, err
	}
	cfg.Site.ID = id
	src := sourceOf(cfg)
	found, serials, checked := Check(ctx, cfg, held.of(src))
	saving, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.Client.Timeout)
	defer cancel()
	return record(saving, store, src, outcome{found: found, serials: serials, err: checked, at: cfg.Now()})
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

// record stores the outcome of a check of src over the state stored now, keeping a newer check's.
func record(ctx context.Context, store Settings, src source, done outcome) (Result, error) {
	held, stored, err := stateOf(ctx, store)
	if err != nil {
		return Result{}, errors.Join(fmt.Errorf("%w: %w", ErrNotStored, err), done.err)
	}
	if done.err == nil && held.of(src).newerThan(done.serials) {
		return stored, nil
	}
	result := stored.after(done)
	values := map[string]string{resultKey: written(result)}
	if done.err == nil {
		values[serialsKey] = written(held.with(src, done.serials))
	}
	if err := store.Save(ctx, values); err != nil {
		return result, errors.Join(fmt.Errorf("%w: %w", ErrNotStored, err), done.err)
	}
	return result, done.err
}

// newerThan reports whether either serial comes after the matching one of other.
func (s Serials) newerThan(other Serials) bool {
	return s.Keys > other.Keys || s.Index > other.Index
}

// sourceOf returns what a check under cfg reads.
func sourceOf(cfg Config) source {
	site := cfg.Address.String()
	return source{site: site, index: site + " " + cfg.Channel}
}

// of returns the serials taken from src.
func (t taken) of(src source) Serials {
	return Serials{Keys: t.Keys[src.site], Index: t.Index[src.index]}
}

// with returns t holding serials as the ones taken from src.
func (t taken) with(src source, serials Serials) taken {
	keys, index := orEmpty(t.Keys), orEmpty(t.Index)
	keys[src.site], index[src.index] = serials.Keys, serials.Index
	return taken{Keys: keys, Index: index}
}

// orEmpty returns m, or an empty map when m is nil.
func orEmpty(m map[string]int64) map[string]int64 {
	if m == nil {
		return map[string]int64{}
	}
	return m
}

// after returns the result once a check ended as done says.
func (r Result) after(done outcome) Result {
	if done.err != nil {
		r.FailedAt, r.Error = &done.at, done.err.Error()
		return r
	}
	return Result{CheckedAt: &done.at, Offer: done.found}
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
func stateOf(ctx context.Context, store Settings) (taken, Result, error) {
	var serials taken
	var result Result
	for _, read := range []struct {
		key  string
		into any
	}{{serialsKey, &serials}, {resultKey, &result}} {
		held, found, err := store.Lookup(ctx, read.key)
		if err != nil {
			return taken{}, Result{}, err
		}
		if found {
			_ = json.Unmarshal([]byte(held), read.into)
		}
	}
	return serials, result, nil
}

// written returns v as the JSON a setting holds.
func written(v any) string {
	body, _ := json.Marshal(v)
	return string(body)
}
