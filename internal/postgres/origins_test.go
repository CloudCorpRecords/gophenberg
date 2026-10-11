// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestMigrationKeysExistingGroupsFromTheirTitles(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	url := pool.Config().ConnString()
	if err := postgres.MigrateDownTo(t.Context(), url, 18); err != nil {
		t.Fatalf("MigrateDownTo(18) error = %v, want nil", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO core.field_groups (title, location, position, active, created_at, updated_at)
		VALUES ('Details', '[]', 1, true, now(), now()),
		       ('Details', '[]', 2, true, now(), now()),
		       ('!!!', '[]', 3, true, now(), now()),
		       ('2024 plans', '[]', 4, true, now(), now()),
		       ('Maria''s picks', '[]', 5, true, now(), now())`,
	); err != nil {
		t.Fatalf("seeding the pre migration groups: %v, want nil", err)
	}

	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	want := []string{"details", "details-2", "untitled", "group-2024-plans", "marias-picks"}
	if len(groups) != len(want) {
		t.Fatalf("ListGroups() holds %d groups, want %d", len(groups), len(want))
	}
	for i, group := range groups {
		if group.Key != want[i] {
			t.Errorf("groups[%d].Key = %q, want %q", i, group.Key, want[i])
		}
		if group.Origin != "" {
			t.Errorf("groups[%d].Origin = %q, want none for a group that predates plugins", i, group.Origin)
		}
	}
}

func TestSiteDefinitionsStoreNoOriginInEachTable(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	trim := declaredInside(t, store, specs, "trim", content.FieldKindText)

	for _, row := range []struct {
		name, query string
		key         any
	}{
		{"the type", `SELECT origin IS NULL FROM core.content_types WHERE key = $1`, "car"},
		{"the group", `SELECT origin IS NULL FROM core.field_groups WHERE id = $1`, specs.GroupID},
		{"the field", `SELECT origin IS NULL FROM core.content_fields WHERE id = $1`, specs.ID},
		{"the sub field", `SELECT origin IS NULL FROM core.content_fields WHERE id = $1`, trim.ID},
	} {
		var none bool
		if err := pool.QueryRow(t.Context(), row.query, row.key).Scan(&none); err != nil {
			t.Fatalf("reading the origin of %s: %v, want nil", row.name, err)
		}
		if !none {
			t.Errorf("origin IS NULL on %s = false, want true for a definition the site made", row.name)
		}
	}
}
