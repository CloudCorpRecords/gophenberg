// SPDX-License-Identifier: Apache-2.0

package update

import (
	"fmt"
	"net"
	"net/url"
)

// DefaultAddress is the official update site.
const DefaultAddress = "https://updates.gophenberg.org"

// ParseAddress returns the update site text names: https, or plain http on a loopback host, with no user or query.
func ParseAddress(text string) (*url.URL, error) {
	address, err := url.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("update: refused the address %q: %w", text, err)
	}
	switch {
	case address.Host == "":
		return nil, fmt.Errorf("update: refused the address %q, which names no host", text)
	case address.User != nil:
		return nil, fmt.Errorf("update: refused the address %q, which names a user", text)
	case address.RawQuery != "" || address.Fragment != "":
		return nil, fmt.Errorf("update: refused the address %q, which holds a query or a fragment", text)
	case address.Scheme == "https", address.Scheme == "http" && loopback(address.Hostname()):
		return address, nil
	}
	return nil, fmt.Errorf("update: refused the address %q, which must be https, or http on a loopback host", text)
}

// loopback reports whether host names this machine's loopback.
func loopback(host string) bool {
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}
