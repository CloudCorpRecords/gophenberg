// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/update"
)

// siteID is the identity of the site every index is read for.
const siteID = "5f1c0e9a2b7d4c3e8a6b1d0f9e2c7a4b"

// site returns the site running version on a PostgreSQL server of major.
func site(running string, major int) update.Site {
	return update.Site{ID: siteID, Running: running, ServerMajor: major}
}

func TestUpdateForTakesTheNewestOfferAboveTheRunningVersion(t *testing.T) {
	t.Parallel()

	offers := index(offer("0.21.0", "0.21.0"), offer("0.22.0", "0.22.0"), offer("0.20.2", "0.20.0"))

	got := offers.UpdateFor(site("0.20.1", 18))

	if got == nil || got.Version != "0.22.0" {
		t.Errorf("UpdateFor() = %+v, want 0.22.0, the newest above 0.20.1", got)
	}
}

func TestUpdateForFindsNothingWhenNoOfferIsNewer(t *testing.T) {
	t.Parallel()

	for _, running := range []string{"0.22.0", "0.23.0"} {
		if got := index(offer("0.22.0", "0.22.0"), offer("0.21.0", "0.21.0")).UpdateFor(site(running, 18)); got != nil {
			t.Errorf("UpdateFor() on %s = %+v, want nothing", running, got)
		}
	}
}

func TestUpdateForSkipsAWithdrawnOffer(t *testing.T) {
	t.Parallel()

	withdrawn := offer("0.22.0", "0.22.0")
	withdrawn.Withdrawn = true

	got := index(withdrawn, offer("0.21.0", "0.21.0")).UpdateFor(site("0.20.0", 18))

	if got == nil || got.Version != "0.21.0" {
		t.Errorf("UpdateFor() = %+v, want 0.21.0, since 0.22.0 is withdrawn", got)
	}
}

func TestUpdateForSkipsAVersionTheSiteRefused(t *testing.T) {
	t.Parallel()

	refusing := site("0.20.0", 18)
	refusing.Refused = []string{"0.22.0"}

	got := index(offer("0.21.0", "0.21.0"), offer("0.22.0", "0.22.0")).UpdateFor(refusing)

	if got == nil || got.Version != "0.21.0" {
		t.Errorf("UpdateFor() = %+v, want 0.21.0, since the site refused 0.22.0", got)
	}
}

func TestLiftLetsASiteTakeARefusedVersionAgain(t *testing.T) {
	t.Parallel()

	refusing := site("0.20.0", 18)
	refusing.Refused = []string{"0.21.0", "0.22.0"}

	lifted := refusing.Lift("0.22.0")

	if got := index(offer("0.22.0", "0.22.0")).UpdateFor(lifted); got == nil || got.Version != "0.22.0" {
		t.Errorf("UpdateFor() after Lift(0.22.0) = %+v, want 0.22.0 taken again", got)
	}
	if !slices.Equal(lifted.Refused, []string{"0.21.0"}) {
		t.Errorf("Lift(0.22.0) refuses %v, want only 0.21.0 left", lifted.Refused)
	}
	if !slices.Equal(refusing.Refused, []string{"0.21.0", "0.22.0"}) {
		t.Errorf("Lift(0.22.0) changed the site it was called on to %v, want it kept", refusing.Refused)
	}
}

func TestUpdateForTakesAnOfferOnlyWhenItsRolloutReachesTheSite(t *testing.T) {
	t.Parallel()

	bucket := update.Bucket(siteID, "0.21.0")
	for rollout, reached := range map[int]bool{0: false, bucket: false, bucket + 1: true, 100: true} {
		partial := offer("0.21.0", "0.21.0")
		partial.Rollout = rollout
		got := index(partial).UpdateFor(site("0.20.0", 18))
		if (got != nil) != reached {
			t.Errorf("UpdateFor() at rollout %d for bucket %d = %+v, want reached %v", rollout, bucket, got, reached)
		}
	}
}

func TestBucketIsTheFirstTwoBytesOfTheDigestOfTheSiteAndVersion(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"0.21.0", "0.22.0", "1.0.0"} {
		digest := sha256.Sum256([]byte(siteID + version))
		want := int(binary.BigEndian.Uint16(digest[:2]) % 100)
		if got := update.Bucket(siteID, version); got != want {
			t.Errorf("Bucket(%s) = %d, want %d", version, got, want)
		}
	}
}

func TestUpdateForNamesALongJumpAndANewerPostgreSQL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		minimum         string
		major           int
		longJump, newer bool
	}{
		{name: "a short step on a recent server", minimum: "0.20.0", major: 18},
		{name: "a long jump", minimum: "0.21.0", major: 18, longJump: true},
		{name: "an older server", minimum: "0.20.0", major: 14, newer: true},
		{name: "the oldest server it takes", minimum: "0.20.0", major: 15},
		{name: "a SQLite file", minimum: "0.20.0"},
	}
	for _, tc := range cases {
		got := index(offer("0.21.0", tc.minimum)).UpdateFor(site("0.20.1", tc.major))
		if got == nil || got.LongJump != tc.longJump || got.NeedsPostgreSQL != tc.newer {
			t.Errorf("UpdateFor() for %s = %+v, want long jump %v and newer PostgreSQL %v", tc.name, got, tc.longJump,
				tc.newer)
		}
	}
}

func TestUpdateForCarriesWhatTheOfferSays(t *testing.T) {
	t.Parallel()

	auto := offer("0.21.0", "0.21.0")
	auto.Auto, auto.Kits = true, []string{"0.9.0", "0.15.1"}

	got := index(auto).UpdateFor(site("0.20.0", 18))

	if got == nil || got.MinimumVersion != "0.21.0" || got.PostgreSQL != 15 || !got.Auto ||
		got.Notes != "https://example.com/releases/0.21.0" || !slices.Equal(got.Kits, []string{"0.9.0", "0.15.1"}) {
		t.Errorf("UpdateFor() = %+v, want every field of the offer carried", got)
	}
}
