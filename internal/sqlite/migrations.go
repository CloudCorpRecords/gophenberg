// SPDX-License-Identifier: Apache-2.0

package sqlite

import "embed"

// migrations holds the core table migration files applied by goose.
//
//go:embed migrations/*.sql
var migrations embed.FS
