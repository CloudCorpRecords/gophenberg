// SPDX-License-Identifier: Apache-2.0

// Package columns holds the Go types SQLite columns read into and write from.
package columns

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/gopherium/gophenberg/internal/media"
)

// Sizes is a media item's sub-sizes stored as JSON text.
type Sizes media.Sizes

// Value returns the map as the JSON text json.Marshal writes, or no value for a nil map.
func (s Sizes) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	text, err := json.Marshal(media.Sizes(s))
	return string(text), err
}

// Scan replaces the map with the one the JSON text holds, nil for null.
func (s *Sizes) Scan(src any) error {
	text, ok := src.(string)
	if !ok {
		return fmt.Errorf("columns: the sizes read JSON text, got %T", src)
	}
	var read media.Sizes
	if err := json.Unmarshal([]byte(text), &read); err != nil {
		return fmt.Errorf("columns: read the sizes: %w", err)
	}
	*s = Sizes(read)
	return nil
}
