// SPDX-License-Identifier: Apache-2.0

package update_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/update"
)

func TestParseAddressTakesHTTPSAndPlainHTTPOnlyOnLoopback(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		update.DefaultAddress, "https://mirror.example.com/gophenberg", "http://127.0.0.1:8090",
		"http://localhost:8090/updates", "http://[::1]:8090",
	} {
		if _, err := update.ParseAddress(text); err != nil {
			t.Errorf("ParseAddress(%q) = %v, want it taken", text, err)
		}
	}
}

func TestParseAddressRefusesEveryOtherAddress(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"http://mirror.example.com", "ftp://mirror.example.com", "https://", "mirror.example.com",
		"https://mirror.example.com/?channel=beta", "https://mirror.example.com/#top", "https://%zz",
		"https://maria@mirror.example.com",
	} {
		if _, err := update.ParseAddress(text); err == nil {
			t.Errorf("ParseAddress(%q) = nil, want it refused", text)
		}
	}
}
