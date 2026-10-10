// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole"
	accounts "github.com/gopherium/framework/gonsole/auth"

	"github.com/gopherium/gophenberg/internal/role"
)

// recordTimeout bounds storing one command record when COMMAND_RECORD_TIMEOUT names no other.
const recordTimeout = 5 * time.Second

// recordsLimit is how many records account:records lists when COMMAND_RECORDS_LIMIT names no other.
const recordsLimit = 50

// accountConfig returns the configuration of the account commands and of the check on the acting account over registry.
func accountConfig(registry *goncierge.Registry) accounts.Config {
	return accounts.Config{
		Roles:         declaredRoles(registry),
		Capability:    string(role.ManageUsers),
		RecordTimeout: recordTimeout,
		RecordsLimit:  recordsLimit,
	}
}

// declaredRoles returns the hook answering the roles the core and the call's plugins declared into registry.
func declaredRoles(registry *goncierge.Registry) func(context.Context, gonsole.Call) (accounts.Roles, error) {
	return func(ctx context.Context, call gonsole.Call) (accounts.Roles, error) {
		if _, err := call.Plugins(ctx); err != nil {
			return accounts.Roles{}, err
		}
		return accounts.RolesFrom(registry, registry.HoldersOf(string(role.ManageUsers))), nil
	}
}
