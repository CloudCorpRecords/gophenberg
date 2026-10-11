// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// groupedStore is a fake exposing the groups the test hands it.
type groupedStore struct {
	*fakeTypeStore
	groups []content.Group
}

// ListGroups returns the held groups.
func (s *groupedStore) ListGroups(context.Context) ([]content.Group, error) {
	return s.groups, nil
}

func TestRegistryKeepsATypeARelationInsideASectionTargets(t *testing.T) {
	t.Parallel()

	store := &groupedStore{fakeTypeStore: newFakeTypeStore(), groups: []content.Group{{
		ID: 1, Title: "Extras", Active: true,
		Fields: []content.Field{{
			Key: "specs", Label: "Specs", Kind: content.FieldKindSection,
			Fields: []content.Field{{
				Key: "cars", Label: "Cars", Kind: content.FieldKindRelation, RelatesTo: "car", Many: true,
			}},
		}},
	}}}
	registry := content.NewRegistry(store)
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}

	err := registry.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeTargeted) {
		t.Fatalf("Delete() error = %v, want the relation inside the section still guarding", err)
	}
}

func TestUntargetedNamesTheRelationAndTheGroupHoldingIt(t *testing.T) {
	t.Parallel()

	groups := []content.Group{{Title: "Extras", Fields: []content.Field{{
		Key: "specs", Kind: content.FieldKindSection,
		Fields: []content.Field{{Key: "cars", Kind: content.FieldKindRelation, RelatesTo: "car"}},
	}}}}

	err := content.Untargeted(groups, "car")

	var refused *content.Error
	if !errors.As(err, &refused) || refused.Held["field"] != "cars" || refused.Held["group"] != "Extras" {
		t.Errorf("Untargeted(car) error = %v, want the relation inside the section and its group named", err)
	}
	if err := content.Untargeted(groups, "van"); err != nil {
		t.Errorf("Untargeted(van) error = %v, want nil for a type nothing points at", err)
	}
}

func TestRegistryRefusesDeletingATypeARestingGroupStillTargets(t *testing.T) {
	t.Parallel()

	store := &groupedStore{fakeTypeStore: newFakeTypeStore(), groups: []content.Group{{
		ID: 1, Title: "Resting extras", Active: false,
		Fields: []content.Field{{
			Key: "cars", Label: "Cars", Kind: content.FieldKindRelation, RelatesTo: "car", Many: true,
		}},
	}}}
	registry := content.NewRegistry(store)
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}

	err := registry.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeTargeted) {
		t.Fatalf("Delete() error = %v, want the inactive group's relation still guarding", err)
	}
	var refused *content.Error
	if !errors.As(err, &refused) || refused.Held["group"] != "Resting extras" {
		t.Errorf("details = %v, want the guarding group named", err)
	}
}

func TestRegistryDeletesATypeNoRelationTargets(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}

	if err := registry.Delete(t.Context(), "car"); err != nil {
		t.Fatalf("Delete() error = %v, want the untargeted type released", err)
	}

	if _, err := registry.ByKey(t.Context(), "car"); !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ByKey() error = %v, want the type gone from the registry", err)
	}
}

func TestRegistryHoldsATypeItsOwnGroupStillTargets(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}
	relationOnCars(t, registry, groupNaming(t, registry, "Car fields", namingType("car")), "sibling", "car")

	err := registry.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeTargeted) {
		t.Fatalf("Delete() error = %v, want the surviving relation holding the type", err)
	}
}

// threeFieldCar returns a registry holding the car type and the id of its group with three text fields in order.
func threeFieldCar(t *testing.T) (*content.Registry, int) {
	t.Helper()
	registry := content.NewRegistry(newGroupingStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}
	held := groupNaming(t, registry, "Car fields", namingType("car"))
	for _, key := range []string{"color", "engine", "doors"} {
		if _, err := registry.CreateFieldInGroup(t.Context(), held.ID, content.Field{
			Key: key, Label: key, Kind: content.FieldKindText,
		}); err != nil {
			t.Fatalf("declaring %q: %v, want nil", key, err)
		}
	}
	return registry, held.ID
}

// fieldKeysOf returns the keys of the fields the group holds at its top in declared order.
func fieldKeysOf(t *testing.T, registry *content.Registry, groupID int) []string {
	t.Helper()
	held := topFieldsOf(t, registry, groupID)
	keys := make([]string, len(held))
	for i, f := range held {
		keys[i] = f.Key
	}
	return keys
}

func TestRegistryRefusesAReorderNamingAStranger(t *testing.T) {
	t.Parallel()

	registry, groupID := threeFieldCar(t)

	_, err := registry.ReorderFieldsInGroup(t.Context(), groupID, []string{"color", "engine", "finish"})

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want %v", err, content.ErrFieldNotFound)
	}
	if got := fieldKeysOf(t, registry, groupID); !slices.Equal(got, []string{"color", "engine", "doors"}) {
		t.Errorf("fields after the refused write = %v, want the declared order kept", got)
	}
}

func TestRegistryRefusesAReorderLeavingAFieldOut(t *testing.T) {
	t.Parallel()

	registry, groupID := threeFieldCar(t)

	_, err := registry.ReorderFieldsInGroup(t.Context(), groupID, []string{"doors", "color"})

	if !errors.Is(err, content.ErrFieldOrder) {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want %v", err, content.ErrFieldOrder)
	}
	if code, _ := content.CodeOf(err); code != "field_order_incomplete" {
		t.Errorf("code = %q, want field_order_incomplete", code)
	}
}

func TestRegistryRefusesAReorderRepeatingAField(t *testing.T) {
	t.Parallel()

	registry, groupID := threeFieldCar(t)

	_, err := registry.ReorderFieldsInGroup(t.Context(), groupID, []string{"doors", "color", "color"})

	if !errors.Is(err, content.ErrFieldOrder) {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want %v", err, content.ErrFieldOrder)
	}
}

// AdoptType accepts the adoption without changing what the store holds.
func (s *groupedStore) AdoptType(context.Context, string) error {
	return nil
}

// AdoptGroup accepts the adoption without changing what the store holds.
func (s *groupedStore) AdoptGroup(context.Context, string) error {
	return nil
}
