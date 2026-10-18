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

// memoryStore is a site setting store in memory that fails a lookup, of one key or of all, or a save when told to.
type memoryStore struct {
	values     map[string]string
	lookupFail error
	failKey    string
	saveFail   error
}

// Lookup returns the value stored under key and whether the key is set at all.
func (m *memoryStore) Lookup(_ context.Context, key string) (string, bool, error) {
	if m.lookupFail != nil && (m.failKey == "" || m.failKey == key) {
		return "", false, m.lookupFail
	}
	value, found := m.values[key]
	return value, found, nil
}

// Save stores every given value, or none when told to fail.
func (m *memoryStore) Save(_ context.Context, values map[string]string) error {
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
	if store.values["update_serials"] != `{"keys":5,"index":9}` {
		t.Errorf("update_serials = %s, want the serials taken", store.values["update_serials"])
	}
	stored, err := update.ResultOf(t.Context(), store)
	if err != nil || stored.Offer == nil || stored.Offer.Version != "0.21.0" || stored.FailedAt != nil {
		t.Errorf("ResultOf() = %+v, %v, want the result Run stored", stored, err)
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
	if store.values["update_serials"] != `{"keys":5,"index":9}` {
		t.Errorf("update_serials = %s after a failure, want the last good serials kept", store.values["update_serials"])
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
	withIdentity := &memoryStore{values: map[string]string{"site_id": siteID}, saveFail: errPlanted}
	if _, err := update.Run(t.Context(), server.config(k), withIdentity); !errors.Is(err, errPlanted) {
		t.Errorf("Run() with the result's save failing = %v, want the store's failure", err)
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
