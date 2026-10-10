// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/sdk"
)

// guardPlugin returns the middleware requiring and filing a session on a plugin route, kept out of every cache.
func guardPlugin(auth *authkit.Handlers, roles *goncierge.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return readerOnly(auth.RequireSession(fileSession(roles, next)))
	}
}

// fileSession copies the authenticated identity onto the sdk seam, with the capabilities roles gives its role.
func fileSession(roles *goncierge.Registry, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := authkit.IdentityFromContext(r.Context())
		filed := sdk.NewSession(
			sdk.ID(identity.ID), identity.Email, identity.Name, identity.Role,
			roles.CapabilitiesOf(identity.Role),
		)
		next.ServeHTTP(w, r.WithContext(sdk.WithSession(r.Context(), filed)))
	})
}
