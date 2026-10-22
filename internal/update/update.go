// SPDX-License-Identifier: Apache-2.0

// Package update checks a signed update site for newer Gophenberg versions and keeps what each check found.
package update

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	// KeyListPath is where the key list sits on an update site.
	KeyListPath = "v1/keys.json"
	// TimestampPath is where the timestamp file sits on an update site.
	TimestampPath = "v1/timestamp.json"
	// SignatureSuffix ends the name of the file that signs another.
	SignatureSuffix = ".sig"
	// schema is the version of the file formats this release reads.
	schema = 1
)

// Role names what a listed key may sign.
type Role string

const (
	// RoleRelease signs the channel indexes and the release files.
	RoleRelease Role = "release"
	// RoleStandby signs them in place of a release key that leaked.
	RoleStandby Role = "standby"
	// RoleTimestamp signs the timestamp file that keeps the indexes fresh.
	RoleTimestamp Role = "timestamp"
)

// ListedKey is one key the key list names, with what it may sign.
type ListedKey struct {
	Role     Role   `json:"role"`
	Multikey string `json:"multikey"`
}

// KeyList is the list of release keys a root signed, with its serial and expiry.
type KeyList struct {
	Schema  int         `json:"schema"`
	Serial  int64       `json:"serial"`
	Expires time.Time   `json:"expires"`
	Keys    []ListedKey `json:"keys"`
}

// Offer is one version a channel index offers, with what a site needs to take it.
type Offer struct {
	Rollout        int             `json:"rollout"`
	Auto           bool            `json:"auto"`
	Withdrawn      bool            `json:"withdrawn"`
	MinimumVersion string          `json:"minimum_version"`
	PostgreSQL     int             `json:"postgresql"`
	Kits           []string        `json:"kits"`
	Notes          string          `json:"notes"`
	Release        json.RawMessage `json:"release"`
}

// Index is the list of offers of one channel, signed by a release or standby key.
type Index struct {
	Schema  int       `json:"schema"`
	Channel string    `json:"channel"`
	Serial  int64     `json:"serial"`
	Expires time.Time `json:"expires"`
	Offers  []Offer   `json:"offers"`
}

// Timestamp is the timestamp file a timestamp key signed, naming the serials of the newest key list and indexes.
type Timestamp struct {
	Schema  int              `json:"schema"`
	Expires time.Time        `json:"expires"`
	Keys    int64            `json:"keys"`
	Indexes map[string]int64 `json:"indexes"`
}

// IndexPath returns where the index of channel sits on an update site.
func IndexPath(channel string) string {
	return "v1/core/" + channel + ".json"
}

// readable returns an error unless a file of the kind named, of version schema, is still valid at now.
func readable(kind string, version int, expires, now time.Time) error {
	if err := knownSchema(kind, version); err != nil {
		return err
	}
	return unexpired(kind, expires, now)
}

// knownSchema returns an error unless a file of the kind named is of version schema, the one this version reads.
func knownSchema(kind string, version int) error {
	if version != schema {
		return fmt.Errorf("update: refused the %s of schema %d, this version reads schema %d", kind, version, schema)
	}
	return nil
}

// unexpired returns an error unless a file of the kind named names an expiry still ahead of now.
func unexpired(kind string, expires, now time.Time) error {
	switch {
	case expires.IsZero():
		return fmt.Errorf("update: refused the %s, which names no expiry", kind)
	case !now.Before(expires):
		return fmt.Errorf("update: refused the %s, which expired at %s, the site's clock reads %s", kind,
			expires.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339))
	}
	return nil
}
