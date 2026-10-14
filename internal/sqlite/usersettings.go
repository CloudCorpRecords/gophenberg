// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// UserSettingStore meets the user setting contract.
var _ content.UserSettingStore = (*UserSettingStore)(nil)

// UserSettingStore persists one reader's own preferences in the core tables of the site's SQLite file.
type UserSettingStore struct {
	queries *db.Queries
}

// NewUserSettingStore returns a [UserSettingStore] on the site's one SQLite handle.
func NewUserSettingStore(handle *sql.DB) *UserSettingStore {
	return &UserSettingStore{queries: db.New(handle)}
}

// Lookup returns the value the reader stored under key, and whether it is set at all.
func (s *UserSettingStore) Lookup(
	ctx context.Context, userID uuid.UUID, key string,
) (string, bool, error) {
	value, err := s.queries.GetUserSetting(ctx, db.GetUserSettingParams{UserID: userID, Key: key})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, failed(fmt.Sprintf("reading the setting %q", key), err)
	}
	return value, true, nil
}

// Save stores the value under the reader's key.
func (s *UserSettingStore) Save(ctx context.Context, userID uuid.UUID, key, value string) error {
	err := s.queries.SetUserSetting(ctx, db.SetUserSettingParams{UserID: userID, Key: key, Value: value})
	if err != nil {
		return failed(fmt.Sprintf("writing the setting %q", key), err)
	}
	return nil
}
