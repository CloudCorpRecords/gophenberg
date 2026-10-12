// SPDX-License-Identifier: Apache-2.0

package typestore

import "github.com/gopherium/gophenberg/internal/content"

// Constraint names a stored rule a statement broke, whatever the engine calls that rule.
type Constraint int

// The stored rules the steps tell apart.
const (
	NoConstraint Constraint = iota
	GroupKeyUnique
	FieldKeyUnique
	FieldTargetExists
	FieldGroupExists
	TypeKeyUnique
	RouteWordUnique
	TypeHeldByContent
	ContentPathUnique
)

// Rule names the stored rule an engine's error code and rule name stand for, an empty one matching any.
type Rule struct {
	Code, Name string
	Broken     Constraint
}

// matches reports whether the rule holds the code and the name an engine read from a failure.
func (r Rule) matches(code, name string) bool {
	return (r.Code == "" || r.Code == code) && (r.Name == "" || r.Name == name)
}

// broken returns the stored rule the first rule matching what the engine reads from err names, or none.
func (s *Store) broken(err error) Constraint {
	code, name, found := s.backend.ReadError(err)
	if !found {
		return NoConstraint
	}
	for _, rule := range s.backend.Rules {
		if rule.matches(code, name) {
			return rule.Broken
		}
	}
	return NoConstraint
}

// errorFor returns the error the table holds for the stored rule err broke, or nil.
func (s *Store) errorFor(table map[Constraint]error, err error) error {
	return table[s.broken(err)]
}

// groupWriteErrors maps the rule a group write can break to the error it stands for.
var groupWriteErrors = map[Constraint]error{
	GroupKeyUnique: content.ErrGroupKeyTaken,
}

// fieldWriteErrors maps each rule a field write can break to the error it stands for.
var fieldWriteErrors = map[Constraint]error{
	FieldKeyUnique:    content.ErrFieldTaken,
	FieldTargetExists: content.ErrTargetUnknown,
	FieldGroupExists:  content.ErrGroupNotFound,
}

// typeWriteErrors maps each rule a type write can break to the error it stands for.
var typeWriteErrors = map[Constraint]error{
	TypeKeyUnique:   content.ErrTypeTaken,
	RouteWordUnique: content.ErrRouteWordTaken,
}

// typeUpdateErrors maps each rule a type update can break to the error it stands for.
var typeUpdateErrors = map[Constraint]error{
	TypeKeyUnique:     content.ErrTypeTaken,
	RouteWordUnique:   content.ErrRouteWordTaken,
	ContentPathUnique: content.ErrSlugTaken,
}

// typeDeleteErrors maps each rule a type delete can break to the error it stands for.
var typeDeleteErrors = map[Constraint]error{
	TypeHeldByContent: content.ErrTypeInUse,
	FieldTargetExists: content.ErrTypeTargeted,
}
