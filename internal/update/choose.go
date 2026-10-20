// SPDX-License-Identifier: Apache-2.0

package update

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/gopherium/gophenberg/internal/version"
)

// ErrNoUpdate says the update site offers this site no version newer than the one it runs.
var ErrNoUpdate = errors.New("update: the update site offers no version newer than the one this site runs")

// RefusedError refuses a version that failed its start check after an earlier update, never taken again on its own.
type RefusedError struct {
	Version string
}

// Error says which version the site refused and why.
func (e RefusedError) Error() string {
	return "update: refused Gophenberg " + e.Version + ", which failed its start check after an earlier update"
}

// Chosen is the offer an update takes, read again from the signed index, with its release record and signing keys.
type Chosen struct {
	Found   Found
	Release Release
	Signers []ed25519.PublicKey
}

// Choose verifies the key list and the index again, then returns the offer named, or the newest the site takes.
func Choose(ctx context.Context, cfg Config, store Settings, named string) (Chosen, error) {
	defer cfg.Client.CloseIdleConnections()
	cfg, taken, err := prepared(ctx, cfg, store)
	if err != nil {
		return Chosen{}, err
	}
	files, err := cfg.verified(ctx, taken)
	if err != nil {
		return Chosen{}, err
	}
	offer, err := files.index.pick(cfg.Site, named)
	if err != nil {
		return Chosen{}, err
	}
	release, err := offer.record()
	if err != nil {
		return Chosen{}, err
	}
	return Chosen{Found: *cfg.Site.found(offer), Release: release, Signers: files.list.Signers()}, nil
}

// pick returns the offer of the version named, or the newest one the site takes when named is empty.
func (i Index) pick(site Site, named string) (Offer, error) {
	if named != "" {
		return i.named(site, named)
	}
	if newest := i.newestFor(site); newest != nil {
		return *newest, nil
	}
	if skipped := i.newestFor(Site{ID: site.ID, Running: site.Running}); skipped != nil {
		return Offer{}, RefusedError{Version: skipped.Version()}
	}
	return Offer{}, ErrNoUpdate
}

// named returns the offer of the version a person named, refusing one withdrawn, held back, not newer or refused.
func (i Index) named(site Site, named string) (Offer, error) {
	offer, found := i.offerOf(named)
	switch {
	case !found:
		return Offer{}, fmt.Errorf("update: the update site offers no Gophenberg %s", named)
	case offer.Withdrawn:
		return Offer{}, fmt.Errorf("update: refused Gophenberg %s, which the update site withdrew", named)
	case offer.Rollout == 0:
		return Offer{}, fmt.Errorf("update: refused Gophenberg %s, which the update site holds back from every site for now",
			named)
	case !version.Before(site.Running, named):
		return Offer{}, fmt.Errorf("update: refused Gophenberg %s, which is not newer than %s, the version this site runs",
			named, site.Running)
	case site.refuses(named):
		return Offer{}, RefusedError{Version: named}
	}
	return offer, nil
}

// offerOf returns the offer of the version named, and whether the index holds one.
func (i Index) offerOf(named string) (Offer, bool) {
	for _, offer := range i.Offers {
		if offer.Version() == named {
			return offer, true
		}
	}
	return Offer{}, false
}
