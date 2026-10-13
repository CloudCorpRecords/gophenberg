// SPDX-License-Identifier: Apache-2.0

package sdk

import "context"

// Person is an account as plugins see it: its public face and never its email address.
type Person struct {
	ID       ID
	Name     string
	Role     string
	Avatar   string
	Disabled bool
}

// AccountReader reads many accounts by id in one call for plugins, which only the site implements.
type AccountReader interface {
	People(ctx context.Context, ids []ID, avatarSize int) (map[ID]Person, error)
	hostOnly()
}
