// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gopherium/gophenberg/internal/content/typestore"
)

func TestReadErrorReadsTheCodeAndTheConstraintAPostgreSQLErrorCarries(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		err        error
		code       string
		constraint string
	}{
		"a unique violation with its constraint": {
			&pgconn.PgError{Code: uniqueViolationCode, ConstraintName: fieldKeyConstraint},
			uniqueViolationCode, fieldKeyConstraint,
		},
		"a foreign key violation": {
			&pgconn.PgError{Code: foreignKeyViolationCode, ConstraintName: fieldTargetConstraint},
			foreignKeyViolationCode, fieldTargetConstraint,
		},
		"a restrict violation": {
			&pgconn.PgError{Code: restrictViolationCode, ConstraintName: "content_type_fkey"},
			restrictViolationCode, "content_type_fkey",
		},
		"a PostgreSQL error wrapped on the way out": {
			fmt.Errorf("retype content paths: %w",
				&pgconn.PgError{Code: uniqueViolationCode, ConstraintName: slugConstraint}),
			uniqueViolationCode, slugConstraint,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			code, constraint, found := readError(test.err)

			if code != test.code || constraint != test.constraint || !found {
				t.Errorf("readError() = (%q, %q, %t), want (%q, %q, true)",
					code, constraint, found, test.code, test.constraint)
			}
		})
	}
}

func TestReadErrorFindsNothingInAnErrorPostgreSQLDidNotRaise(t *testing.T) {
	t.Parallel()

	for name, err := range map[string]error{
		"a plain error": errors.New("the connection is gone"),
		"no error":      nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			code, constraint, found := readError(err)

			if code != "" || constraint != "" || found {
				t.Errorf("readError() = (%q, %q, %t), want no code, no constraint and nothing found",
					code, constraint, found)
			}
		})
	}
}

func TestRulesBrokenListsEachRuleThePostgreSQLErrorsNameInMatchOrder(t *testing.T) {
	t.Parallel()

	want := []struct {
		name string
		rule typestore.Rule
	}{
		{"content holding the type under any constraint", typestore.Rule{
			Code: restrictViolationCode, Broken: typestore.TypeHeldByContent,
		}},
		{"a content path taken under any code", typestore.Rule{
			Name: slugConstraint, Broken: typestore.ContentPathUnique,
		}},
		{"a group key taken", typestore.Rule{
			Code: uniqueViolationCode, Name: groupKeyConstraint, Broken: typestore.GroupKeyUnique,
		}},
		{"a field key taken in its group", typestore.Rule{
			Code: uniqueViolationCode, Name: fieldKeyConstraint, Broken: typestore.FieldKeyUnique,
		}},
		{"a field key taken in its container", typestore.Rule{
			Code: uniqueViolationCode, Name: containerKeyConstraint, Broken: typestore.FieldKeyUnique,
		}},
		{"a type key taken", typestore.Rule{
			Code: uniqueViolationCode, Name: typeKeyConstraint, Broken: typestore.TypeKeyUnique,
		}},
		{"a route word taken", typestore.Rule{
			Code: uniqueViolationCode, Name: routeWordConstraint, Broken: typestore.RouteWordUnique,
		}},
		{"a relation target reference broken", typestore.Rule{
			Code: foreignKeyViolationCode, Name: fieldTargetConstraint, Broken: typestore.FieldTargetExists,
		}},
		{"a field group reference broken", typestore.Rule{
			Code: foreignKeyViolationCode, Name: fieldGroupConstraint, Broken: typestore.FieldGroupExists,
		}},
	}

	if len(rulesBroken) != len(want) {
		t.Errorf("rulesBroken holds %d rules, want %d", len(rulesBroken), len(want))
	}
	for at, test := range want {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if at >= len(rulesBroken) {
				t.Fatalf("rulesBroken holds no rule at %d, want %+v", at, test.rule)
			}
			if rulesBroken[at] != test.rule {
				t.Errorf("rulesBroken[%d] = %+v, want %+v", at, rulesBroken[at], test.rule)
			}
		})
	}
}
