// SPDX-License-Identifier: Apache-2.0

package update

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// MaxFileSize is the most bytes a check reads of one file of the update site.
const MaxFileSize = 64 << 10

const (
	// takenBefore names the serial a site took from an earlier check.
	takenBefore = "taken before"
	// namedByTimestamp names the serial the timestamp file names.
	namedByTimestamp = "the timestamp file names"
)

// Config is what a check needs: the update site, the channel, the trusted roots, the client and the site it is for.
type Config struct {
	Address *url.URL
	Channel string
	Roots   []*ecdsa.PublicKey
	Client  *http.Client
	Site    Site
	Now     func() time.Time
}

// Serials are the serials of the last key list and index a site took.
type Serials struct {
	Keys  int64 `json:"keys"`
	Index int64 `json:"index"`
}

// NewClient returns the client a check fetches with, giving up after timeout and following no redirect.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true, DisableCompression: true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// signed is a key list a trusted root signed and the index a key it names signed.
type signed struct {
	list  KeyList
	index Index
}

// Check fetches and verifies the key list, the timestamp file and the index, refuses older serials, finds the update.
func Check(ctx context.Context, cfg Config, taken Serials) (*Found, Serials, error) {
	defer cfg.Client.CloseIdleConnections()
	files, err := cfg.verified(ctx, taken)
	if err != nil {
		return nil, taken, err
	}
	return files.index.UpdateFor(cfg.Site), files.serials(), nil
}

// serials returns the serials of the key list and the index.
func (s signed) serials() Serials {
	return Serials{Keys: s.list.Serial, Index: s.index.Serial}
}

// verified fetches and verifies the key list, the timestamp file and the index, refusing older serials.
func (c Config) verified(ctx context.Context, taken Serials) (signed, error) {
	list, err := c.keyList(ctx)
	if err != nil {
		return signed{}, err
	}
	if err := notOlder("key list", list.Serial, taken.Keys, takenBefore); err != nil {
		return signed{}, err
	}
	named, err := c.namedIndexSerial(ctx, list)
	if err != nil {
		return signed{}, err
	}
	index, err := c.index(ctx, list)
	if err != nil {
		return signed{}, err
	}
	if err := notOlder("index", index.Serial, taken.Index, takenBefore); err != nil {
		return signed{}, err
	}
	if err := notOlder("index", index.Serial, named, namedByTimestamp); err != nil {
		return signed{}, err
	}
	return signed{list: list, index: index}, nil
}

// notOlder returns an error when the serial of the file of kind comes before floor, the serial source names.
func notOlder(kind string, serial, floor int64, source string) error {
	if serial < floor {
		return fmt.Errorf("update: refused the %s of serial %d, older than serial %d %s", kind, serial, floor, source)
	}
	return nil
}

// namedIndexSerial returns the index serial the timestamp file names for the channel, refusing a file it cannot take.
func (c Config) namedIndexSerial(ctx context.Context, list KeyList) (int64, error) {
	body, sig, err := c.fetchSigned(ctx, TimestampPath)
	if err != nil {
		return 0, err
	}
	stamp, err := list.ReadTimestamp(body, sig, c.Now())
	if err != nil {
		return 0, err
	}
	serial, err := stamp.indexOf(c.Channel)
	if err != nil {
		return 0, err
	}
	return serial, notOlder("key list", list.Serial, stamp.Keys, namedByTimestamp)
}

// keyList fetches the key list and its signatures and reads them under the trusted roots.
func (c Config) keyList(ctx context.Context) (KeyList, error) {
	body, sigs, err := c.fetchSigned(ctx, KeyListPath)
	if err != nil {
		return KeyList{}, err
	}
	return ReadKeyList(body, sigs, c.Roots, c.Now())
}

// index fetches the channel's index and its signature and reads them under the key list.
func (c Config) index(ctx context.Context, list KeyList) (Index, error) {
	body, sig, err := c.fetchSigned(ctx, IndexPath(c.Channel))
	if err != nil {
		return Index{}, err
	}
	return list.ReadIndex(body, sig, c.Channel, c.Now())
}

// fetchSigned fetches the file at path, then the file that signs it.
func (c Config) fetchSigned(ctx context.Context, path string) ([]byte, []byte, error) {
	body, err := c.fetch(ctx, path)
	if err != nil {
		return nil, nil, err
	}
	sig, err := c.fetch(ctx, path+SignatureSuffix)
	return body, sig, err
}

// fetch returns the file at path on the update site, sending only the User-Agent and reading at most maxFileSize.
func (c Config) fetch(ctx context.Context, path string) ([]byte, error) {
	address := c.Address.JoinPath(path).String()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	request.Header = http.Header{"User-Agent": {"Gophenberg/" + c.Site.Running}}
	response, err := c.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("update: fetch %s: %w", address, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: fetch %s: the site answered %s", address, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("update: fetch %s: %w", address, err)
	}
	if len(body) > MaxFileSize {
		return nil, fmt.Errorf("update: refused %s, larger than %d bytes", address, MaxFileSize)
	}
	return body, nil
}
