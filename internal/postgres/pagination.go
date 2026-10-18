// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// checkPaginationPaths refuses addresses a write would take from the root listing.
func checkPaginationPaths(paths []string) error {
	for _, path := range paths {
		if content.RootPagination(path) {
			return content.ErrReservedAddress
		}
	}
	return nil
}

// moveDescendants carries the children to the requested address, refusing a reserved address in the same transaction.
func moveDescendants(ctx context.Context, queries *db.Queries, params db.MoveDescendantsParams) error {
	paths, err := queries.MoveDescendants(ctx, params)
	if err != nil {
		return err
	}
	return checkPaginationPaths(paths)
}
