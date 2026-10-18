// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestResolveKeepsRootPaginationAheadOfLegacyItems(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold("page", "page/2")
	resolver := newResolver(t, store)
	answer, err := resolver.Resolve(t.Context(), "/page/2")
	if err != nil {
		t.Fatal(err)
	}
	if answer.Kind != content.KindArchive || answer.Type.Key != "post" || answer.Page != 2 {
		t.Errorf("Resolve() = %+v, want the second page of the root listing", answer)
	}
	store.hold("page", "pages/page/2")
	nested, err := resolver.Resolve(t.Context(), "/pages/page/2")
	if err != nil {
		t.Fatal(err)
	}
	if nested.Kind != content.KindItem || nested.Item.Path != "pages/page/2" {
		t.Errorf("Resolve() = %+v, want the nested item to keep its address", nested)
	}
}

func TestContentKeepsRootPaginationAddressesReserved(t *testing.T) {
	t.Parallel()

	root := postType()
	root.Hierarchical = true
	parent := mustNew(t, root, nil, "Page")
	for _, slug := range []string{"1", "2", "42", "02"} {
		t.Run(slug, func(t *testing.T) {
			_, created := content.New(root, &parent, slug, uuid.Must(uuid.NewV7()))
			child := mustNew(t, root, &parent, "About")
			_, renamed := child.Rename(slug)
			item := mustNew(t, root, nil, slug)
			_, moved := content.Reparent(root, item, &parent, 1)
			for action, err := range map[string]error{"create": created, "rename": renamed, "move": moved} {
				if !errors.Is(err, content.ErrReservedAddress) {
					t.Errorf("%s page/%s error = %v, want %v", action, slug, err, content.ErrReservedAddress)
				}
			}
		})
	}
}

func TestContentAllowsPageAddressesOutsideRootPagination(t *testing.T) {
	t.Parallel()

	root := postType()
	root.Hierarchical = true
	page := mustNew(t, root, nil, "Page")
	if child := mustNew(t, root, &page, "About"); child.Path != "page/about" {
		t.Errorf("path = %q, want page/about", child.Path)
	}
	for _, slug := range []string{"0", "2nd"} {
		mustNew(t, root, &page, slug)
	}
	nested := mustNew(t, pageType(), nil, "Page")
	if child := mustNew(t, pageType(), &nested, "2"); child.Path != "pages/page/2" {
		t.Errorf("path = %q, want pages/page/2", child.Path)
	}
}

func TestLegacyPageTypeCannotTakeRootPaginationAddresses(t *testing.T) {
	t.Parallel()

	legacy := pageType()
	legacy.RouteWord = "page"
	_, err := content.New(legacy, nil, "2", uuid.Must(uuid.NewV7()))
	if !errors.Is(err, content.ErrReservedAddress) {
		t.Errorf("New() error = %v, want %v", err, content.ErrReservedAddress)
	}
	if item := mustNew(t, legacy, nil, "About"); item.Path != "page/about" {
		t.Errorf("path = %q, want page/about", item.Path)
	}
}

func TestRegistryReservesPageForNewRouteWords(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	car := carType(t)
	store.types = append(store.types, car)
	registry := content.NewRegistry(store)
	car.RouteWord = "page"
	_, created := registry.Create(t.Context(), car)
	_, updated := registry.Update(t.Context(), car)
	for action, err := range map[string]error{"create": created, "update": updated} {
		if !errors.Is(err, content.ErrRouteWordReserved) {
			t.Errorf("%s page route word error = %v, want %v", action, err, content.ErrRouteWordReserved)
		}
	}
}

func TestRegistryKeepsALegacyPageRouteWordUntilItChanges(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	legacy := carType(t)
	legacy.RouteWord = "page"
	store.types = append(store.types, legacy)
	registry := content.NewRegistry(store)
	legacy.Description = "Updated description"
	updated, err := registry.Update(t.Context(), legacy)
	if err != nil {
		t.Fatalf("Update() error = %v, want the unchanged legacy route word kept", err)
	}
	if updated.RouteWord != "page" || updated.Description != legacy.Description {
		t.Errorf("Update() = %+v, want the description edited without moving the type", updated)
	}
	legacy.RouteWord = "cars"
	if _, err := registry.Update(t.Context(), legacy); err != nil {
		t.Fatalf("Update() away from page error = %v, want nil", err)
	}
	legacy.RouteWord = "page"
	if _, err := registry.Update(t.Context(), legacy); !errors.Is(err, content.ErrRouteWordReserved) {
		t.Errorf("Update() back to page error = %v, want %v", err, content.ErrRouteWordReserved)
	}
}
