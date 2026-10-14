// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
)

// inTx runs body in one write transaction on handle, committing it when body succeeds and rolling it back otherwise.
func inTx(ctx context.Context, handle *sql.DB, body func(tx *sql.Tx) error) error {
	tx, err := handle.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := body(tx); err != nil {
		return err
	}
	return tx.Commit()
}
