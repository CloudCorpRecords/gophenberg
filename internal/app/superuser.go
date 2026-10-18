// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/postgres"
)

// superuserPolicy is what serve does when it connects to PostgreSQL as a superuser.
type superuserPolicy string

const (
	// superuserWarn logs a warning and serves.
	superuserWarn superuserPolicy = "warn"
	// superuserRefuse stops before any migration.
	superuserRefuse superuserPolicy = "refuse"
)

// superuserWarning is what serve logs when it connects to PostgreSQL as a superuser under warn.
const superuserWarning = "gophenberg connects to its database as a superuser, which can run programs on the " +
	"database server, so connect as the database's own owner role"

// errSuperuser is the refusal of serve when it connects to PostgreSQL as a superuser under refuse.
var errSuperuser = errors.New("gophenberg connects to its database as a superuser and " + settingsPrefix +
	"SUPERUSER_POLICY is refuse, so connect as the database's own owner role")

// prepareDatabase checks the role the site connects as under its policy, then migrates the database it serves.
func prepareDatabase(ctx context.Context, settings runConfig, built site, logger *slog.Logger) error {
	db := built.pgHandle.DB
	asked := func(ctx context.Context) (bool, error) { return postgres.Superuser(ctx, db) }
	if err := superuserCheck(ctx, settings.superuser, asked, logger); err != nil {
		return err
	}
	return migrate(ctx, settings.databaseURL, db)
}

// superuserPolicyFrom reads what serve does when it connects as a superuser, warn when the setting names nothing.
func superuserPolicyFrom(env gonsole.Env) (superuserPolicy, error) {
	return gonsole.Parse(env, "SUPERUSER_POLICY", superuserWarn, parseSuperuserPolicy)
}

// parseSuperuserPolicy returns the policy text names, refusing any word but warn and refuse.
func parseSuperuserPolicy(text string) (superuserPolicy, error) {
	switch policy := superuserPolicy(text); policy {
	case superuserWarn, superuserRefuse:
		return policy, nil
	}
	return "", fmt.Errorf("must be warn or refuse, got %q", text)
}

// superuserCheck asks whether the site connects as a superuser, then warns or refuses as policy says.
func superuserCheck(
	ctx context.Context, policy superuserPolicy, asked func(context.Context) (bool, error), logger *slog.Logger,
) error {
	super, err := asked(ctx)
	switch {
	case err != nil:
		return err
	case !super:
		return nil
	case policy == superuserRefuse:
		return errSuperuser
	}
	logger.Warn(superuserWarning)
	return nil
}
