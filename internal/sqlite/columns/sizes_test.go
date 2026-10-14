// SPDX-License-Identifier: Apache-2.0

package columns_test

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"

	"github.com/gopherium/gophenberg/internal/sqlite/columns"
)

// thumbJSON is the JSON text json.Marshal writes for thumb.
const thumbJSON = `{"thumb":{"file":"2026/08/harbor-150x150.jpg","width":150,"height":150,` +
	`"mime_type":"image/jpeg","filesize":4096}}`

// thumb is a set of sub-sizes holding one small copy of an image.
func thumb() columns.Sizes {
	return columns.Sizes{
		"thumb": {File: "2026/08/harbor-150x150.jpg", Width: 150, Height: 150, MimeType: "image/jpeg", Filesize: 4096},
	}
}

// openTable returns a handle on a fresh SQLite file holding a table whose one column keeps JSON text.
func openTable(t *testing.T) *sql.DB {
	t.Helper()
	db := sqlitetest.Open(t, dbsqlite.Options{
		BusyTimeout: 5 * time.Second, CacheSize: 2048, MaxConns: 4, Synchronous: dbsqlite.SynchronousFull,
	})
	statement := `CREATE TABLE images (sizes TEXT NOT NULL CHECK (json_valid(sizes)))`
	if _, err := db.ExecContext(t.Context(), statement); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
	return db
}

func TestSizesValueWritesJSONText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		sizes columns.Sizes
		want  string
	}{
		{name: "one sub-size", sizes: thumb(), want: thumbJSON},
		{name: "no sub-size", sizes: columns.Sizes{}, want: "{}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.sizes.Value()
			if err != nil {
				t.Fatalf("Value() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Value() = %#v, want the text %#v", got, tt.want)
			}
		})
	}
}

func TestSizesValueWritesNoValueForANilMap(t *testing.T) {
	t.Parallel()

	got, err := columns.Sizes(nil).Value()

	if err != nil || got != nil {
		t.Errorf("Value() of a nil map = %#v, error %v, want no value, the SQL NULL pgx binds for a nil map", got, err)
	}
}

func TestSizesScanReadsJSONText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want columns.Sizes
	}{
		{name: "one sub-size", text: thumbJSON, want: thumb()},
		{name: "no sub-size", text: "{}", want: columns.Sizes{}},
		{name: "null", text: "null", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got columns.Sizes
			if err := got.Scan(tt.text); err != nil {
				t.Fatalf("Scan(%q) error = %v", tt.text, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Scan(%q) = %#v, want %#v", tt.text, got, tt.want)
			}
		})
	}
}

func TestSizesScanReplacesWhatTheMapHeld(t *testing.T) {
	t.Parallel()

	got := columns.Sizes{"medium": {File: "2026/08/harbor-300x300.jpg", Width: 300, Height: 300}}
	if err := got.Scan(thumbJSON); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if !reflect.DeepEqual(got, thumb()) {
		t.Errorf("Scan() over a held map = %#v, want only the scanned %#v", got, thumb())
	}
}

func TestSizesScanFailsOnJSONOfTheWrongShape(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"[]", `{"thumb":1}`, `{"thumb":{"width":"wide"}}`} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			var got columns.Sizes
			err := got.Scan(text)
			if err == nil || !strings.HasPrefix(err.Error(), "columns: read the sizes: ") {
				t.Errorf("Scan(%q) error = %v, want a failure naming the sizes", text, err)
			}
		})
	}
}

func TestSizesScanFailsOnAValueThatIsNotText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  any
		want string
	}{
		{name: "an integer", src: int64(1), want: "int64"},
		{name: "a blob", src: []byte("{}"), want: "[]uint8"},
		{name: "a NULL", src: nil, want: "<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got columns.Sizes
			err := got.Scan(tt.src)
			if err == nil || !strings.HasSuffix(err.Error(), "got "+tt.want) {
				t.Errorf("Scan(%#v) error = %v, want a failure naming %s", tt.src, err, tt.want)
			}
		})
	}
}

func TestSizesKeepTheirValuesThroughSQLite(t *testing.T) {
	t.Parallel()

	db := openTable(t)
	if _, err := db.ExecContext(t.Context(), `INSERT INTO images (sizes) VALUES (?)`, thumb()); err != nil {
		t.Fatalf("storing the sizes: %v", err)
	}

	var kind, stored string
	var got columns.Sizes
	row := db.QueryRowContext(t.Context(), `SELECT typeof(sizes), sizes, sizes FROM images`)
	if err := row.Scan(&kind, &stored, &got); err != nil {
		t.Fatalf("reading the sizes back: %v", err)
	}
	if kind != "text" || stored != thumbJSON {
		t.Errorf("stored a %s holding %q, want text holding %q", kind, stored, thumbJSON)
	}
	if !reflect.DeepEqual(got, thumb()) {
		t.Errorf("read back %#v, want %#v", got, thumb())
	}
}

func TestSizesOfANilMapFailANotNullColumn(t *testing.T) {
	t.Parallel()

	db := openTable(t)

	_, err := db.ExecContext(t.Context(), `INSERT INTO images (sizes) VALUES (?)`, columns.Sizes(nil))

	if !errors.Is(dbsqlite.Classify(err), dbkit.ErrNotNull) {
		t.Errorf("storing a nil map error = %v, want dbkit.ErrNotNull, as PostgreSQL refuses it too", err)
	}
}
