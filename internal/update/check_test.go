// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
)

// updateServer is a test update site serving the files it holds under a base path, recording each request's headers.
type updateServer struct {
	t       *testing.T
	base    string
	mu      sync.Mutex
	files   map[string][]byte
	headers []http.Header
	delay   time.Duration
	server  *httptest.Server
}

// newUpdateServer starts a test update site under base, serving the key list and index k signs.
func newUpdateServer(t *testing.T, k keys, base string, offers ...update.Offer) *updateServer {
	t.Helper()
	s := &updateServer{t: t, base: base, files: map[string][]byte{}}
	s.server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.server.Close)
	list := marshal(t, k.keyList(t))
	s.put(update.KeyListPath, list)
	s.put(update.KeyListPath+update.SignatureSuffix, signLines(t, list, k.root1))
	s.putIndex(k, index(offers...))
	return s
}

// serve answers the file the request names under the base path, after the delay.
func (s *updateServer) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.headers = append(s.headers, r.Header.Clone())
	body, found := s.files[strings.TrimPrefix(r.URL.Path, s.base+"/")]
	delay := s.delay
	s.mu.Unlock()
	time.Sleep(delay)
	if !found {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(body)
}

// put serves body at path.
func (s *updateServer) put(path string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[path] = body
}

// drop serves nothing at path.
func (s *updateServer) drop(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, path)
}

// putIndex serves the index the release key signed, at the path of its channel.
func (s *updateServer) putIndex(k keys, held update.Index) {
	body := marshal(s.t, held)
	s.put(update.IndexPath(held.Channel), body)
	s.put(update.IndexPath(held.Channel)+update.SignatureSuffix, signIndex(body, k.release))
}

// config returns the check of the stable channel on the server, for a site running 0.20.0 on PostgreSQL 18.
func (s *updateServer) config(k keys) update.Config {
	address, err := url.Parse(s.server.URL + s.base)
	if err != nil {
		s.t.Fatalf("reading the test address: %v", err)
	}
	return update.Config{
		Address: address, Channel: "stable", Roots: k.trusted(), Client: update.NewClient(time.Second),
		Site: site("0.20.0", 18), Now: func() time.Time { return now },
	}
}

func TestCheckPicksAnOfferAndReturnsTheSerialsItTook(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "/mirror", offer("0.21.0", "0.21.0"))

	found, taken, err := update.Check(t.Context(), server.config(k), update.Serials{})

	if err != nil || found == nil || found.Version != "0.21.0" {
		t.Fatalf("Check() = %+v, %v, want 0.21.0 offered", found, err)
	}
	if taken != (update.Serials{Keys: 5, Index: 9}) {
		t.Errorf("Check() took serials %+v, want the key list's 5 and the index's 9", taken)
	}
}

func TestCheckSendsNothingButItsUserAgent(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")

	if _, _, err := update.Check(t.Context(), server.config(k), update.Serials{}); err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if len(server.headers) != 4 {
		t.Fatalf("Check() made %d requests, want the key list, the index and their two signatures", len(server.headers))
	}
	for _, header := range server.headers {
		names := slices.Sorted(func(yield func(string) bool) {
			for name := range header {
				if !yield(name) {
					return
				}
			}
		})
		if !slices.Equal(names, []string{"User-Agent"}) || header.Get("User-Agent") != "Gophenberg/0.20.0" {
			t.Errorf("a request carried %v, want only User-Agent: Gophenberg/0.20.0", header)
		}
	}
}

func TestCheckRefusesSerialsOlderThanTheOnesTaken(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "", offer("0.21.0", "0.21.0"))
	for taken, text := range map[update.Serials]string{
		{Keys: 6, Index: 9}:  "key list of serial 5, older than serial 6",
		{Keys: 5, Index: 10}: "index of serial 9, older than serial 10",
	} {
		_, _, err := update.Check(t.Context(), server.config(k), taken)
		wantRefused(t, err, text)
	}
	if _, _, err := update.Check(t.Context(), server.config(k), update.Serials{Keys: 5, Index: 9}); err != nil {
		t.Errorf("Check() with the same serials taken = %v, want the same files taken again", err)
	}
}

func TestCheckRefusesAMissingFile(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	for _, path := range []string{
		update.KeyListPath, update.KeyListPath + update.SignatureSuffix,
		update.IndexPath("stable"), update.IndexPath("stable") + update.SignatureSuffix,
	} {
		server := newUpdateServer(t, k, "")
		server.drop(path)
		_, _, err := update.Check(t.Context(), server.config(k), update.Serials{})
		wantRefused(t, err, "404")
	}
}

func TestCheckRefusesAFileLargerThanTheCap(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	server.put(update.IndexPath("stable"), []byte(strings.Repeat(" ", 64<<10+1)))

	_, _, err := update.Check(t.Context(), server.config(k), update.Serials{})

	wantRefused(t, err, fmt.Sprintf("larger than %d bytes", update.MaxFileSize))
}

func TestCheckRefusesASiteThatAnswersTooLate(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	server.delay = 2 * time.Second

	_, _, err := update.Check(t.Context(), server.config(k), update.Serials{})

	wantRefused(t, err, "fetch")
}

func TestCheckRefusesASiteNothingAnswersAt(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	config := server.config(k)
	server.server.Close()

	_, _, err := update.Check(t.Context(), config, update.Serials{})

	wantRefused(t, err, "fetch")
}

func TestCheckRefusesARedirect(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	target := newUpdateServer(t, k, "")
	moved := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.server.URL+r.URL.Path, http.StatusMovedPermanently)
	}))
	t.Cleanup(moved.Close)
	config := target.config(k)
	config.Address, _ = url.Parse(moved.URL)

	_, _, err := update.Check(t.Context(), config, update.Serials{})

	wantRefused(t, err, "301")
	if len(target.headers) != 0 {
		t.Errorf("the redirect target got %d requests, want the redirect refused before it", len(target.headers))
	}
}

// closingTransport is a transport that counts the times its client lets go of the idle connections.
type closingTransport struct {
	*http.Transport
	closed int
}

// CloseIdleConnections counts the call and closes the idle connections.
func (c *closingTransport) CloseIdleConnections() {
	c.closed++
	c.Transport.CloseIdleConnections()
}

func TestCheckLetsGoOfItsIdleConnections(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	server := newUpdateServer(t, k, "")
	transport := &closingTransport{Transport: &http.Transport{}}
	config := server.config(k)
	config.Client = &http.Client{Transport: transport}

	if _, _, err := update.Check(t.Context(), config, update.Serials{}); err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if transport.closed != 1 {
		t.Errorf("idle connections closed %d times, want once when the check ended", transport.closed)
	}
}

func TestCheckRefusesAFileCutShort(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("short"))
	}))
	t.Cleanup(short.Close)
	config := newUpdateServer(t, k, "").config(k)
	config.Address, _ = url.Parse(short.URL)

	_, _, err := update.Check(t.Context(), config, update.Serials{})

	wantRefused(t, err, "unexpected EOF")
}

func TestCheckRefusesAKeyListOrIndexItCannotTake(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	unsignedList := newUpdateServer(t, k, "")
	unsignedList.put(update.KeyListPath+update.SignatureSuffix, signLines(t, []byte("other"), k.root1))
	_, _, err := update.Check(t.Context(), unsignedList.config(k), update.Serials{})
	wantRefused(t, err, "does not hold")

	unsignedIndex := newUpdateServer(t, k, "")
	unsignedIndex.put(update.IndexPath("stable")+update.SignatureSuffix, signIndex([]byte("other"), k.release))
	_, _, err = update.Check(t.Context(), unsignedIndex.config(k), update.Serials{})
	wantRefused(t, err, "no release or standby key the list names")
}
