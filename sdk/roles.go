// SPDX-License-Identifier: Apache-2.0

package sdk

// RoleDeclaration is one role a plugin declares, or capabilities it adds to a role the host knows.
type RoleDeclaration struct {
	Name         string
	Capabilities []string
}

// RoleProvider is implemented by plugins declaring roles or widening the ones the host knows.
type RoleProvider interface {
	Roles() []RoleDeclaration
}
