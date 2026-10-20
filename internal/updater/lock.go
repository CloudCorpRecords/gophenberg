// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const (
	// lockQuery takes the update lock for the session, answering false while another session holds it.
	lockQuery = "SELECT pg_try_advisory_lock(hashtext('core.update'))"
	// unlockQuery lets go of the update lock the session holds.
	unlockQuery = "SELECT pg_advisory_unlock(hashtext('core.update'))"
)

// ErrUpdateRuns stops a job while another update holds the update lock.
var ErrUpdateRuns = errors.New("updater: another update runs, so wait for it to end and run update:apply again")

// Lock takes the update lock on a connection of db it keeps, and returns what lets go of it and of the connection.
func Lock(ctx context.Context, db *sql.DB) (func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("updater: take the update lock: %w", err)
	}
	if err := tryLock(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), unlockQuery)
		_ = conn.Close()
	}, nil
}

// tryLock takes the update lock on conn, refusing while another session holds it.
func tryLock(ctx context.Context, conn *sql.Conn) error {
	var taken bool
	if err := conn.QueryRowContext(ctx, lockQuery).Scan(&taken); err != nil {
		return fmt.Errorf("updater: take the update lock: %w", err)
	}
	if !taken {
		return ErrUpdateRuns
	}
	return nil
}
