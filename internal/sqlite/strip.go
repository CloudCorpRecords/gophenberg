// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// keyPath is the object keys a strip function walks down, read from a JSON array of text.
type keyPath []string

// stripFieldPath answers strip_field_path(doc, path), doc without the value at path, the same path in every array.
func stripFieldPath(args []driver.Value) (driver.Value, error) {
	return strip("strip_field_path", args, dropKey)
}

// stripLayout answers strip_layout(doc, path), doc without the layout rows that hold the last key of path.
func stripLayout(args []driver.Value) (driver.Value, error) {
	return strip("strip_layout", args, dropHolders)
}

// strip answers fn(doc, path) with last applied where path has one key left, NULL in either giving NULL.
func strip(fn string, args []driver.Value, last func(doc any, key string) any) (driver.Value, error) {
	if args[0] == nil || args[1] == nil {
		return nil, nil
	}
	doc, path, err := decodeArgs(args)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	return encode(cutAt(doc, path, last))
}

// decodeArgs returns the document and the path a strip function was called with.
func decodeArgs(args []driver.Value) (any, keyPath, error) {
	doc, err := decodeDocument(args[0])
	if err != nil {
		return nil, nil, err
	}
	path, err := decodePath(args[1])
	if err != nil {
		return nil, nil, err
	}
	return doc, path, nil
}

// textOf returns a SQL text or blob value as text, and false for any other value.
func textOf(value driver.Value) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	}
	return "", false
}

// decodeDocument returns the one JSON value of value with every number kept as written.
func decodeDocument(value driver.Value) (any, error) {
	text, ok := textOf(value)
	if !ok {
		return nil, fmt.Errorf("the document must be JSON text, got %T", value)
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("the document is not JSON: %w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("the document holds more after its JSON value")
	}
	return doc, nil
}

// decodePath returns the keys of value, a JSON array of text.
func decodePath(value driver.Value) (keyPath, error) {
	text, ok := textOf(value)
	if !ok {
		return nil, fmt.Errorf("the path must be JSON text, got %T", value)
	}
	path, ok := keysOf(text)
	if !ok {
		return nil, fmt.Errorf("the path must be a JSON array of text, got %s", text)
	}
	return path, nil
}

// keysOf returns the keys of text, and false when text is not a JSON array of text with no null in it.
func keysOf(text string) (keyPath, bool) {
	var keys []*string
	if err := json.Unmarshal([]byte(text), &keys); err != nil || keys == nil {
		return nil, false
	}
	path := make(keyPath, len(keys))
	for i, key := range keys {
		if key == nil {
			return nil, false
		}
		path[i] = *key
	}
	return path, true
}

// encode returns doc as compact JSON text with its keys sorted and its markup left unescaped.
func encode(doc any) (driver.Value, error) {
	var text strings.Builder
	encoder := json.NewEncoder(&text)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(doc)
	return strings.TrimSuffix(text.String(), "\n"), err
}

// cutAt returns doc with last applied where path has one key left, walking every array with the same path.
func cutAt(doc any, path keyPath, last func(doc any, key string) any) any {
	if len(path) == 1 {
		return last(doc, path[0])
	}
	switch v := doc.(type) {
	case []any:
		return mapElements(v, func(element any) any { return cutAt(element, path, last) })
	case map[string]any:
		return descend(v, path, last)
	}
	return doc
}

// descend returns object with the value under the first key of path cut at the rest of path, when both exist.
func descend(object map[string]any, path keyPath, last func(doc any, key string) any) any {
	if len(path) == 0 {
		return object
	}
	if inner, ok := object[path[0]]; ok {
		object[path[0]] = cutAt(inner, path[1:], last)
	}
	return object
}

// mapElements returns the elements of array, each passed through cut.
func mapElements(array []any, cut func(element any) any) []any {
	cutArray := make([]any, len(array))
	for i, element := range array {
		cutArray[i] = cut(element)
	}
	return cutArray
}

// dropKey returns doc without key, from an object or from every object an array holds at any depth.
func dropKey(doc any, key string) any {
	switch v := doc.(type) {
	case []any:
		return mapElements(v, func(element any) any { return dropKey(element, key) })
	case map[string]any:
		delete(v, key)
	}
	return doc
}

// dropHolders returns an array without its rows that hold key, and any other value unchanged.
func dropHolders(doc any, key string) any {
	rows, ok := doc.([]any)
	if !ok {
		return doc
	}
	kept := make([]any, 0, len(rows))
	for _, row := range rows {
		if !holds(row, key) {
			kept = append(kept, row)
		}
	}
	return kept
}

// holds reports whether jsonb's ? operator finds key in value, as an object key, a text element or the text itself.
func holds(value any, key string) bool {
	switch v := value.(type) {
	case map[string]any:
		_, ok := v[key]
		return ok
	case []any:
		return slices.ContainsFunc(v, func(element any) bool { return holdsText(element, key) })
	}
	return holdsText(value, key)
}

// holdsText reports whether value is the text key.
func holdsText(value any, key string) bool {
	text, ok := value.(string)
	return ok && text == key
}
