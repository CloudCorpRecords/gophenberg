// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// checkPaginationPaths refuses addresses a write would take from the root listing, naming the first one.
func checkPaginationPaths(paths []string) error {
	for _, path := range paths {
		if content.RootPagination(path) {
			return content.ReservedAddress(path)
		}
	}
	return nil
}

// checkPaginationEdit refuses a reserved address unless the stored item already occupies it.
func checkPaginationEdit(ctx context.Context, queries *db.Queries, params db.UpdateContentParams) error {
	if !content.RootPagination(params.Path) {
		return nil
	}
	stored, err := byID(ctx, queries, params.ID)
	if err != nil {
		return err
	}
	if stored.Path != params.Path {
		return content.ReservedAddress(params.Path)
	}
	return nil
}

// moveDescendants carries the children to the requested address, refusing a reserved address.
func moveDescendants(ctx context.Context, queries *db.Queries, params db.MoveDescendantsParams) error {
	paths, err := queries.MoveDescendants(ctx, params)
	if err != nil {
		return err
	}
	return checkPaginationPaths(paths)
}
