// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/authkit/testkit"
)

// memoryStore holds one scenario's accounts, sessions and tokens in gouncer's in-memory store, counting reads by id.
type memoryStore struct {
	*testkit.Store
	byIDs atomic.Int64
}

// newMemoryStore returns an empty account store.
func newMemoryStore() *memoryStore {
	return &memoryStore{Store: testkit.NewStore()}
}

// UsersByIDs returns the accounts carrying the ids in one read, counting the read.
func (m *memoryStore) UsersByIDs(ctx context.Context, ids []uuid.UUID) ([]gouncer.User, error) {
	m.byIDs.Add(1)
	return m.Store.UsersByIDs(ctx, ids)
}

// readsByIDs returns how many reads of many ids the store answered.
func (m *memoryStore) readsByIDs() int {
	return int(m.byIDs.Load())
}

// nameOf returns the name of the account carrying the id, empty when none does.
func (m *memoryStore) nameOf(id uuid.UUID) string {
	user, err := m.UserByID(context.Background(), id)
	if err != nil {
		return ""
	}
	return user.Name
}

// holdsUser reports whether an account carries the id.
func (m *memoryStore) holdsUser(id uuid.UUID) bool {
	_, err := m.UserByID(context.Background(), id)
	return err == nil
}

// idNamed returns the id of the account carrying the name, and whether one does.
func (m *memoryStore) idNamed(name string) (uuid.UUID, bool) {
	users, _ := m.ListUsers(context.Background())
	for _, user := range users {
		if user.Name == name {
			return user.ID, true
		}
	}
	return uuid.UUID{}, false
}
