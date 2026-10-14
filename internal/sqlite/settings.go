// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// SettingStore meets the site setting contract.
var _ content.SiteSettingStore = (*SettingStore)(nil)

// SettingStore persists the values a site chooses for itself in the core tables of its SQLite file.
type SettingStore struct {
	handle  *sql.DB
	queries *db.Queries
}

// NewSettingStore returns a [SettingStore] on the site's one SQLite handle.
func NewSettingStore(handle *sql.DB) *SettingStore {
	return &SettingStore{handle: handle, queries: db.New(handle)}
}

// Lookup returns the value stored under key and whether the key is set at all.
func (s *SettingStore) Lookup(ctx context.Context, key string) (string, bool, error) {
	value, err := s.queries.GetSetting(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, failed(fmt.Sprintf("reading setting %q", key), err)
	}
	return value, true, nil
}

// Save stores every given value, or stores none of them, and writes nothing when given no value.
func (s *SettingStore) Save(ctx context.Context, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	return failed("writing settings", inTx(ctx, s.handle, func(tx *sql.Tx) error {
		return writeSettings(ctx, s.queries.WithTx(tx), values)
	}))
}

// writeSettings upserts each value through queries in key order and names the key of the first write that fails.
func writeSettings(ctx context.Context, queries *db.Queries, values map[string]string) error {
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if err := queries.SetSetting(ctx, db.SetSettingParams{Key: key, Value: values[key]}); err != nil {
			return fmt.Errorf("%q: %w", key, dbsqlite.Classify(err))
		}
	}
	return nil
}

// failed returns nil for a nil err, or err under the sqlite prefix and action, marked with its dbkit class.
func failed(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("sqlite: %s: %w", action, dbsqlite.Classify(err))
}
