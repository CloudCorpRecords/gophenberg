// SPDX-License-Identifier: Apache-2.0

// Package accountbridge serves accounts to plugins over the sdk seam, never their email addresses.
package accountbridge

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/sdk"
)

var _ sdk.AccountReader = (*Reader)(nil)

// largestAvatar is the widest avatar in pixels an avatar service draws.
const largestAvatar = 2048

// ErrNotOpen reports a read made before every plugin registered.
var ErrNotOpen = errors.New("accountbridge: accounts are read from Start on, once every plugin registered")

// ErrAvatarSize reports an avatar size no avatar service draws.
var ErrAvatarSize = errors.New("accountbridge: avatar size out of range")

// Users reads many accounts by id in one call.
type Users interface {
	UsersByIDs(ctx context.Context, ids []uuid.UUID) ([]gouncer.User, error)
}

// Options is how the reader draws avatars and how long one read may take.
type Options struct {
	Avatar  func(name, email string, size int) string
	Timeout time.Duration
}

// Reader serves accounts to plugins, refusing until the site opens it.
type Reader struct {
	sdk.HostOnly
	users   Users
	options Options
	open    atomic.Bool
}

// New returns a closed reader over users.
func New(users Users, options Options) *Reader {
	return &Reader{users: users, options: options}
}

// Open lets plugins read, once every plugin registered.
func (r *Reader) Open() {
	r.open.Store(true)
}

// People returns the confirmed accounts holding ids, keyed by id, with an avatar address and never an email.
func (r *Reader) People(ctx context.Context, ids []sdk.ID, avatarSize int) (map[sdk.ID]sdk.Person, error) {
	if !r.open.Load() {
		return nil, ErrNotOpen
	}
	if avatarSize < 1 || avatarSize > largestAvatar {
		return nil, fmt.Errorf("%w: %d pixels, want 1 to %d", ErrAvatarSize, avatarSize, largestAvatar)
	}
	asked := distinct(ids)
	if len(asked) == 0 {
		return map[sdk.ID]sdk.Person{}, nil
	}
	read, cancel := context.WithTimeout(ctx, r.options.Timeout)
	defer cancel()
	users, err := r.users.UsersByIDs(read, asked)
	if err != nil {
		return nil, fmt.Errorf("accountbridge: read accounts: %w", err)
	}
	return r.peopleOf(users, avatarSize), nil
}

// peopleOf returns the confirmed accounts among users as plugins see them, keyed by id.
func (r *Reader) peopleOf(users []gouncer.User, avatarSize int) map[sdk.ID]sdk.Person {
	people := make(map[sdk.ID]sdk.Person, len(users))
	for _, user := range users {
		if !user.Confirmed {
			continue
		}
		people[sdk.ID(user.ID)] = sdk.Person{
			ID:       sdk.ID(user.ID),
			Name:     user.Name,
			Role:     user.Role,
			Avatar:   r.options.Avatar(user.Name, user.Email, avatarSize),
			Disabled: user.Disabled,
		}
	}
	return people
}

// distinct returns the ids as account ids, each once, in the order first asked.
func distinct(ids []sdk.ID) []uuid.UUID {
	seen := make(map[sdk.ID]bool, len(ids))
	asked := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		asked = append(asked, uuid.UUID(id))
	}
	return asked
}
