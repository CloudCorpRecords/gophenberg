// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
)

// offerUpdate starts an update site whose stable index offers version, and points the settings at it.
func (s *operatorScenario) offerUpdate(version string) {
	s.site = newUpdateSite(s.t)
	s.site.offered = version
	s.site.publishKeyList("the first root", "the second root")
	s.site.publishIndex(s.site.release, unchanged)
	s.env["GOPHENBERG_UPDATE_URL"] = s.site.address
	s.env["GOPHENBERG_TRUSTED_KEYS"] = s.site.trustedRoots()
}

// signKeyListAlone serves the key list signed by the one key name.
func (s *operatorScenario) signKeyListAlone(name string) {
	s.site.publishKeyList(name)
}

// signIndexAsStranger serves the index signed by a key the key list does not name.
func (s *operatorScenario) signIndexAsStranger() {
	s.site.publishIndex(s.site.stranger, unchanged)
}

// expireIndex serves an index that expired a day ago.
func (s *operatorScenario) expireIndex() {
	s.site.publishIndex(s.site.release, func(index *update.Index) { index.Expires = time.Now().AddDate(0, 0, -1) })
}

// renameChannel serves an index naming channel.
func (s *operatorScenario) renameChannel(channel string) {
	s.site.publishIndex(s.site.release, func(index *update.Index) { index.Channel = channel })
}

// goBackAnIndex serves an index of a serial older than the one served before.
func (s *operatorScenario) goBackAnIndex() {
	s.site.publishIndex(s.site.release, func(index *update.Index) { index.Serial = 1 })
}

// ranLine runs the command line and fails unless it succeeds.
func (s *operatorScenario) ranLine(line string) error {
	s.runLine(line)
	return s.succeeds()
}

// offeredResult returns the result update:check -json printed.
func (s *operatorScenario) offeredResult() (update.Result, error) {
	var result update.Result
	if err := json.Unmarshal([]byte(s.result.Stdout), &result); err != nil {
		return result, fmt.Errorf("the answer %q is no JSON document: %w", s.result.Stdout, err)
	}
	return result, nil
}

// offersVersion fails unless the answer offers version.
func (s *operatorScenario) offersVersion(version string) error {
	result, err := s.offeredResult()
	if err != nil {
		return err
	}
	if result.Offer == nil || result.Offer.Version != version {
		return fmt.Errorf("the answer %q offers %+v, want version %s", s.result.Stdout, result.Offer, version)
	}
	return nil
}

// offersNoVersion fails unless the answer offers nothing.
func (s *operatorScenario) offersNoVersion() error {
	result, err := s.offeredResult()
	if err != nil {
		return err
	}
	if result.Offer != nil {
		return fmt.Errorf("the answer %q offers %+v, want nothing", s.result.Stdout, result.Offer)
	}
	return nil
}
