// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// eastOfUTC is a fixed zone two hours ahead of UTC.
var eastOfUTC = time.FixedZone("UTC+2", 2*60*60)

// storedCreated is the creation time a stored row holds, read in a zone other than UTC.
var storedCreated = time.Date(2026, time.March, 14, 9, 30, 0, 0, eastOfUTC)

// storedUpdated is the update time a stored row holds, read in a zone other than UTC.
var storedUpdated = time.Date(2026, time.March, 15, 18, 5, 0, 0, eastOfUTC)

// bookOnly is the location serving the book type alone.
var bookOnly = content.Rules{{{Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "book"}}}

// checkUTC fails the test when the named time is not in UTC.
func checkUTC(t *testing.T, name string, got time.Time) {
	t.Helper()
	if got.Location() != time.UTC {
		t.Errorf("%s location = %v, want UTC", name, got.Location())
	}
}

func TestGroupFromRowReadsEveryColumnWithTimesInUTC(t *testing.T) {
	t.Parallel()

	row := GroupRow{
		ID: 7, Position: 2, Key: "book-details", Title: "Book details", Origin: "library",
		Location:  []byte(`[[{"source":"content_type","operator":"==","value":"book"}]]`),
		Active:    true,
		CreatedAt: storedCreated, UpdatedAt: storedUpdated,
	}

	got, err := GroupFromRow(dbkit.Postgres, row)
	if err != nil {
		t.Fatalf("GroupFromRow() error = %v, want the row read", err)
	}

	want := content.Group{
		ID: 7, Key: "book-details", Title: "Book details", Location: bookOnly, Position: 2,
		Active: true, Origin: "library", CreatedAt: storedCreated.UTC(), UpdatedAt: storedUpdated.UTC(),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GroupFromRow() = %+v, want %+v", got, want)
	}
	checkUTC(t, "CreatedAt", got.CreatedAt)
	checkUTC(t, "UpdatedAt", got.UpdatedAt)
}

func TestGroupFromRowNamesTheEngineWhenTheLocationDoesNotRead(t *testing.T) {
	t.Parallel()

	unreadable := []byte("{not json")
	var rules content.Rules
	decodeErr := json.Unmarshal(unreadable, &rules)
	if decodeErr == nil {
		t.Fatal("json.Unmarshal() error = nil, want the location unreadable")
	}

	for name, test := range map[string]struct {
		engine dbkit.Engine
		want   string
	}{
		"under postgres": {dbkit.Postgres, "postgres: reading a group location: " + decodeErr.Error()},
		"under sqlite":   {dbkit.SQLite, "sqlite: reading a group location: " + decodeErr.Error()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := GroupFromRow(test.engine, GroupRow{ID: 7, Key: "book-details", Location: unreadable})

			if err == nil {
				t.Fatal("GroupFromRow() error = nil, want the unreadable location named")
			}
			if err.Error() != test.want {
				t.Errorf("GroupFromRow() error = %q, want %q", err, test.want)
			}
			if inner := errors.Unwrap(err); inner == nil || inner.Error() != decodeErr.Error() {
				t.Errorf("errors.Unwrap() = %v, want the decode error carried", inner)
			}
			if !reflect.DeepEqual(got, content.Group{}) {
				t.Errorf("GroupFromRow() = %+v, want no group", got)
			}
		})
	}
}

func TestToFieldReadsEveryColumnWithTimesInUTC(t *testing.T) {
	t.Parallel()

	row := FieldRow{
		ID: 11, GroupID: 7, ParentID: 4, Depth: 1,
		Key: "author", Label: "Author", Kind: "relation", RelatesTo: "person", Origin: "library",
		Many: true, Required: true, Settings: []byte(`{"max":3}`),
		CreatedAt: storedCreated, UpdatedAt: storedUpdated,
	}

	got := toField(row)

	want := content.Field{
		ID: 11, GroupID: 7, ParentID: 4, Key: "author", Label: "Author", Kind: content.FieldKindRelation,
		RelatesTo: "person", Many: true, Required: true, Origin: "library",
		Settings:  map[string]any{"max": float64(3)},
		CreatedAt: storedCreated.UTC(), UpdatedAt: storedUpdated.UTC(),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("toField() = %+v, want %+v", got, want)
	}
	checkUTC(t, "CreatedAt", got.CreatedAt)
	checkUTC(t, "UpdatedAt", got.UpdatedAt)
}

func TestToFieldReadsParentZeroAsAGroupTopAndKeepsAnyOther(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		parent int
		want   int
	}{
		"a field at a group's top":   {0, 0},
		"a field inside a container": {4, 4},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := toField(FieldRow{ID: 11, GroupID: 7, ParentID: test.parent, Key: "street", Settings: []byte("{}")})

			if got.ParentID != test.want {
				t.Errorf("toField().ParentID = %d, want %d", got.ParentID, test.want)
			}
		})
	}
}

func TestToFieldReadsAnEmptySettingsObjectAsNone(t *testing.T) {
	t.Parallel()

	got := toField(FieldRow{ID: 11, GroupID: 7, Key: "street", Settings: []byte("{}")})

	if got.Settings != nil {
		t.Errorf("toField().Settings = %v, want none", got.Settings)
	}
}

func TestSettingsJSONWritesAnEmptyObjectForNoSettings(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		settings map[string]any
		want     string
	}{
		"no settings":    {nil, "{}"},
		"empty settings": {map[string]any{}, "{}"},
		"some settings":  {map[string]any{"max": 3}, `{"max":3}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := string(settingsJSON(test.settings)); got != test.want {
				t.Errorf("settingsJSON() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestSettingsOfReadsNothingFromAnEmptyOrUnreadableObject(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string][]byte{
		"an empty object":   []byte("{}"),
		"a null":            []byte("null"),
		"nothing stored":    nil,
		"an unreadable one": []byte("{not json"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := settingsOf(raw); got != nil {
				t.Errorf("settingsOf(%s) = %v, want none", raw, got)
			}
		})
	}
}

func TestSettingsOfReadsWhatTheObjectHolds(t *testing.T) {
	t.Parallel()

	got := settingsOf([]byte(`{"max":3,"unit":"stars"}`))

	want := map[string]any{"max": float64(3), "unit": "stars"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("settingsOf() = %v, want %v", got, want)
	}
}

func TestSettingsWrittenEmptyReadBackAsNone(t *testing.T) {
	t.Parallel()

	if got := settingsOf(settingsJSON(nil)); got != nil {
		t.Errorf("settingsOf(settingsJSON(nil)) = %v, want none", got)
	}
}

func TestToTypeReadsEveryColumnWithTimesInUTC(t *testing.T) {
	t.Parallel()

	row := TypeRow{
		Key: "book", SingularLabel: "Book", PluralLabel: "Books", Description: "Printed works",
		RouteWord: "books", PageKind: "archive", Origin: "library",
		Hierarchical: true, Revisions: true, IsDefault: true, Active: true, RevisionCap: 25,
		CreatedAt: storedCreated, UpdatedAt: storedUpdated,
	}

	got := toType(row)

	want := content.Type{
		Key: "book", SingularLabel: "Book", PluralLabel: "Books", Description: "Printed works",
		RouteWord: "books", Hierarchical: true, Revisions: true, RevisionCap: 25,
		PageKind: content.PageKindArchive, Default: true, Active: true, Origin: "library",
		CreatedAt: storedCreated.UTC(), UpdatedAt: storedUpdated.UTC(),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("toType() = %+v, want %+v", got, want)
	}
	checkUTC(t, "CreatedAt", got.CreatedAt)
	checkUTC(t, "UpdatedAt", got.UpdatedAt)
}

func TestLocationJSONWritesTheNormalizedRules(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		location content.Rules
		want     string
	}{
		"no location": {nil, "[]"},
		"zero rules and empty groups": {
			content.Rules{
				{{}, {Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "book"}},
				{},
			},
			`[[{"source":"content_type","operator":"==","value":"book"}]]`,
		},
		"a value under an operator that takes none": {
			content.Rules{{{Source: "status", Operator: content.OperatorEmpty, Value: "draft"}}},
			`[[{"source":"status","operator":"empty","value":""}]]`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := string(locationJSON(test.location)); got != test.want {
				t.Errorf("locationJSON() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestFieldsByGroupBuildsEachGroupsTreeInStoredOrder(t *testing.T) {
	t.Parallel()

	declared := []FieldRow{
		{ID: 1, GroupID: 7, Key: "title", Kind: "text"},
		{ID: 2, GroupID: 7, Key: "address", Kind: "section"},
		{ID: 3, GroupID: 7, ParentID: 2, Key: "lines", Kind: "repeater"},
		{ID: 4, GroupID: 9, ParentID: 2, Key: "city", Kind: "text"},
		{ID: 5, GroupID: 7, ParentID: 3, Key: "line", Kind: "text"},
		{ID: 6, GroupID: 9, Key: "rating", Kind: "number"},
		{ID: 8, GroupID: 7, ParentID: 99, Key: "stray", Kind: "text"},
	}

	got := FieldsByGroup(declared)

	line := content.Field{ID: 5, GroupID: 7, ParentID: 3, Key: "line", Kind: content.FieldKindText}
	lines := content.Field{
		ID: 3, GroupID: 7, ParentID: 2, Key: "lines", Kind: content.FieldKindRepeater, Fields: []content.Field{line},
	}
	city := content.Field{ID: 4, GroupID: 7, ParentID: 2, Key: "city", Kind: content.FieldKindText}
	want := map[int][]content.Field{
		7: {
			{ID: 1, GroupID: 7, Key: "title", Kind: content.FieldKindText},
			{ID: 2, GroupID: 7, Key: "address", Kind: content.FieldKindSection, Fields: []content.Field{lines, city}},
		},
		9: {{ID: 6, GroupID: 9, Key: "rating", Kind: content.FieldKindNumber}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FieldsByGroup() = %+v, want %+v", got, want)
	}
}

func TestFieldsByGroupHoldsNoGroupForNoRows(t *testing.T) {
	t.Parallel()

	if got := FieldsByGroup(nil); len(got) != 0 {
		t.Errorf("FieldsByGroup(nil) = %v, want no group", got)
	}
}

func TestServesOnlyAnActiveGroupWhoseLocationMatchesTheType(t *testing.T) {
	t.Parallel()

	everyType := content.Rules{{{
		Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.AnyContentType,
	}}}
	for name, test := range map[string]struct {
		group content.Group
		want  bool
	}{
		"an active group matching the type":   {content.Group{Active: true, Location: bookOnly}, true},
		"an active group matching every type": {content.Group{Active: true, Location: everyType}, true},
		"an inactive group matching the type": {content.Group{Location: bookOnly}, false},
		"an active group matching another type": {
			content.Group{Active: true, Location: content.Rules{{{
				Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "film",
			}}}},
			false,
		},
		"an active group with no location": {content.Group{Active: true}, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := Serves(test.group, "book"); got != test.want {
				t.Errorf("Serves(%+v, book) = %v, want %v", test.group, got, test.want)
			}
		})
	}
}
