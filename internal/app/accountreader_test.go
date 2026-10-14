// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/framework/gonsole/testkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/gopherium/gouncer/avatar"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/seed"
	"github.com/gopherium/gophenberg/sdk"
)

// readSize is the avatar size the accounts plugin asks for.
const readSize = 64

// accountsPlugin reads the accounts it was asked for once the host starts it.
type accountsPlugin struct {
	accounts sdk.AccountReader
	asked    []sdk.ID
	people   map[sdk.ID]sdk.Person
}

// ID returns the plugin's identifier.
func (*accountsPlugin) ID() string {
	return "accounts"
}

// Start reads the asked accounts in one call.
func (p *accountsPlugin) Start(ctx context.Context) error {
	people, err := p.accounts.People(ctx, p.asked, readSize)
	p.people = people
	return err
}

// Stop stops nothing.
func (*accountsPlugin) Stop(context.Context) error {
	return nil
}

// seededIDs returns the ids the seed stored under the emails.
func seededIDs(t *testing.T, databaseURL string, emails ...string) []sdk.ID {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	users := authkitpg.NewUserStore(pool)
	ids := make([]sdk.ID, 0, len(emails))
	for _, email := range emails {
		user, err := users.UserByEmail(t.Context(), email)
		if err != nil {
			t.Fatalf("reading the seeded %s: %v", email, err)
		}
		ids = append(ids, sdk.ID(user.ID))
	}
	return ids
}

func TestAPluginReadsTheSeededAccountsByID(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL":   databaseURL,
		"GOPHENBERG_ADDR":           "localhost:0",
		"GOPHENBERG_WEB_DIR":        t.TempDir(),
		"GOPHENBERG_AVATARS":        "gravatar",
		"GOPHENBERG_AVATAR_DEFAULT": "identicon",
		"GOPHENBERG_AVATAR_RATING":  "PG",
	}
	if err := seedMigrated(t.Context(), testkit.Getenv(env), io.Discard); err != nil {
		t.Fatalf("seeding the demo data: %v", err)
	}
	ids := seededIDs(t, databaseURL, seed.AuthorEmail, seed.MemberEmail)
	reading := &accountsPlugin{asked: ids}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel}, func(deps sdk.Deps) ([]sdk.Plugin, error) {
		reading.accounts = deps.Accounts
		return []sdk.Plugin{reading}, nil
	})

	if err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
	options := avatar.Options{Mode: avatar.ModeGravatar, Default: avatar.DefaultIdenticon, Rating: avatar.RatingPG}
	want := map[sdk.ID]sdk.Person{
		ids[0]: {ID: ids[0], Name: seed.AuthorName, Role: role.Author,
			Avatar: avatar.Address(seed.AuthorName, seed.AuthorEmail, readSize, options)},
		ids[1]: {ID: ids[1], Name: seed.MemberName, Role: role.Member,
			Avatar: avatar.Address(seed.MemberName, seed.MemberEmail, readSize, options)},
	}
	if fmt.Sprint(reading.people) != fmt.Sprint(want) {
		t.Errorf("the plugin read %+v, want %+v", reading.people, want)
	}
}

func TestAPluginReadingAccountsWhileItRegistersIsRefused(t *testing.T) {
	t.Parallel()

	var refused error

	built, err := compose(t.Context(), composeConfig{databaseURL: unreachableDatabaseURL, fieldDepth: 4},
		func(deps sdk.Deps) ([]sdk.Plugin, error) {
			_, refused = deps.Accounts.People(t.Context(), []sdk.ID{sdk.ID(uuid.New())}, readSize)
			return []sdk.Plugin{}, nil
		})
	if err != nil {
		t.Fatalf("compose() error = %v, want nil", err)
	}
	defer built.close()

	if !errors.Is(refused, accountbridge.ErrNotOpen) {
		t.Errorf("People() during Register = %v, want ErrNotOpen", refused)
	}
}

func TestAPluginReadingAccountsEndsAtThePluginQueryTimeout(t *testing.T) {
	t.Parallel()

	var handed sdk.Deps
	built, err := compose(t.Context(), composeConfig{
		databaseURL: emptyDatabaseURL(t), fieldDepth: 4,
		getenv: testkit.Getenv(map[string]string{"GOPHENBERG_PLUGIN_QUERY_TIMEOUT": "1ns"}),
	}, func(deps sdk.Deps) ([]sdk.Plugin, error) {
		handed = deps
		return []sdk.Plugin{}, nil
	})
	if err != nil {
		t.Fatalf("compose() error = %v, want nil", err)
	}
	defer built.close()

	_, err = handed.Accounts.People(t.Context(), []sdk.ID{sdk.ID(uuid.New())}, readSize)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("People() under a 1ns plugin query timeout = %v, want the read ended at the timeout", err)
	}
}
