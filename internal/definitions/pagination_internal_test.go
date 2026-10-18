// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestImportKeepsPageReservedUnlessTheTypeAlreadyUsesIt(t *testing.T) {
	t.Parallel()

	declaration := TypeDefinition{
		Key: "chapter", SingularLabel: "Chapter", PluralLabel: "Chapters",
		RouteWord: "page", PageKind: "single", Active: true,
	}
	for _, test := range []struct {
		name   string
		stored []content.Type
		want   error
	}{
		{name: "new type", want: content.ErrRouteWordReserved},
		{
			name: "changed route word", want: content.ErrRouteWordReserved,
			stored: []content.Type{{Key: "chapter", RouteWord: "chapters"}},
		},
		{
			name:   "unchanged legacy route word",
			stored: []content.Type{{Key: "chapter", RouteWord: "page"}},
		},
		{
			name: "another type uses page", want: content.ErrRouteWordReserved,
			stored: []content.Type{{Key: "other", RouteWord: "page"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateTypes([]TypeDefinition{declaration}, test.stored)
			if !errors.Is(err, test.want) {
				t.Errorf("validateTypes() error = %v, want %v", err, test.want)
			}
		})
	}
}
