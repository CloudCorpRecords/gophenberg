// SPDX-License-Identifier: Apache-2.0

package media

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrConflict reports that the media item changed after the update was prepared.
var ErrConflict = errors.New("media: conflicting update")

// ErrInvalidOrderBy reports that a sort column is not one the library sorts by.
var ErrInvalidOrderBy = errors.New("media: invalid orderby")

// ErrInvalidOrder reports that a sort direction is not one the library sorts in.
var ErrInvalidOrder = errors.New("media: invalid order")

// OrderBy names a column a media listing can be sorted by.
type OrderBy string

// The columns a media listing can be sorted by.
const (
	OrderByDate     OrderBy = "date"
	OrderByModified OrderBy = "modified"
	OrderByTitle    OrderBy = "title"
	OrderByAuthor   OrderBy = "author"
)

// Order names a sort direction.
type Order string

// The directions a media listing can be sorted in.
const (
	OrderAsc  Order = "asc"
	OrderDesc Order = "desc"
)

// ParseOrderBy returns the column named by raw, or [ErrInvalidOrderBy].
func ParseOrderBy(raw string) (OrderBy, error) {
	switch OrderBy(raw) {
	case OrderByDate, OrderByModified, OrderByTitle, OrderByAuthor:
		return OrderBy(raw), nil
	default:
		return "", ErrInvalidOrderBy
	}
}

// ParseOrder returns the direction named by raw, or [ErrInvalidOrder].
func ParseOrder(raw string) (Order, error) {
	switch Order(raw) {
	case OrderAsc, OrderDesc:
		return Order(raw), nil
	default:
		return "", ErrInvalidOrder
	}
}

// Filter narrows a media listing by type, content type prefix, author, date and search, sorted as it names.
type Filter struct {
	Type           Type
	Mimes          []string
	Authors        []uuid.UUID
	ExcludeAuthors []uuid.UUID
	Before         *time.Time
	After          *time.Time
	ModifiedBefore *time.Time
	ModifiedAfter  *time.Time
	Search         string
	OrderBy        OrderBy
	Order          Order
	Page           int
	PerPage        int
}

// Store persists the media library.
type Store interface {
	Create(ctx context.Context, m Media) (Media, error)
	ByID(ctx context.Context, id int64) (Media, error)
	ByIDs(ctx context.Context, ids []int64) ([]Media, error)
	List(ctx context.Context, f Filter) ([]Listed, int, error)
	Update(ctx context.Context, m Media, expectedUpdatedAt time.Time) (Media, error)
	Delete(ctx context.Context, id int64) (Media, error)
}
