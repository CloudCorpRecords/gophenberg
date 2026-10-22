// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/update"
)

// wantRefusedVersion fails t unless err refuses version as one the site refused.
func wantRefusedVersion(t *testing.T, err error, version string) {
	t.Helper()
	var refused update.RefusedError
	if !errors.As(err, &refused) || refused.Version != version {
		t.Errorf("error = %v, want %s refused as a version that failed before", err, version)
	}
}

func TestChooseTakesTheNewestOfferWithItsReleaseRecordAndSigners(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	newest := offer("0.22.0", "0.21.0")
	newest.Release = archives(t, "0.22.0")
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"), newest)

	chosen, err := update.Choose(t.Context(), server.config(k), newStore(), "")

	if err != nil || chosen.Found.Version != "0.22.0" || !chosen.Found.LongJump {
		t.Fatalf("Choose() = %+v, %v, want 0.22.0 taken as a long jump from 0.20.0", chosen.Found, err)
	}
	if listed := chosen.Release.Artifacts.Package; len(listed) != 2 || listed[1].ID != "linux-arm64" {
		t.Errorf("Choose() carried the archives %+v, want the two the release record lists", listed)
	}
	if len(chosen.Signers) != 2 || !chosen.Signers[0].Equal(k.release.Public()) ||
		!chosen.Signers[1].Equal(k.standby.Public()) {
		t.Errorf("Choose() carried the signers %v, want the release key and the standby", chosen.Signers)
	}
}

func TestChooseTakesTheVersionNamed(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.20.0"), offer("0.22.0", "0.22.0"))

	chosen, err := update.Choose(t.Context(), server.config(k), newStore(), "0.21.0")

	if err != nil || chosen.Found.Version != "0.21.0" || chosen.Release.Version != "0.21.0" || chosen.Found.LongJump {
		t.Errorf("Choose(0.21.0) = %+v, %v, want 0.21.0 taken as a short step", chosen, err)
	}
}

func TestChooseTakesANamedVersionAPartialRolloutHasNotReached(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	partial := offer("0.21.0", "0.21.0")
	partial.Rollout = 10
	if bucket := update.Bucket(siteID, "0.21.0"); bucket < partial.Rollout {
		t.Fatalf("Bucket(siteID, 0.21.0) = %d, want a bucket the rollout of %d does not reach", bucket, partial.Rollout)
	}
	server := newUpdateServer(t, k, "", partial)
	store := newStore()
	store.values["site_id"] = siteID

	chosen, err := update.Choose(t.Context(), server.config(k), store, "0.21.0")
	if err != nil || chosen.Found.Version != "0.21.0" {
		t.Errorf("Choose(0.21.0) = %+v, %v, want the version a person named taken", chosen.Found, err)
	}
	if _, err := update.Choose(t.Context(), server.config(k), store, ""); !errors.Is(err, update.ErrNoUpdate) {
		t.Errorf("Choose() = %v, want no update, since the rollout does not reach this site", err)
	}
}

func TestChooseRefusesANamedVersionTheUpdateSiteHoldsBack(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	held := offer("0.21.0", "0.21.0")
	held.Rollout = 0
	server := newUpdateServer(t, k, "", held)

	_, err := update.Choose(t.Context(), server.config(k), newStore(), "0.21.0")

	wantRefused(t, err, "refused Gophenberg 0.21.0, which the update site holds back from every site for now")
}

func TestChooseRefusesANamedVersionItCannotTake(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	withdrawn := offer("0.22.0", "0.22.0")
	withdrawn.Withdrawn = true
	server := newUpdateServer(t, k, "",
		offer("0.19.0", "0.19.0"), offer("0.20.0", "0.20.0"), offer("0.21.0", "0.21.0"), withdrawn)
	config := server.config(k)
	config.Site.Refused = []string{"0.21.0"}
	for named, text := range map[string]string{
		"0.23.0": "the update site offers no Gophenberg 0.23.0",
		"0.22.0": "refused Gophenberg 0.22.0, which the update site withdrew",
		"0.20.0": "refused Gophenberg 0.20.0, which is not newer than 0.20.0, the version this site runs",
		"0.19.0": "refused Gophenberg 0.19.0, which is not newer than 0.20.0, the version this site runs",
	} {
		_, err := update.Choose(t.Context(), config, newStore(), named)
		wantRefused(t, err, text)
	}
	_, err := update.Choose(t.Context(), config, newStore(), "0.21.0")
	wantRefusedVersion(t, err, "0.21.0")
}

func TestChooseNeverPicksARefusedVersionOnItsOwn(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"), offer("0.22.0", "0.22.0"))
	config := server.config(k)
	config.Site.Refused = []string{"0.22.0"}

	chosen, err := update.Choose(t.Context(), config, newStore(), "")
	if err != nil || chosen.Found.Version != "0.21.0" {
		t.Errorf("Choose() = %+v, %v, want 0.21.0, since the site refused 0.22.0", chosen.Found, err)
	}

	config.Site.Refused = []string{"0.21.0", "0.22.0"}
	_, err = update.Choose(t.Context(), config, newStore(), "")
	wantRefusedVersion(t, err, "0.22.0")
}

func TestChooseFindsNothingWhenNoOfferIsNewer(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.20.0", "0.20.0"))

	_, err := update.Choose(t.Context(), server.config(k), newStore(), "")

	if !errors.Is(err, update.ErrNoUpdate) {
		t.Errorf("Choose() = %v, want no update", err)
	}
}

func TestChooseRefusesAnIndexOlderThanTheOneACheckTook(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	store := newStore()
	if _, err := update.Run(t.Context(), server.config(k), store); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	older := index(offer("0.21.0", "0.21.0"))
	older.Serial = 8
	server.putIndex(k, older)

	_, err := update.Choose(t.Context(), server.config(k), store, "")

	wantRefused(t, err, "index of serial 8, older than serial 9 taken before")
}

func TestChooseRefusesAnOlderIndexOnceItSawANewerOneWithdrawTheVersion(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	for named, text := range map[string]string{
		"":       "offers no version newer than the one this site runs",
		"0.21.0": "refused Gophenberg 0.21.0, which the update site withdrew",
	} {
		withdrawn := offer("0.21.0", "0.21.0")
		withdrawn.Withdrawn = true
		newer := index(withdrawn)
		newer.Serial = 10
		server := newUpdateServer(t, k, "")
		server.putIndex(k, newer)
		store := newStore()
		_, err := update.Choose(t.Context(), server.config(k), store, named)
		wantRefused(t, err, text)
		server.putIndex(k, index(offer("0.21.0", "0.21.0")))

		_, err = update.Choose(t.Context(), server.config(k), store, named)

		wantRefused(t, err, "index of serial 9, older than serial 10 taken before")
	}
}

func TestChooseRaisesTheStoredSerialsWithoutLoweringAny(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	store := newStore()
	store.meanwhile = func(m *memoryStore) {
		m.values["update_serials"] = string(marshal(t, takenSerials{
			Keys:  map[string]int64{server.site(): 6},
			Index: map[string]int64{server.source("stable"): 3, server.source("beta"): 4},
		}))
	}

	if _, err := update.Choose(t.Context(), server.config(k), store, ""); err != nil {
		t.Fatalf("Choose() = %v, want nil", err)
	}
	for channel, want := range map[string]update.Serials{"stable": {Keys: 6, Index: 9}, "beta": {Keys: 6, Index: 4}} {
		if got := storedSerials(t, store, server, channel); got != want {
			t.Errorf("update_serials of %s = %+v, want %+v", channel, got, want)
		}
	}
}

func TestChooseStopsWhenItCannotStoreTheSerials(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	store := &memoryStore{values: map[string]string{"site_id": siteID}, saveFail: errPlanted}

	_, err := update.Choose(t.Context(), server.config(k), store, "")

	if !errors.Is(err, errPlanted) || !strings.Contains(err.Error(), "store the serials taken") {
		t.Errorf("Choose() = %v, want the store's failure to keep the serials", err)
	}
}

func TestChooseRefusesAnIndexNoListedKeySigned(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	body := marshal(t, index(offer("0.21.0", "0.21.0")))
	server.put(update.IndexPath("stable")+update.SignatureSuffix, signIndex(body, k.stranger))

	_, err := update.Choose(t.Context(), server.config(k), newStore(), "")

	wantRefused(t, err, "no release or standby key the list names")
}

func TestChooseRefusesAReleaseRecordItCannotRead(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	broken := offer("0.21.0", "0.21.0")
	broken.Release = json.RawMessage(`{"version":"0.21.0","artifacts":{"package":"none"}}`)
	server := newUpdateServer(t, k, "", broken)

	_, err := update.Choose(t.Context(), server.config(k), newStore(), "")

	wantRefused(t, err, "refused the release record of Gophenberg 0.21.0")
}

func TestChooseStopsWhenTheStoreFails(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	for _, key := range []string{"site_id", "update_serials"} {
		store := newStore()
		store.lookupFail, store.failKey = errPlanted, key
		if _, err := update.Choose(t.Context(), server.config(k), store, ""); !errors.Is(err, errPlanted) {
			t.Errorf("Choose() with %s unreadable = %v, want the store's failure", key, err)
		}
	}
}

func TestChooseMarksWhatTheUpdateSiteServesAsNotTakenAndLeavesAStoreThatFailedUnmarked(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	offering := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	unsigned := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	body := marshal(t, index(offer("0.21.0", "0.21.0")))
	unsigned.put(update.IndexPath("stable")+update.SignatureSuffix, signIndex(body, k.stranger))
	current := newUpdateServer(t, k, "", offer("0.20.0", "0.20.0"))
	broken := offer("0.21.0", "0.21.0")
	broken.Release = json.RawMessage(`{"version":"0.21.0","artifacts":{"package":"none"}}`)
	for name, asked := range map[string]struct {
		server *updateServer
		store  *memoryStore
		named  string
		marked bool
	}{
		"an index no listed key signed":            {unsigned, newStore(), "", true},
		"a version the update site does not offer": {offering, newStore(), "0.23.0", true},
		"no version newer than the one it runs":    {current, newStore(), "", true},
		"a release record it cannot read":          {newUpdateServer(t, k, "", broken), newStore(), "", true},
		"a store it cannot read": {
			offering, &memoryStore{values: map[string]string{}, lookupFail: errPlanted, failKey: "site_id"}, "", false,
		},
		"a store that cannot keep the serials": {
			offering, &memoryStore{values: map[string]string{"site_id": siteID}, saveFail: errPlanted}, "", false,
		},
	} {
		_, err := update.Choose(t.Context(), asked.server.config(k), asked.store, asked.named)

		if err == nil || errors.Is(err, update.ErrNotTaken) != asked.marked {
			t.Errorf("Choose() over %s = %v, want it marked as not taken %t", name, err, asked.marked)
		}
	}
}

func TestChooseLetsGoOfItsIdleConnections(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	transport := &closingTransport{Transport: &http.Transport{}}
	config := server.config(k)
	config.Client = &http.Client{Transport: transport}

	if _, err := update.Choose(t.Context(), config, newStore(), ""); err != nil {
		t.Fatalf("Choose() error = %v, want nil", err)
	}
	if transport.closed != 1 {
		t.Errorf("idle connections closed %d times, want once when the choice ended", transport.closed)
	}
}

func TestRefusedErrorSaysTheVersionFailedBefore(t *testing.T) {
	t.Parallel()

	err := update.RefusedError{Version: "0.21.0"}

	wantRefused(t, err, "refused Gophenberg 0.21.0, which failed its start check after an earlier update")
}
