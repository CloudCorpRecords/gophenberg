// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestApplyCanEditALegacyPageTypeWithoutMovingIt(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	store := postgres.NewTypeStore(pool)
	legacy, err := content.NewType("chapter", "Chapter", "Chapters", "page")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), legacy); err != nil {
		t.Fatalf("storing legacy type: %v", err)
	}
	registry := content.NewRegistry(store)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == "chapter" {
			envelope.Types[i].PluralLabel = "Site chapters"
		}
	}
	compared(t, registry, envelope)
	applied(t, registry, importing(envelope))
	held, err := registry.ByKey(t.Context(), "chapter")
	if err != nil {
		t.Fatal(err)
	}
	if held.RouteWord != "page" || held.PluralLabel != "Site chapters" {
		t.Errorf("imported type = %+v, want the labels edited at the existing route", held)
	}
}

func TestDeclareTypeCanEditALegacyPageTypeWithoutMovingIt(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	store := postgres.NewTypeStore(pool)
	legacy, err := content.NewType("event", "Event", "Events", "page")
	if err != nil {
		t.Fatal(err)
	}
	legacy.Origin = "events"
	if _, err := store.Create(t.Context(), legacy); err != nil {
		t.Fatalf("storing legacy plugin type: %v", err)
	}
	registry := content.NewRegistry(store)
	declared := eventType()
	declared.RouteWord, declared.Description = "page", "Updated events"
	if err := definitions.New(registry, "events").DeclareType(t.Context(), declared); err != nil {
		t.Fatalf("DeclareType() error = %v, want the existing plugin type updated", err)
	}
	held, err := registry.ByKey(t.Context(), "event")
	if err != nil {
		t.Fatal(err)
	}
	if held.RouteWord != "page" || held.Description != "Updated events" {
		t.Errorf("declared type = %+v, want the plugin's changes at the existing route", held)
	}
}
