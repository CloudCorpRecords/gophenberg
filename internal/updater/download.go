// SPDX-License-Identifier: Apache-2.0

// Package updater updates a server install: it unpacks a checked release, backs up the database and switches builds.
package updater

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gopherium/gophenberg/internal/signature"
	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/version"
)

// maxRedirects is the most redirects a download follows, as many as net/http follows on its own.
const maxRedirects = 10

// errUnsigned refuses an archive no release or standby key of the key list signed.
var errUnsigned = errors.New("updater: refused the archive, no release or standby key the key list names signed it")

// Download is one archive to fetch: its record entry, the keys that may sign it, its partial file, cap and client.
type Download struct {
	Artifact update.Artifact
	Signers  []ed25519.PublicKey
	Partial  string
	Cap      int64
	Client   *http.Client
}

// NewClient returns the client a download fetches with, giving up after timeout and following https redirects only.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true, DisableCompression: true,
		},
		CheckRedirect: followHTTPS,
	}
}

// Fetch downloads the archive into a new partial file and returns it checked and open at its start, deleted on failure.
func (d Download) Fetch(ctx context.Context) (*os.File, error) {
	defer d.Client.CloseIdleConnections()
	file, err := d.fetch(ctx)
	if err != nil {
		_ = os.Remove(d.Partial)
		return nil, err
	}
	return file, nil
}

// followHTTPS lets the client follow a redirect only to an https address, and only up to maxRedirects of them.
func followHTTPS(request *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("updater: stopped after %d redirects", maxRedirects)
	}
	if request.URL.Scheme != "https" {
		return fmt.Errorf("updater: refused a redirect to %s, which is not https", request.URL.Redacted())
	}
	return nil
}

// fetch writes the archive into the partial file and returns the file once it checks it, closing it otherwise.
func (d Download) fetch(ctx context.Context) (*os.File, error) {
	response, err := d.get(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	file, err := d.create()
	if err != nil {
		return nil, err
	}
	if err := d.fill(file, response.Body); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

// create makes a new partial file in place of any earlier one, so it never writes into a file another download holds.
func (d Download) create() (*os.File, error) {
	_ = os.Remove(d.Partial)
	file, err := os.OpenFile(d.Partial, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("updater: write the download: %w", err)
	}
	return file, nil
}

// fill copies body into file and checks it, leaving file at its start.
func (d Download) fill(file *os.File, body io.Reader) error {
	if err := d.copy(file, body); err != nil {
		return err
	}
	if err := d.check(file); err != nil {
		return err
	}
	_, _ = file.Seek(0, io.SeekStart)
	return nil
}

// get asks for the archive, sending only the User-Agent, and returns the answer once the site sends it.
func (d Download) get(ctx context.Context) (*http.Response, error) {
	address, err := update.ParseAddress(d.Artifact.URL)
	if err != nil {
		return nil, err
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, address.String(), nil)
	request.Header = http.Header{"User-Agent": {"Gophenberg/" + version.Version()}}
	response, err := d.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("updater: fetch the archive: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("updater: fetch %s: the site answered %s", address.Redacted(), response.Status)
	}
	return response, nil
}

// copy writes body into file, refusing an archive larger than the cap.
func (d Download) copy(file io.Writer, body io.Reader) error {
	written, err := io.Copy(file, io.LimitReader(body, d.Cap+1))
	if err != nil {
		return fmt.Errorf("updater: fetch the archive: %w", err)
	}
	if written > d.Cap {
		return fmt.Errorf("updater: refused the archive, larger than %d bytes", d.Cap)
	}
	return nil
}

// check refuses the archive file holds unless its checksum is the one the record names and a listed key signed it.
func (d Download) check(file io.ReadSeeker) error {
	_, _ = file.Seek(0, io.SeekStart)
	sum, _ := signature.Checksum(file)
	if sum != d.Artifact.Checksum {
		return fmt.Errorf("updater: refused the archive, its checksum is %s, not %s, the one the release record names",
			sum, d.Artifact.Checksum)
	}
	_, _ = file.Seek(0, io.SeekStart)
	digest, _ := signature.Digest(file)
	for _, key := range d.Signers {
		if signature.Verify(key, digest, d.Artifact.Signature) == nil {
			return nil
		}
	}
	return errUnsigned
}
