// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestPaginationAddressReservedNamesTheAddress(t *testing.T) {
	t.Parallel()

	err := content.PaginationAddressReserved("page/2")

	if !errors.Is(err, content.ErrReservedAddress) {
		t.Errorf("error = %v, want it to carry %v", err, content.ErrReservedAddress)
	}
	if code, _ := content.CodeOf(err); code != "pagination_address_reserved" {
		t.Errorf("code = %q, want pagination_address_reserved", code)
	}
	if held, _ := content.DetailsOf(err); held["path"] != "page/2" {
		t.Errorf("details = %v, want the path page/2 named", held)
	}
	if !strings.Contains(err.Error(), `"page/2"`) {
		t.Errorf("message = %q, want it to name the address", err.Error())
	}
}

func TestLabelRouteWordReservedNamesTheLabelAndWord(t *testing.T) {
	t.Parallel()

	err := content.LabelRouteWordReserved(content.Type{PluralLabel: "Page", RouteWord: "page"})

	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Errorf("error = %v, want it to carry %v", err, content.ErrRouteWordReserved)
	}
	if code, _ := content.CodeOf(err); code != "route_word_reserved_by_label" {
		t.Errorf("code = %q, want route_word_reserved_by_label", code)
	}
	if held, _ := content.DetailsOf(err); held["label"] != "Page" || held["word"] != "page" {
		t.Errorf("details = %v, want the label Page and the word page named", held)
	}
	if !strings.Contains(err.Error(), `"Page"`) || !strings.Contains(err.Error(), `"page"`) {
		t.Errorf("message = %q, want it to name the label and the word", err.Error())
	}
}
