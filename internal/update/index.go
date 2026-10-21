// SPDX-License-Identifier: Apache-2.0

package update

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/internal/signature"
	"github.com/gopherium/gophenberg/internal/version"
)

// maxRollout is the share of sites an offer reaches when it reaches all of them.
const maxRollout = 100

// errUnsigned refuses an index no release or standby key of the key list signed.
var errUnsigned = errors.New("update: refused the index, no release or standby key the list names signed it")

// SignIndex returns the FAIR signature line of the exact bytes of an index under a release or standby key.
func SignIndex(body []byte, key ed25519.PrivateKey) string {
	digest, _ := signature.Digest(bytes.NewReader(body))
	return signature.Sign(key, digest)
}

// ReadIndex returns the index of channel body holds once a signer the list names signed it in sig, valid at now.
func (l KeyList) ReadIndex(body, sig []byte, channel string, now time.Time) (Index, error) {
	if !signedByList(body, strings.TrimSpace(string(sig)), l) {
		return Index{}, errUnsigned
	}
	var index Index
	if err := json.Unmarshal(body, &index); err != nil {
		return Index{}, fmt.Errorf("update: refused the index, which is not JSON: %w", err)
	}
	if err := readable("index", index.Schema, index.Expires, now); err != nil {
		return Index{}, err
	}
	if index.Channel != channel {
		return Index{}, fmt.Errorf("update: refused the index of channel %q, the site asks for %q", index.Channel, channel)
	}
	return index, checkOffers(index.Offers)
}

// signedByList reports whether one of the key list's signers signed body.
func signedByList(body []byte, sig string, list KeyList) bool {
	digest, _ := signature.Digest(bytes.NewReader(body))
	for _, key := range list.Signers() {
		if signature.Verify(key, digest, sig) == nil {
			return true
		}
	}
	return false
}

// checkOffers returns an error naming the first offer whose version, minimum version or rollout cannot be read.
func checkOffers(offers []Offer) error {
	for i, offer := range offers {
		switch {
		case !version.Valid(offer.Version()):
			return fmt.Errorf("update: refused the index, offer %d names no release version", i+1)
		case !version.Valid(offer.MinimumVersion):
			return fmt.Errorf("update: refused the index, offer %d names the minimum version %q", i+1,
				offer.MinimumVersion)
		case offer.Rollout < 0 || offer.Rollout > maxRollout:
			return fmt.Errorf("update: refused the index, offer %d rolls out to %d%% of sites", i+1, offer.Rollout)
		}
	}
	return nil
}
