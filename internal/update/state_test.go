// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"regexp"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
)

// errPlanted is the failure a test store answers with.
var errPlanted = errors.New("planted failure")

// memoryStore is a site setting store in memory that honours its context and fails a lookup or a save when told to.
type memoryStore struct {
	values      map[string]string
	lookupFail  error
	failKey     string
	saveFail    error
	serialsRead int
	meanwhile   func(m *memoryStore)
}

// Lookup returns the value stored under key, running meanwhile before the second read of the serials.
func (m *memoryStore) Lookup(ctx context.Context, key string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	if m.lookupFail != nil && (m.failKey == "" || m.failKey == key) {
		return "", false, m.lookupFail
	}
	if key == "update_serials" {
		m.serialsRead++
		if m.serialsRead == 2 && m.meanwhile != nil {
			m.meanwhile(m)
		}
	}
	value, found := m.values[key]
	return value, found, nil
}

// Save stores every given value, or none when told to fail or when the context ended.
func (m *memoryStore) Save(ctx context.Context, values map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.saveFail != nil {
		return m.saveFail
	}
	maps.Copy(m.values, values)
	return nil
}

// newStore returns an empty memory store.
func newStore() *memoryStore {
	return &memoryStore{values: map[string]string{}}
}

// storedSerials returns the serials the store holds for each site and channel.
func storedSerials(t *testing.T, store *memoryStore) map[string]update.Serials {
	t.Helper()
	held := map[string]update.Serials{}
	if err := json.Unmarshal([]byte(store.values["update_serials"]), &held); err != nil {
		t.Fatalf("reading update_serials %q: %v", store.values["update_serials"], err)
	}
	return held
}

// source returns the key the serials of the server's channel are stored under.
func (s *updateServer) source(channel string) string {
	return s.server.URL + s.base + " " + channel
}

func TestRunStoresTheOfferTheSerialsAndASiteIdentity(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	store := newStore()

	result, err := update.Run(t.Context(), server.config(k), store)

	if err != nil || result.Offer == nil || result.Offer.Version != "0.21.0" || !result.CheckedAt.Equal(now) {
		t.Fatalf("Run() = %+v, %v, want 0.21.0 found now", result, err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(store.values["site_id"]) {
		t.Errorf("site_id = %q, want 16 random bytes in hexadecimal", store.values["site_id"])
	}
	want := map[string]update.Serials{server.source("stable"): {Keys: 5, Index: 9}}
	if got := storedSerials(t, store); !maps.Equal(got, want) {
		t.Errorf("update_serials = %v, want %v", got, want)
	}
	stored, err := update.ResultOf(t.Context(), store)
	if err != nil || stored.Offer == nil || stored.Offer.Version != "0.21.0" || stored.FailedAt != nil {
		t.Errorf("ResultOf() = %+v, %v, want the result Run stored", stored, err)
	}
}

func TestRunKeepsTheSerialsOfEachSiteAndChannelApart(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	beta := index(offer("0.22.0-rc.1", "0.22.0-rc.1"))
	beta.Channel, beta.Serial = "beta", 3
	server.putIndex(k, beta)
	store := newStore()
	if _, err := update.Run(t.Context(), server.config(k), store); err != nil {
		t.Fatalf("Run() on stable = %v, want nil", err)
	}
	moved := server.config(k)
	moved.Channel = "beta"

	result, err := update.Run(t.Context(), moved, store)

	if err != nil || result.Offer == nil || result.Offer.Version != "0.22.0-rc.1" {
		t.Errorf("Run() on beta after stable = %+v, %v, want the older beta index taken", result, err)
	}
	want := map[string]update.Serials{
		server.source("stable"): {Keys: 5, Index: 9}, server.source("beta"): {Keys: 5, Index: 3},
	}
	if got := storedSerials(t, store); !maps.Equal(got, want) {
		t.Errorf("update_serials = %v, want %v", got, want)
	}
}

func TestRunKeepsANewerCheckThatFinishedFirst(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	newer := update.Result{CheckedAt: &now, Offer: &update.Found{Version: "0.22.0"}}
	store := newStore()
	newerSerials := map[string]update.Serials{server.source("stable"): {Keys: 5, Index: 10}}
	store.meanwhile = func(m *memoryStore) {
		m.values["update_serials"] = string(marshal(t, newerSerials))
		m.values["update_result"] = string(marshal(t, newer))
	}

	result, err := update.Run(t.Context(), server.config(k), store)

	if err != nil || result.Offer == nil || result.Offer.Version != "0.22.0" {
		t.Errorf("Run() = %+v, %v, want the newer check kept", result, err)
	}
	if got := storedSerials(t, store)[server.source("stable")]; got != (update.Serials{Keys: 5, Index: 10}) {
		t.Errorf("update_serials = %+v, want the newer serials kept", got)
	}
}

func TestRunKeepsTheSiteIdentity(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	store := newStore()
	store.values["site_id"] = siteID

	if _, err := update.Run(t.Context(), server.config(k), store); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if store.values["site_id"] != siteID {
		t.Errorf("site_id = %q after a check, want %q kept", store.values["site_id"], siteID)
	}
}

func TestRunKeepsTheLastGoodResultBesideAFailure(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	store := newStore()
	if _, err := update.Run(t.Context(), server.config(k), store); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	server.drop(update.IndexPath("stable"))
	later := server.config(k)
	later.Now = func() time.Time { return now.Add(time.Hour) }

	result, err := update.Run(t.Context(), later, store)

	if err == nil || result.Offer == nil || result.Offer.Version != "0.21.0" || !result.CheckedAt.Equal(now) {
		t.Errorf("Run() = %+v, %v, want the failure beside the last good result", result, err)
	}
	if result.FailedAt == nil || !result.FailedAt.Equal(now.Add(time.Hour)) || result.Error != err.Error() {
		t.Errorf("Run() = %+v, want the failure and its time stored", result)
	}
	if got := storedSerials(t, store)[server.source("stable")]; got != (update.Serials{Keys: 5, Index: 9}) {
		t.Errorf("update_serials = %+v after a failure, want the last good serials kept", got)
	}
}

func TestRunStoresAFailureAfterItsContextEnded(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	server.delay = time.Second
	store := newStore()
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	_, err := update.Run(ctx, server.config(k), store)

	stored, readErr := update.ResultOf(t.Context(), store)
	if err == nil || readErr != nil || stored.FailedAt == nil || stored.Error == "" {
		t.Errorf("Run() = %v, stored %+v, %v, want the failure stored after the context ended", err, stored, readErr)
	}
}

func TestRunClearsAFailureOnTheNextGoodCheck(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	store := newStore()
	store.values["update_result"] = string(marshal(t, update.Result{FailedAt: &now, Error: "update: fetch failed"}))

	result, err := update.Run(t.Context(), server.config(k), store)

	if err != nil || result.FailedAt != nil || result.Error != "" || result.Offer != nil {
		t.Errorf("Run() = %+v, %v, want a good check that found nothing and no failure", result, err)
	}
}

func TestRunStartsAgainFromStateItCannotRead(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	store := newStore()
	store.values["update_serials"] = "not json"
	store.values["update_result"] = "not json"

	result, err := update.Run(t.Context(), server.config(k), store)

	if err != nil || result.Offer == nil {
		t.Errorf("Run() = %+v, %v, want a fresh check", result, err)
	}
}

func TestRunReportsAStoreItCannotUse(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	for name, store := range map[string]*memoryStore{
		"a lookup":               {values: map[string]string{}, lookupFail: errPlanted},
		"the lookup of a serial": {values: map[string]string{}, lookupFail: errPlanted, failKey: "update_serials"},
		"the lookup of a result": {values: map[string]string{}, lookupFail: errPlanted, failKey: "update_result"},
		"a save":                 {values: map[string]string{}, saveFail: errPlanted},
	} {
		if _, err := update.Run(t.Context(), server.config(k), store); !errors.Is(err, errPlanted) {
			t.Errorf("Run() with %s failing = %v, want the store's failure", name, err)
		}
	}
}

func TestRunMarksAResultWhoseStateItCouldNotReadAgain(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	store := newStore()
	store.meanwhile = func(m *memoryStore) {
		m.lookupFail, m.failKey = errPlanted, "update_result"
	}

	_, err := update.Run(t.Context(), server.config(k), store)

	if !errors.Is(err, update.ErrNotStored) || !errors.Is(err, errPlanted) {
		t.Errorf("Run() = %v, want the read before the save marked as not stored", err)
	}
}

func TestRunMarksAResultItCouldNotStore(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	server.drop(update.KeyListPath)
	store := &memoryStore{values: map[string]string{"site_id": siteID}, saveFail: errPlanted}

	result, err := update.Run(t.Context(), server.config(k), store)

	if !errors.Is(err, update.ErrNotStored) || !errors.Is(err, errPlanted) || result.FailedAt == nil {
		t.Errorf("Run() = %+v, %v, want the failed check marked as not stored", result, err)
	}
}

func TestResultOfIsEmptyBeforeTheFirstCheckAndRefusesStateItCannotRead(t *testing.T) {
	t.Parallel()

	empty, err := update.ResultOf(t.Context(), newStore())
	if err != nil || empty != (update.Result{}) {
		t.Errorf("ResultOf() on an empty store = %+v, %v, want an empty result", empty, err)
	}
	broken := newStore()
	broken.values["update_result"] = "not json"
	if _, err := update.ResultOf(t.Context(), broken); err == nil {
		t.Error("ResultOf() of a result that is not JSON = nil, want it refused")
	}
	if _, err := update.ResultOf(t.Context(), &memoryStore{lookupFail: errPlanted}); !errors.Is(err, errPlanted) {
		t.Errorf("ResultOf() with the lookup failing = %v, want the store's failure", err)
	}
}

func TestResultIsWrittenWithEveryField(t *testing.T) {
	t.Parallel()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(marshal(t, update.Result{}), &fields); err != nil || len(fields) != 4 {
		t.Errorf("an empty result writes %v, %v, want checked_at, offer, failed_at and error", fields, err)
	}
}
