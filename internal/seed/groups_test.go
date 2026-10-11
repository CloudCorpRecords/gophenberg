// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// groupedTypeStore answers the post type and the field groups it holds, recording what lands in them.
type groupedTypeStore struct {
	stubTypeStore
	groups   []content.Group
	declared []content.Field
}

// ListGroups returns the groups held, each carrying the fields declared into it.
func (s *groupedTypeStore) ListGroups(context.Context) ([]content.Group, error) {
	return slices.Clone(s.groups), nil
}

// CreateFieldInGroup records the field in the group the identity names and hands it back.
func (s *groupedTypeStore) CreateFieldInGroup(
	_ context.Context, groupID int, f content.Field, _ content.Recheck,
) (content.Field, error) {
	f.ID, f.GroupID = len(s.declared)+1, groupID
	s.declared = append(s.declared, f)
	for i := range s.groups {
		if s.groups[i].ID == groupID {
			s.groups[i].Fields = append(s.groups[i].Fields, f)
		}
	}
	return f, nil
}

// CreateField records the field on its type and hands it back.
func (s *groupedTypeStore) CreateField(_ context.Context, f content.Field) (content.Field, error) {
	f.ID = len(s.declared) + 1
	s.declared = append(s.declared, f)
	return f, nil
}

// locationOn is the shared fixture under the name these tests use.
var locationOn = contenttest.LocationOf

// postFieldsGroup returns the post type's own field group, turned on or off and reaching the type named.
func postFieldsGroup(active bool, reaches string) content.Group {
	return content.Group{
		ID: 1, Key: "post-fields", Title: "Post fields", Active: active, Location: locationOn(reaches),
	}
}

func TestFeaturedLeavesAPostFieldsGroupThatNoLongerServesPostsAlone(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]content.Group{
		"turned off but still reaching posts": postFieldsGroup(false, content.TypePost),
		"turned on but moved to pages":        postFieldsGroup(true, PageTypeKey),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			types := &groupedTypeStore{groups: []content.Group{held}}

			err := Featured(t.Context(), content.NewRegistry(types))

			if err == nil || !strings.Contains(err.Error(), held.Key) {
				t.Errorf("Featured() error = %v, want a refusal naming %s", err, held.Key)
			}
			if len(types.declared) != 0 {
				t.Errorf("the store holds %+v, want nothing declared", types.declared)
			}
		})
	}
}
