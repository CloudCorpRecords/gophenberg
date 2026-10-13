// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"
)

// stripFunctions names both field strip functions SQL calls.
var stripFunctions = []string{"strip_field_path", "strip_layout"}

// stripCase is one document and path a strip function is called with and the JSON text it answers.
type stripCase struct {
	name string
	doc  string
	path string
	want string
}

// stripped returns the text the SQL function fn answers for doc and path on db.
func stripped(t *testing.T, db *sql.DB, fn string, doc, path any) string {
	t.Helper()
	var got string
	if err := db.QueryRowContext(t.Context(), "SELECT "+fn+"(?, ?)", doc, path).Scan(&got); err != nil {
		t.Fatalf("%s(%v, %v) error = %v, want nil", fn, doc, path, err)
	}
	return got
}

// checkStripped fails t for every case whose answer from the SQL function fn differs from the case's want.
func checkStripped(t *testing.T, fn string, cases []stripCase) {
	t.Helper()
	db := sqlitetest.Open(t, testSettings().Options())
	for _, c := range cases {
		if got := stripped(t, db, fn, c.doc, c.path); got != c.want {
			t.Errorf("%s on %s (%s, %s) = %s, want %s", fn, c.name, c.doc, c.path, got, c.want)
		}
	}
}

func TestStripFieldPathRemovesTheValueAtThePath(t *testing.T) {
	t.Parallel()

	checkStripped(t, "strip_field_path", []stripCase{
		{"a top level key", `{"a":1,"b":2}`, `["a"]`, `{"b":2}`},
		{"a nested key", `{"a":{"x":1,"y":2},"b":3}`, `["a","x"]`, `{"a":{"y":2},"b":3}`},
		{"every element of an array", `[{"a":1,"b":2},{"a":3},5]`, `["a"]`, `[{"b":2},{},5]`},
		{"every element of a nested array", `{"rows":[{"a":1,"b":2},{"a":3}]}`, `["rows","a"]`, `{"rows":[{"b":2},{}]}`},
		{"every element of an array inside an array", `[[{"a":1,"b":2}],[]]`, `["a"]`, `[[{"b":2}],[]]`},
		{"an empty key", `{"":1,"a":2}`, `[""]`, `{"a":2}`},
		{"the last of a repeated key", `{"a":1,"a":2,"b":3}`, `["b"]`, `{"a":2}`},
		{"a key beside an integer above 2^53", `{"n":9007199254740993,"a":1}`, `["a"]`, `{"n":9007199254740993}`},
		{"a key beside markup and accents", `{"t":"<b>é</b> & co","a":1}`, `["a"]`, `{"t":"<b>é</b> & co"}`},
	})
}

func TestStripFieldPathLeavesADocumentWithNothingAtThePathUnchanged(t *testing.T) {
	t.Parallel()

	checkStripped(t, "strip_field_path", []stripCase{
		{"an empty array", `[]`, `["a"]`, `[]`},
		{"text", `"a"`, `["a"]`, `"a"`},
		{"a number", `5`, `["a"]`, `5`},
		{"true", `true`, `["a"]`, `true`},
		{"JSON null", `null`, `["a"]`, `null`},
		{"an empty path", `{"a":1}`, `[]`, `{"a":1}`},
		{"a missing key", `{"a":1.50,"b":1e2}`, `["c"]`, `{"a":1.50,"b":1e2}`},
		{"a missing parent key", `{"a":1}`, `["b","a"]`, `{"a":1}`},
		{"a path longer than the document", `{"a":5}`, `["a","b"]`, `{"a":5}`},
	})
}

func TestStripLayoutDropsTheRowsThatHoldTheKey(t *testing.T) {
	t.Parallel()

	checkStripped(t, "strip_layout", []stripCase{
		{"an object row with the key", `{"blocks":[{"hero":{}},{"text":1}]}`, `["blocks","hero"]`,
			`{"blocks":[{"text":1}]}`},
		{"a bare text row equal to the key", `{"blocks":["hero","other"]}`, `["blocks","hero"]`,
			`{"blocks":["other"]}`},
		{"an array row holding the key as text", `{"blocks":[["x","hero"],["x",["hero"]]]}`, `["blocks","hero"]`,
			`{"blocks":[["x",["hero"]]]}`},
		{"no number row that reads as the key", `[7,"7"]`, `["7"]`, `[7]`},
		{"rows of a top level array", `[{"a":1},{"b":2},null,true]`, `["a"]`, `[{"b":2},null,true]`},
		{"every row", `{"rows":[{"a":1}]}`, `["rows","a"]`, `{"rows":[]}`},
		{"rows under every element", `[{"rows":[{"a":1},{"b":1}]},3]`, `["rows","a"]`, `[{"rows":[{"b":1}]},3]`},
		{"rows three levels down", `{"l":{"rows":[{"a":1},{"b":2}]}}`, `["l","rows","a"]`, `{"l":{"rows":[{"b":2}]}}`},
		{"rows beside an integer above 2^53", `{"n":9007199254740993,"rows":[{"a":1}]}`, `["rows","a"]`,
			`{"n":9007199254740993,"rows":[]}`},
	})
}

func TestStripLayoutLeavesADocumentWithNoRowsAtThePathUnchanged(t *testing.T) {
	t.Parallel()

	checkStripped(t, "strip_layout", []stripCase{
		{"an object under a path of one", `{"a":1}`, `["a"]`, `{"a":1}`},
		{"an object under an empty path", `{"a":1}`, `[]`, `{"a":1}`},
		{"an array under an empty path", `[{"a":1}]`, `[]`, `[{"a":1}]`},
		{"an object where the rows belong", `{"rows":{"a":1}}`, `["rows","a"]`, `{"rows":{"a":1}}`},
		{"a missing key", `{"x":[],"y":1.50}`, `["rows","a"]`, `{"x":[],"y":1.50}`},
		{"text", `"rows"`, `["rows","a"]`, `"rows"`},
		{"an empty array", `[]`, `["a"]`, `[]`},
		{"rows that are not text under the empty key", `[1,null,true,[2],{"b":1}]`, `[""]`, `[1,null,true,[2],{"b":1}]`},
	})
}

func TestTheStripFunctionsAnswerNullForANullDocumentOrPath(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	for _, fn := range stripFunctions {
		var docNull, pathNull bool
		query := "SELECT " + fn + `(NULL, '["a"]') IS NULL, ` + fn + "('{}', NULL) IS NULL"
		if err := db.QueryRowContext(t.Context(), query).Scan(&docNull, &pathNull); err != nil {
			t.Fatalf("%s with NULL error = %v, want nil", fn, err)
		}
		if !docNull || !pathNull {
			t.Errorf("%s answers NULL for a NULL document %t and for a NULL path %t, want both", fn, docNull, pathNull)
		}
	}
}

func TestTheStripFunctionsReadJSONTextHeldInABlob(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	if got := stripped(t, db, "strip_field_path", []byte(`{"a":1,"b":2}`), []byte(`["a"]`)); got != `{"b":2}` {
		t.Errorf("strip_field_path on blobs = %s, want {\"b\":2}", got)
	}
	if got := stripped(t, db, "strip_layout", []byte(`{"r":["a","b"]}`), []byte(`["r","a"]`)); got != `{"r":["b"]}` {
		t.Errorf("strip_layout on blobs = %s, want {\"r\":[\"b\"]}", got)
	}
}

func TestTheStripFunctionsRefuseADocumentOrAPathThatIsNotJSONText(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	for _, fn := range stripFunctions {
		for _, bad := range []struct {
			name      string
			doc, path any
		}{
			{"a document that is not JSON", `{`, `["a"]`},
			{"a document with text after its value", `{} {}`, `["a"]`},
			{"an empty document", ``, `["a"]`},
			{"a document that is a number", int64(5), `["a"]`},
			{"a path that is not JSON", `{}`, `[`},
			{"a path that holds a number", `{}`, `["a",1]`},
			{"a path that holds JSON null", `{}`, `["a",null]`},
			{"a path of JSON null", `{}`, `null`},
			{"a path that is an object", `{}`, `{"a":1}`},
			{"a path that is a number", `{}`, int64(1)},
		} {
			var got sql.NullString
			err := db.QueryRowContext(t.Context(), "SELECT "+fn+"(?, ?)", bad.doc, bad.path).Scan(&got)
			if err == nil || !strings.Contains(err.Error(), fn+": the ") {
				t.Errorf("%s on %s error = %v, want the function naming what it was handed", fn, bad.name, err)
			}
		}
	}
}

func TestTheStripFunctionsTakeTwoArguments(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	for _, fn := range stripFunctions {
		_, err := db.ExecContext(t.Context(), "SELECT "+fn+`('{}', '[]', 'x')`)
		if err == nil || !strings.Contains(err.Error(), "wrong number of arguments") {
			t.Errorf("%s with three arguments error = %v, want the argument count refused", fn, err)
		}
	}
}
