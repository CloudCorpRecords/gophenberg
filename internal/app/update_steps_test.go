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
	s.offerRelease(version, version)
}

// offerRelease starts an update site whose stable index offers offered under minimum, and points the settings at it.
func (s *operatorScenario) offerRelease(offered, minimum string) {
	s.site = newOfferingSite(s.t, offered, minimum)
	s.env["GOPHENBERG_UPDATE_URL"] = s.site.address
	s.env["GOPHENBERG_TRUSTED_KEYS"] = s.site.trustedRoots()
}

// signKeyListAlone serves the key list signed by the one key name.
func (s *operatorScenario) signKeyListAlone(name string) {
	s.site.publishKeyList(unchanged, name)
}

// dropTimestampKey serves a key list, signed by both roots, that names no timestamp key.
func (s *operatorScenario) dropTimestampKey() {
	s.site.publishKeyList(func(list *update.KeyList) { list.Keys = list.Keys[:1] }, bothRoots...)
}

// listReleaseKeyAsTimestampKey serves a key list, signed by both roots, naming the release key as its timestamp key.
func (s *operatorScenario) listReleaseKeyAsTimestampKey() {
	s.site.publishKeyList(func(list *update.KeyList) { list.Keys[1].Multikey = list.Keys[0].Multikey }, bothRoots...)
}

// signIndexAsStranger serves the index signed by a key the key list does not name.
func (s *operatorScenario) signIndexAsStranger() {
	s.site.publishIndex(s.site.stranger, unchanged)
}

// signIndexAsTimestampKey serves the index signed by the timestamp key.
func (s *operatorScenario) signIndexAsTimestampKey() {
	s.site.publishIndex(s.site.timestamp, unchanged)
}

// signTimestampAsRelease serves the timestamp file signed by the release key.
func (s *operatorScenario) signTimestampAsRelease() {
	s.site.publishTimestamp(s.site.release, unchanged)
}

// expireTimestamp serves a timestamp file that expired a day ago.
func (s *operatorScenario) expireTimestamp() {
	s.site.publishTimestamp(s.site.timestamp, func(stamp *update.Timestamp) {
		stamp.Expires = time.Now().AddDate(0, 0, -1)
	})
}

// dropTimestamp serves no timestamp file and no signature of one.
func (s *operatorScenario) dropTimestamp() {
	s.site.drop(update.TimestampPath)
	s.site.drop(update.TimestampPath + update.SignatureSuffix)
}

// nameNewerKeyList serves a timestamp file naming a key list one serial past the one served.
func (s *operatorScenario) nameNewerKeyList() {
	s.site.publishTimestamp(s.site.timestamp, func(stamp *update.Timestamp) { stamp.Keys++ })
}

// nameOtherIndex serves a timestamp file naming an index one serial past or before the one served.
func (s *operatorScenario) nameOtherIndex(age string) {
	step := map[string]int64{"a newer": 1, "an older": -1}[age]
	s.site.publishTimestamp(s.site.timestamp, func(stamp *update.Timestamp) { stamp.Indexes["stable"] += step })
}

// nameNoChannel serves a timestamp file naming no index of the stable channel.
func (s *operatorScenario) nameNoChannel() {
	s.site.publishTimestamp(s.site.timestamp, func(stamp *update.Timestamp) { delete(stamp.Indexes, "stable") })
}

// ageIndex serves an index signed the days given ago that expires in the days given, with a fresh timestamp file.
func (s *operatorScenario) ageIndex(signed, expires int) {
	s.site.publishIndex(s.site.release, func(index *update.Index) {
		index.Serial = time.Now().AddDate(0, 0, -signed).Unix()
		index.Expires = time.Now().AddDate(0, 0, expires)
	})
}

// changeArchive serves other bytes in place of the archive the release record names.
func (s *operatorScenario) changeArchive() {
	s.site.put(s.site.archivePath(), []byte("an archive changed after its release"))
}

// signArchiveAsStranger serves the archive signed by a key the key list does not name, in the index too.
func (s *operatorScenario) signArchiveAsStranger() {
	s.site.publishArchive(s.site.stranger)
	s.site.publishIndex(s.site.release, unchanged)
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
