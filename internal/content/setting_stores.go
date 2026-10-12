// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"

	"github.com/google/uuid"
)

// SiteSettingStore keeps the values a site chooses for itself, and saves every value of a write or none.
type SiteSettingStore interface {
	Lookup(ctx context.Context, key string) (string, bool, error)
	Save(ctx context.Context, values map[string]string) error
}

// UserSettingStore keeps the values each reader chooses for themselves.
type UserSettingStore interface {
	Lookup(ctx context.Context, userID uuid.UUID, key string) (string, bool, error)
	Save(ctx context.Context, userID uuid.UUID, key, value string) error
}
