// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// groupedTypeStore answers the post type and the field groups it holds, recording what lands in them.
type groupedTypeStore struct {
	stubTypeStore
	groups    []content.Group
	declared  []content.Field
	groupsErr error
	raiseErr  error
}

// ListGroups returns the groups held, each carrying the fields declared into it, or the scripted failure.
func (s *groupedTypeStore) ListGroups(context.Context) ([]content.Group, error) {
	if s.groupsErr != nil {
		return nil, s.groupsErr
	}
	return slices.Clone(s.groups), nil
}

// CreateGroup holds the group turned on at the end of the order, or reports the scripted failure.
func (s *groupedTypeStore) CreateGroup(_ context.Context, g content.Group) (content.Group, error) {
	if s.raiseErr != nil {
		return content.Group{}, s.raiseErr
	}
	g.ID, g.Active = len(s.groups)+1, true
	s.groups = append(s.groups, g)
	return g, nil
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

func TestDeclareOnTypeRaisesTheTypesOwnGroupWhenNoneIsHeld(t *testing.T) {
	t.Parallel()

	types := &groupedTypeStore{}

	declared, err := declareOnType(t.Context(), content.NewRegistry(types), content.TypePost, FeaturedField())

	if err != nil {
		t.Fatalf("declareOnType() error = %v, want nil", err)
	}
	if len(types.groups) != 1 {
		t.Fatalf("the store holds %+v, want the one group raised", types.groups)
	}
	raised := types.groups[0]
	if raised.Key != "post-fields" || raised.Title != "Post fields" {
		t.Errorf("raised %s titled %q, want post-fields titled Post fields", raised.Key, raised.Title)
	}
	if !raised.Location.Equal(locationOn(content.TypePost)) {
		t.Errorf("raised a group reaching %v, want it reaching posts alone", raised.Location)
	}
	if len(declared) != 1 || declared[0].Key != FeaturedFieldKey || declared[0].GroupID != raised.ID {
		t.Errorf("declared %+v, want the switch in the raised group %d", declared, raised.ID)
	}
}

// categoryListingStore answers the post and category types beside the groups it holds.
type categoryListingStore struct {
	groupedTypeStore
}

// List returns the post type and the demo category type.
func (s *categoryListingStore) List(ctx context.Context) ([]content.Type, error) {
	types, err := s.groupedTypeStore.List(ctx)
	return append(types, CategoryType()), err
}

func TestDeclareOnTypeRaisesTheGroupOfTheTypeItNames(t *testing.T) {
	t.Parallel()

	types := &categoryListingStore{}
	summary := content.Field{Key: "summary", Label: "Summary", Kind: content.FieldKindText}

	if _, err := declareOnType(t.Context(), content.NewRegistry(types), CategoryTypeKey, summary); err != nil {
		t.Fatalf("declareOnType() error = %v, want nil", err)
	}

	if len(types.groups) != 1 || types.groups[0].Key != "category-fields" ||
		types.groups[0].Title != "Category fields" || !types.groups[0].Location.Equal(locationOn(CategoryTypeKey)) {
		t.Errorf("the store holds %+v, want category-fields titled Category fields reaching categories alone", types.groups)
	}
}

func TestDeclareOnTypeReusesTheTypesOwnGroupListedAfterAnother(t *testing.T) {
	t.Parallel()

	another := content.Group{
		ID: 2, Key: "another-group", Title: "Another group", Active: true, Location: locationOn(content.TypePost),
	}
	types := &groupedTypeStore{groups: []content.Group{another, postFieldsGroup(true, content.TypePost)}}

	declared, err := declareOnType(t.Context(), content.NewRegistry(types), content.TypePost, FeaturedField())

	if err != nil {
		t.Fatalf("declareOnType() error = %v, want nil", err)
	}
	if len(types.groups) != 2 {
		t.Errorf("the store holds %+v, want no group raised", types.groups)
	}
	if len(declared) != 1 || declared[0].GroupID != postFieldsGroup(true, content.TypePost).ID {
		t.Errorf("declared %+v, want the switch in post-fields", declared)
	}
}

func TestDeclareOnTypeReportsWhatStopsIt(t *testing.T) {
	t.Parallel()

	ownedByAPlugin := postFieldsGroup(true, content.TypePost)
	ownedByAPlugin.Origin = "a-plugin"
	tests := map[string]struct {
		types   *groupedTypeStore
		typeKey string
		want    error
	}{
		"the groups it cannot read": {&groupedTypeStore{groupsErr: errStub}, content.TypePost, errStub},
		"a type the registry lacks": {&groupedTypeStore{}, "unregistered", content.ErrTypeNotFound},
		"the group it cannot raise": {&groupedTypeStore{raiseErr: errStub}, content.TypePost, errStub},
		"a group a plugin declared": {
			&groupedTypeStore{groups: []content.Group{ownedByAPlugin}}, content.TypePost,
			content.ErrDefinitionReadOnly,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := len(tc.types.groups)

			_, err := declareOnType(t.Context(), content.NewRegistry(tc.types), tc.typeKey, FeaturedField())

			if !errors.Is(err, tc.want) {
				t.Errorf("declareOnType() error = %v, want %v", err, tc.want)
			}
			if len(tc.types.declared) != 0 || len(tc.types.groups) != held {
				t.Errorf("the store holds %+v in %d groups, want nothing declared or raised",
					tc.types.declared, len(tc.types.groups))
			}
		})
	}
}
