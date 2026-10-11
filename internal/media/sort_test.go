// SPDX-License-Identifier: Apache-2.0

package media_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/media"
)

func TestParseOrderByAcceptsEverySortableColumn(t *testing.T) {
	t.Parallel()

	for _, want := range []media.OrderBy{
		media.OrderByDate, media.OrderByModified, media.OrderByTitle, media.OrderByAuthor,
	} {
		got, err := media.ParseOrderBy(string(want))

		if err != nil {
			t.Errorf("ParseOrderBy(%q) error = %v, want nil", want, err)
		}
		if got != want {
			t.Errorf("ParseOrderBy(%q) = %q, want %q", want, got, want)
		}
	}
}

func TestParseOrderByRejectsUnknownColumns(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "Date", "Title", "slug", "id", "parent"} {
		if _, err := media.ParseOrderBy(in); !errors.Is(err, media.ErrInvalidOrderBy) {
			t.Errorf("ParseOrderBy(%q) error = %v, want %v", in, err, media.ErrInvalidOrderBy)
		}
	}
}

func TestParseOrderAcceptsEveryDirection(t *testing.T) {
	t.Parallel()

	for _, want := range []media.Order{media.OrderAsc, media.OrderDesc} {
		got, err := media.ParseOrder(string(want))

		if err != nil {
			t.Errorf("ParseOrder(%q) error = %v, want nil", want, err)
		}
		if got != want {
			t.Errorf("ParseOrder(%q) = %q, want %q", want, got, want)
		}
	}
}

func TestParseOrderRejectsUnknownDirections(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "ASC", "ascending", "sideways", "1"} {
		if _, err := media.ParseOrder(in); !errors.Is(err, media.ErrInvalidOrder) {
			t.Errorf("ParseOrder(%q) error = %v, want %v", in, err, media.ErrInvalidOrder)
		}
	}
}
