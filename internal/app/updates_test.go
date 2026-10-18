// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/version"
)

// errMajor is the failure a stubbed read of the server's major answers with.
var errMajor = errors.New("major failure")

// heldSettings is a site setting store in memory.
type heldSettings struct {
	mu     sync.Mutex
	values map[string]string
}

// Lookup returns the value stored under key and whether the key is set at all.
func (h *heldSettings) Lookup(_ context.Context, key string) (string, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	value, found := h.values[key]
	return value, found, nil
}

// Save stores every given value.
func (h *heldSettings) Save(_ context.Context, values map[string]string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	maps.Copy(h.values, values)
	return nil
}

// updatesAt returns the update settings for the site, and the store a check keeps its state in.
func updatesAt(t *testing.T, site *updateSite) (updateSettings, *heldSettings) {
	t.Helper()
	settings, err := updateSettingsFrom(settingsEnv(testkit.Getenv(map[string]string{
		"GOPHENBERG_UPDATE_URL": site.address, "GOPHENBERG_TRUSTED_KEYS": site.trustedRoots(),
	})))
	if err != nil {
		t.Fatalf("updateSettingsFrom() error = %v, want nil", err)
	}
	return settings, &heldSettings{values: map[string]string{}}
}

// offeringSite returns an update site whose stable index offers 99.0.0, signed by both roots.
func offeringSite(t *testing.T) *updateSite {
	t.Helper()
	site := newUpdateSite(t)
	site.offered = "99.0.0"
	site.publishKeyList("the first root", "the second root")
	site.publishIndex(site.release, unchanged)
	return site
}

// major returns a read of the server's major that answers major and err.
func major(major int, err error) func(context.Context) (int, error) {
	return func(context.Context) (int, error) {
		return major, err
	}
}

func TestUpdateSettingsFallBackToTheOfficialSite(t *testing.T) {
	t.Parallel()

	settings, err := updateSettingsFrom(settingsEnv(testkit.Getenv(nil)))

	if err != nil || settings.address.String() != update.DefaultAddress || settings.channel != "stable" ||
		len(settings.roots) != 2 || !settings.checks {
		t.Fatalf("updateSettingsFrom() = %+v, %v, want the official site, stable, its two roots and checks on",
			settings, err)
	}
	if settings.every != 12*time.Hour || settings.screenAge != time.Minute || settings.timeout != 30*time.Second {
		t.Errorf("updateSettingsFrom() times = %v, %v and %v, want 12h, 1m and 30s", settings.every,
			settings.screenAge, settings.timeout)
	}
}

func TestUpdateSettingsTrustMoreRootsOnlyBesideAnotherSite(t *testing.T) {
	t.Parallel()

	site := newUpdateSite(t)
	settings, err := updateSettingsFrom(settingsEnv(testkit.Getenv(map[string]string{
		"GOPHENBERG_UPDATE_URL": "https://mirror.example.com", "GOPHENBERG_TRUSTED_KEYS": site.trustedRoots(),
		"GOPHENBERG_UPDATE_CHANNEL": "beta", "GOPHENBERG_UPDATE_CHECKS": "false",
	})))

	if err != nil || len(settings.roots) != 4 || settings.channel != "beta" || settings.checks {
		t.Errorf("updateSettingsFrom() = %+v, %v, want the built-in roots and the two trusted ones, beta, checks off",
			settings, err)
	}
}

func TestUpdateSettingsRefuseWhatTheyCannotUse(t *testing.T) {
	t.Parallel()

	site := newUpdateSite(t)
	for name, tc := range map[string]struct {
		env  map[string]string
		want string
	}{
		"plain HTTP to another host": {map[string]string{"GOPHENBERG_UPDATE_URL": "http://mirror.example.com"},
			"GOPHENBERG_UPDATE_URL"},
		"trusted roots beside the official site": {map[string]string{"GOPHENBERG_TRUSTED_KEYS": site.trustedRoots()},
			"GOPHENBERG_TRUSTED_KEYS"},
		"a trusted root that is no P-256 key": {map[string]string{
			"GOPHENBERG_UPDATE_URL": "https://mirror.example.com", "GOPHENBERG_TRUSTED_KEYS": "z6Mk"},
			"GOPHENBERG_TRUSTED_KEYS"},
		"a channel that climbs out": {map[string]string{"GOPHENBERG_UPDATE_CHANNEL": "../beta"},
			"GOPHENBERG_UPDATE_CHANNEL"},
		"a channel in capitals":     {map[string]string{"GOPHENBERG_UPDATE_CHANNEL": "Stable"}, "GOPHENBERG_UPDATE_CHANNEL"},
		"an interval of zero":       {map[string]string{"GOPHENBERG_UPDATE_CHECK_INTERVAL": "0s"}, "UPDATE_CHECK_INTERVAL"},
		"a screen age in words":     {map[string]string{"GOPHENBERG_UPDATES_SCREEN_CHECK_AGE": "soon"}, "SCREEN_CHECK_AGE"},
		"a negative timeout":        {map[string]string{"GOPHENBERG_UPDATE_TIMEOUT": "-1s"}, "GOPHENBERG_UPDATE_TIMEOUT"},
		"checks neither on nor off": {map[string]string{"GOPHENBERG_UPDATE_CHECKS": "maybe"}, "GOPHENBERG_UPDATE_CHECKS"},
	} {
		_, err := updateSettingsFrom(settingsEnv(testkit.Getenv(tc.env)))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("updateSettingsFrom() with %s = %v, want a refusal naming %s", name, err, tc.want)
		}
	}
}

func TestLoadRunConfigRefusesUpdateSettingsItCannotUse(t *testing.T) {
	t.Parallel()

	_, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_UPDATE_CHANNEL": "../beta",
	}))

	if err == nil || !strings.Contains(err.Error(), "GOPHENBERG_UPDATE_CHANNEL") {
		t.Errorf("loadRunConfig() = %v, want the update channel refused", err)
	}
}

func TestUpdateJobsHoldTheCheckOnlyWhileChecksAreOn(t *testing.T) {
	t.Parallel()

	updates := siteUpdates{store: &heldSettings{values: map[string]string{}}, major: major(18, nil)}
	on := updateJobs(updateSettings{checks: true, every: time.Hour}, updates)
	off := updateJobs(updateSettings{every: time.Hour}, updates)

	if len(on) != 1 || on[0].Name != updateCheckJob || on[0].Every != time.Hour || len(off) != 0 {
		t.Errorf("updateJobs() = %+v on and %+v off, want the hourly check alone, then nothing", on, off)
	}
}

func TestTheUpdateJobStoresWhatTheSiteOffers(t *testing.T) {
	t.Parallel()

	settings, store := updatesAt(t, offeringSite(t))
	settings.checks, settings.every = true, time.Hour
	job := updateJobs(settings, siteUpdates{settings: settings, store: store, major: major(18, nil)})[0]

	if err := job.Run(t.Context(), time.Now()); err != nil {
		t.Fatalf("the update job = %v, want nil", err)
	}
	result, err := update.ResultOf(t.Context(), store)
	if err != nil || result.Offer == nil || result.Offer.Version != "99.0.0" {
		t.Errorf("the stored result = %+v, %v, want 99.0.0 offered", result, err)
	}
}

func TestSiteUpdatesRunAndAnswerTheChecks(t *testing.T) {
	t.Parallel()

	settings, store := updatesAt(t, offeringSite(t))
	updates := siteUpdates{settings: settings, store: store, major: major(14, nil)}

	checked, err := updates.Check(t.Context())
	if err != nil || checked.Offer == nil || !checked.Offer.NeedsPostgreSQL {
		t.Fatalf("Check() = %+v, %v, want 99.0.0 offered, needing a newer PostgreSQL than 14", checked, err)
	}
	stored, err := updates.Result(t.Context())
	if err != nil || stored.Offer == nil || stored.Offer.Version != "99.0.0" {
		t.Errorf("Result() = %+v, %v, want the result Check stored", stored, err)
	}
}

func TestSiteUpdatesReportAMajorTheyCannotRead(t *testing.T) {
	t.Parallel()

	settings, store := updatesAt(t, offeringSite(t))
	updates := siteUpdates{settings: settings, store: store, major: major(0, errMajor)}

	if result, err := updates.Check(t.Context()); !errors.Is(err, errMajor) || result.FailedAt != nil {
		t.Errorf("Check() = %+v, %v, want the failure to read the major and nothing stored", result, err)
	}
}

func TestUpdateSummaryNamesTheVersionOnOffer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		result update.Result
		want   string
	}{
		{update.Result{Offer: &update.Found{Version: "99.0.0"}},
			"Gophenberg 99.0.0 is available, this is " + version.Version()},
		{update.Result{Offer: &update.Found{Version: "99.0.0", LongJump: true, NeedsPostgreSQL: true, PostgreSQL: 17}},
			"Gophenberg 99.0.0 is available, this is " + version.Version() +
				", it needs PostgreSQL 17 or newer, and going back to this version after it needs the backup"},
		{update.Result{}, "Gophenberg " + version.Version() + " is the newest version the update site offers"},
	} {
		if got := updateSummary(tc.result); got != tc.want {
			t.Errorf("updateSummary(%+v) = %q, want %q", tc.result.Offer, got, tc.want)
		}
	}
}

func TestUpdateCheckPrintsOneLineForPeople(t *testing.T) {
	t.Parallel()

	site := offeringSite(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptySQLiteAddress(t), "GOPHENBERG_UPDATE_URL": site.address,
		"GOPHENBERG_TRUSTED_KEYS": site.trustedRoots(),
	}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:check")

	if got.Code != 0 || !strings.Contains(got.Stdout, "Gophenberg 99.0.0 is available") {
		t.Errorf("update:check = %d, %q, %q, want 0 and the version on offer", got.Code, got.Stdout, got.Stderr)
	}
}

func TestUpdateCheckRefusesSettingsItCannotUse(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_URL": emptySQLiteAddress(t), "GOPHENBERG_UPDATE_CHANNEL": "../x"}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:check")

	if got.Code == 0 || !strings.Contains(got.Stderr, "GOPHENBERG_UPDATE_CHANNEL") {
		t.Errorf("update:check = %d, %q, want a refusal naming the channel", got.Code, got.Stderr)
	}
}

func TestUpdateCheckReportsADatabaseItCannotReach(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_URL": "postgres://postgres:gophenberg@localhost:9/postgres?pool_x=1"}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:check")

	if got.Code == 0 {
		t.Errorf("update:check on an address it refuses = %d, %q, want a failure", got.Code, got.Stderr)
	}
}
