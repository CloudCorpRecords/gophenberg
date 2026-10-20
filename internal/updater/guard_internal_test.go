// SPDX-License-Identifier: Apache-2.0

package updater

import "testing"

func TestLoopbackReachesAListenerOnEveryInterfaceFromThisHost(t *testing.T) {
	t.Parallel()

	for address, want := range map[string]string{
		"0.0.0.0:8081":   "127.0.0.1:8081",
		"[::]:8081":      "[::1]:8081",
		"127.0.0.1:8081": "127.0.0.1:8081",
		"[::1]:8081":     "[::1]:8081",
		"10.0.0.7:8081":  "10.0.0.7:8081",
	} {
		if got := loopback(address); got != want {
			t.Errorf("loopback(%q) = %q, want %q", address, got, want)
		}
	}
}
