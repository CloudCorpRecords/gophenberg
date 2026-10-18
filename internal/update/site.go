// SPDX-License-Identifier: Apache-2.0

package update

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"

	"github.com/gopherium/gophenberg/internal/version"
)

// buckets is how many shares of sites a rollout counts in.
const buckets = 100

// Site is what the index is read for: the site's identity, the version it runs and its PostgreSQL major.
type Site struct {
	ID          string
	Running     string
	ServerMajor int
}

// Found is the update a check found for a site, with what taking it asks of the site.
type Found struct {
	Version         string   `json:"version"`
	MinimumVersion  string   `json:"minimum_version"`
	PostgreSQL      int      `json:"postgresql"`
	Kits            []string `json:"kits"`
	Notes           string   `json:"notes"`
	Auto            bool     `json:"auto"`
	LongJump        bool     `json:"long_jump"`
	NeedsPostgreSQL bool     `json:"needs_postgresql"`
}

// UpdateFor returns the newest offer above the running version, not withdrawn and rolled out to the site, nil for none.
func (i Index) UpdateFor(site Site) *Found {
	var newest *Offer
	for n, offer := range i.Offers {
		if site.takes(offer) && newerThan(offer, newest) {
			newest = &i.Offers[n]
		}
	}
	if newest == nil {
		return nil
	}
	return site.found(*newest)
}

// Bucket returns the share of sites, from 0 to 99, that a site falls in for version.
func Bucket(siteID, release string) int {
	digest := sha256.Sum256([]byte(siteID + release))
	return int(binary.BigEndian.Uint16(digest[:2]) % buckets)
}

// newerThan reports whether offer is newer than the newest one so far, any offer while there is none.
func newerThan(offer Offer, newest *Offer) bool {
	return newest == nil || version.Before(newest.Version(), offer.Version())
}

// takes reports whether an offer is newer than the running version, not withdrawn and rolled out to the site.
func (s Site) takes(offer Offer) bool {
	if offer.Withdrawn || !version.Before(s.Running, offer.Version()) {
		return false
	}
	return Bucket(s.ID, offer.Version()) < offer.Rollout
}

// found returns what the site needs to know of the offer it takes.
func (s Site) found(offer Offer) *Found {
	return &Found{
		Version: offer.Version(), MinimumVersion: offer.MinimumVersion, PostgreSQL: offer.PostgreSQL,
		Kits: slices.Clone(offer.Kits), Notes: offer.Notes, Auto: offer.Auto,
		LongJump:        version.Before(s.Running, offer.MinimumVersion),
		NeedsPostgreSQL: s.ServerMajor > 0 && s.ServerMajor < offer.PostgreSQL,
	}
}
