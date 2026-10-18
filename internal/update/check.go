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

// Check fetches and verifies the key list and the index, refuses serials older than the ones taken, finds the update.
func Check(ctx context.Context, cfg Config, taken Serials) (*Found, Serials, error) {
	defer cfg.Client.CloseIdleConnections()
	list, err := cfg.keyList(ctx)
	if err != nil {
		return nil, taken, err
	}
	if err := notOlder("key list", list.Serial, taken.Keys); err != nil {
		return nil, taken, err
	}
	index, err := cfg.index(ctx, list)
	if err != nil {
		return nil, taken, err
	}
	if err := notOlder("index", index.Serial, taken.Index); err != nil {
		return nil, taken, err
	}
	return index.UpdateFor(cfg.Site), Serials{Keys: list.Serial, Index: index.Serial}, nil
}

// notOlder returns an error when the serial of the file of kind comes before the one taken last.
func notOlder(kind string, serial, taken int64) error {
	if serial < taken {
		return fmt.Errorf("update: refused the %s of serial %d, older than serial %d taken before", kind, serial, taken)
	}
	return nil
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
