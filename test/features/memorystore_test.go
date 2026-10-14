// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/hex"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/authkit"
)

// memoryStore holds one scenario's accounts and sessions in memory.
type memoryStore struct {
	mu       sync.Mutex
	users    map[string]gouncer.User
	sessions map[string]gouncer.Session
	byIDs    int
}

// newMemoryStore returns an empty account store.
func newMemoryStore() *memoryStore {
	return &memoryStore{
		users:    make(map[string]gouncer.User),
		sessions: make(map[string]gouncer.Session),
	}
}

// CreateUser stores the account, refusing an email already taken.
func (m *memoryStore) CreateUser(_ context.Context, user gouncer.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, taken := m.users[user.Email]; taken {
		return gouncer.ErrEmailTaken
	}
	m.users[user.Email] = user
	return nil
}

// UserByEmail returns the account with the email.
func (m *memoryStore) UserByEmail(_ context.Context, email string) (gouncer.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, found := m.users[email]
	if !found {
		return gouncer.User{}, gouncer.ErrUserNotFound
	}
	return user, nil
}

// CreateSession stores the session.
func (m *memoryStore) CreateSession(_ context.Context, session gouncer.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[hex.EncodeToString(session.TokenHash)] = session
	return nil
}

// UserBySession returns the account owning an unexpired session.
func (m *memoryStore) UserBySession(_ context.Context, tokenHash []byte, now time.Time) (gouncer.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, found := m.sessions[hex.EncodeToString(tokenHash)]
	if !found || !session.ExpiresAt.After(now) {
		return gouncer.User{}, gouncer.ErrSessionNotFound
	}
	for _, user := range m.users {
		if user.ID == session.UserID {
			return user, nil
		}
	}
	return gouncer.User{}, gouncer.ErrSessionNotFound
}

// DeleteSession removes the session.
func (m *memoryStore) DeleteSession(_ context.Context, tokenHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, hex.EncodeToString(tokenHash))
	return nil
}

// nameOf returns the name of the account carrying the id, empty when none does.
func (m *memoryStore) nameOf(id uuid.UUID) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, user := range m.users {
		if user.ID == id {
			return user.Name
		}
	}
	return ""
}

// holdsUser reports whether an account carries the id.
func (m *memoryStore) holdsUser(id uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, user := range m.users {
		if user.ID == id {
			return true
		}
	}
	return false
}

// UsersByIDs returns the accounts carrying the ids in one read, counting the read.
func (m *memoryStore) UsersByIDs(_ context.Context, ids []uuid.UUID) ([]gouncer.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byIDs++
	var found []gouncer.User
	for _, user := range m.users {
		if slices.Contains(ids, user.ID) {
			found = append(found, user)
		}
	}
	return found, nil
}

// readsByIDs returns how many reads of many ids the store answered.
func (m *memoryStore) readsByIDs() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byIDs
}

// idNamed returns the id of the account carrying the name, and whether one does.
func (m *memoryStore) idNamed(name string) (uuid.UUID, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, user := range m.users {
		if user.Name == name {
			return user.ID, true
		}
	}
	return uuid.UUID{}, false
}

// ListUsers returns every account in email order.
func (m *memoryStore) ListUsers(_ context.Context) ([]gouncer.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	users := make([]gouncer.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Email < users[j].Email })
	return users, nil
}

// SetUserDisabled updates whether the account may log in.
func (m *memoryStore) SetUserDisabled(_ context.Context, id uuid.UUID, disabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for email, user := range m.users {
		if user.ID == id {
			user.Disabled = disabled
			m.users[email] = user
			return nil
		}
	}
	return gouncer.ErrUserNotFound
}

// SetUserDisabledUnderCover disables an account, refusing to leave no enabled account privileged.
func (m *memoryStore) SetUserDisabledUnderCover(
	_ context.Context, id uuid.UUID, disabled bool, privileged gouncer.Roles,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.refuseUncoveredLocked(id, privileged, disabled); err != nil {
		return err
	}
	return m.updateLocked(id, func(user *gouncer.User) { user.Disabled = disabled })
}

// SetUserRole writes an account's role, refusing to leave no enabled account privileged.
func (m *memoryStore) SetUserRole(
	_ context.Context, id uuid.UUID, role string, privileged gouncer.Roles,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.refuseUncoveredLocked(id, privileged, !privileged.Holds(role)); err != nil {
		return err
	}
	return m.updateLocked(id, func(user *gouncer.User) { user.Role = role })
}

// SetUserRoleFrom writes the role of an account still holding held, refusing to leave no enabled account privileged.
func (m *memoryStore) SetUserRoleFrom(
	_ context.Context, id uuid.UUID, held, role string, privileged gouncer.Roles,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.refuseMovedLocked(id, held); err != nil {
		return err
	}
	if err := m.refuseUncoveredLocked(id, privileged, !privileged.Holds(role)); err != nil {
		return err
	}
	return m.updateLocked(id, func(user *gouncer.User) { user.Role = role })
}

// SetUserDisabledFrom disables an account still holding held, refusing to leave no enabled account privileged.
func (m *memoryStore) SetUserDisabledFrom(
	_ context.Context, id uuid.UUID, held string, disabled bool, privileged gouncer.Roles,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.refuseMovedLocked(id, held); err != nil {
		return err
	}
	if err := m.refuseUncoveredLocked(id, privileged, disabled); err != nil {
		return err
	}
	return m.updateLocked(id, func(user *gouncer.User) { user.Disabled = disabled })
}

// refuseMovedLocked refuses an account no longer holding held, the caller holding the lock.
func (m *memoryStore) refuseMovedLocked(id uuid.UUID, held string) error {
	for _, user := range m.users {
		if user.ID != id {
			continue
		}
		if user.Role != held {
			return authkit.ErrRoleChanged
		}
		return nil
	}
	return gouncer.ErrUserNotFound
}

// updateLocked changes one stored account, the caller holding the lock.
func (m *memoryStore) updateLocked(id uuid.UUID, change func(*gouncer.User)) error {
	for email, user := range m.users {
		if user.ID == id {
			change(&user)
			m.users[email] = user
			return nil
		}
	}
	return gouncer.ErrUserNotFound
}

// refuseUncoveredLocked refuses a write leaving no enabled account privileged, the caller holding the lock.
func (m *memoryStore) refuseUncoveredLocked(id uuid.UUID, privileged gouncer.Roles, removesCover bool) error {
	if len(privileged) == 0 || !removesCover {
		return nil
	}
	held, cover := false, 0
	for _, user := range m.users {
		if user.Disabled || !privileged.Holds(user.Role) {
			continue
		}
		if user.ID == id {
			held = true
			continue
		}
		cover++
	}
	if held && cover == 0 {
		return gouncer.ErrLastPrivileged
	}
	return nil
}
