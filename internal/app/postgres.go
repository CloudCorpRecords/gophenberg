// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/dbconns"
)

// poolSettingPrefix starts every pool setting pgx reads.
const poolSettingPrefix = "pool_"

// openPostgres returns the one pool and its view on the PostgreSQL database at address under the cap env names.
func openPostgres(env gonsole.Env, address string) (*dbkitpg.Handle, error) {
	conns, err := dbconns.MaxConnsFrom(env)
	if err != nil {
		return nil, err
	}
	if keys := poolSettings(address); len(keys) > 0 {
		return nil, fmt.Errorf("%s: remove %s from the address, %s sizes the pool",
			env.Key("DATABASE_URL"), strings.Join(keys, " and "), env.Key("DATABASE_MAX_CONNS"))
	}
	handle, err := dbkitpg.Open(address, dbkitpg.Options{MaxConns: conns})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", env.Key("DATABASE_URL"), err)
	}
	return handle, nil
}

// poolSettings returns the pool settings address carries in sorted order, none when pgx cannot parse it.
func poolSettings(address string) []string {
	config, err := pgx.ParseConfig(address)
	if err != nil {
		return nil
	}
	keys := slices.Sorted(maps.Keys(config.RuntimeParams))
	return slices.DeleteFunc(keys, func(key string) bool { return !strings.HasPrefix(key, poolSettingPrefix) })
}

// checkedFirst returns step, run at an address once openPostgres takes it and the database lets this binary on.
func checkedFirst(env gonsole.Env, step gonsole.Step) gonsole.Step {
	return gonsole.Step{Name: step.Name, Run: func(ctx context.Context, address string) error {
		handle, err := openPostgres(env, address)
		if err != nil {
			return err
		}
		if err := guardPostgres(ctx, handle.DB); err != nil {
			return errors.Join(err, handle.Close())
		}
		return errors.Join(handle.Close(), step.Run(ctx, address))
	}}
}

// postgresCore returns the core schema step that opens a capped handle at the address, migrates on it, then closes it.
func postgresCore(env gonsole.Env) gonsole.Step {
	return gonsole.Step{Name: "core", Run: func(ctx context.Context, address string) error {
		handle, err := openPostgres(env, address)
		if err != nil {
			return err
		}
		return errors.Join(migratePostgresCore(ctx, handle.DB), handle.Close())
	}}
}
