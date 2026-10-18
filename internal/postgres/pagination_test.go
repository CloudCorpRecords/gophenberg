// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// rootChild stores a child of the root post type, which the caller has made hierarchical.
func rootChild(
	t *testing.T, store content.Store, parent *content.Content, title string, author uuid.UUID,
) content.Content {
	t.Helper()
	root := contenttest.PostType()
	root.Hierarchical = true
	built, err := content.New(root, parent, title, author)
	if err != nil {
		t.Fatal(err)
	}
	held, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatal(err)
	}
	return held
}

func TestAncestorMovesCannotCreateRootPaginationAddresses(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"rename", "reparent"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			store, author, pool := newContentStoreWithPool(t)
			types := postgres.NewTypeStore(pool)
			root := contenttest.PostType()
			root.Hierarchical = true
			if _, err := types.Update(t.Context(), root); err != nil {
				t.Fatal(err)
			}
			parent := rootChild(t, store, nil, "Chapter", author)
			if action == "reparent" {
				parent = rootChild(t, store, &parent, "Page", author)
			}
			child := rootChild(t, store, &parent, "2", author)
			moved := parent.Place("", "page")
			moved.ParentID, moved.UpdatedAt = nil, time.Now().UTC()
			_, err := store.Update(t.Context(), moved, parent.UpdatedAt, nil, 0)
			if !errors.Is(err, content.ErrReservedAddress) {
				t.Fatalf("Update() error = %v, want %v", err, content.ErrReservedAddress)
			}
			for _, before := range []content.Content{parent, child} {
				after, err := store.ByID(t.Context(), before.ID)
				if err != nil {
					t.Fatal(err)
				}
				if after.Path != before.Path || !after.UpdatedAt.Equal(before.UpdatedAt) {
					t.Errorf("stored item = %+v, want the refused move rolled back", after)
				}
			}
		})
	}
}

func TestTypePromotionCannotCreateRootPaginationAddresses(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	contenttest.RegisterPageType(t, types)
	rootBefore, err := types.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatal(err)
	}
	parent := contenttest.MustNest(t, store, nil, "Page", author)
	child := contenttest.MustNest(t, store, &parent, "2", author)
	promoted := contenttest.PageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()
	_, err = content.NewRegistry(types).Update(t.Context(), promoted)
	if !errors.Is(err, content.ErrReservedAddress) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrReservedAddress)
	}
	held, err := types.ByKey(t.Context(), "page")
	if err != nil {
		t.Fatal(err)
	}
	if held.Default || held.RouteWord != "pages" {
		t.Errorf("type = %+v, want the promotion rolled back", held)
	}
	rootAfter, err := types.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatal(err)
	}
	if !rootAfter.Default || rootAfter.RouteWord != "" || !rootAfter.UpdatedAt.Equal(rootBefore.UpdatedAt) {
		t.Errorf("root type = %+v, want its route, default flag and timestamp kept", rootAfter)
	}
	for _, before := range []content.Content{parent, child} {
		after, err := store.ByID(t.Context(), before.ID)
		if err != nil {
			t.Fatal(err)
		}
		if after.Path != before.Path || !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Errorf("item = %+v, want its address and timestamp kept after rollback", after)
		}
	}
}

func TestStaleLegacyPageTypeCannotReclaimItsReservedRoute(t *testing.T) {
	t.Parallel()

	pool := migratedPool(t)
	store := postgres.NewTypeStore(pool)
	legacy, err := content.NewType("chapter", "Chapter", "Chapters", "page")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), legacy); err != nil {
		t.Fatal(err)
	}
	stale := content.NewRegistry(store)
	if _, err := stale.ByKey(t.Context(), "chapter"); err != nil {
		t.Fatal(err)
	}
	moved := legacy
	moved.RouteWord = "chapters"
	if _, err := content.NewRegistry(store).Update(t.Context(), moved); err != nil {
		t.Fatal(err)
	}
	legacy.Description = "An edit prepared before the move"
	_, err = stale.Update(t.Context(), legacy)
	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Errorf("stale Update() error = %v, want %v", err, content.ErrRouteWordReserved)
	}
	stored, err := store.ByKey(t.Context(), "chapter")
	if err != nil {
		t.Fatal(err)
	}
	if stored.RouteWord != "chapters" {
		t.Errorf("route word = %q, want the completed move kept", stored.RouteWord)
	}
}
