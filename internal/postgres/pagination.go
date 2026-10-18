// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

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

// moveDescendants carries the children to path, refusing a reserved address inside the same transaction.
func moveDescendants(ctx context.Context, queries *db.Queries, id uuid.UUID, path string, at time.Time) error {
	paths, err := queries.MoveDescendants(ctx, db.MoveDescendantsParams{ID: id, Path: path, UpdatedAt: at})
	if err != nil {
		return err
	}
	return checkPaginationPaths(paths)
}
