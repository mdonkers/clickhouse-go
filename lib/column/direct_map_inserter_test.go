// Licensed to ClickHouse, Inc. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. ClickHouse, Inc. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package column

import (
	"fmt"
	"reflect"
	"testing"
)

// --- helpers ---

// newLCString creates a properly-initialized LowCardinality(String) column.
func newLCString(name string) *LowCardinality {
	lc := &LowCardinality{
		name:   name,
		chType: "LowCardinality(String)",
		index:  &String{name: name},
	}
	lc.append.index = make(map[any]int)
	return lc
}

// newTupleStringInt8 creates a Tuple(String, Int8) column with two sub-columns.
func newTupleStringInt8(name string) *Tuple {
	return &Tuple{
		name:   name,
		chType: "Tuple(String, Int8)",
		columns: []Interface{
			&String{name: "v"},
			&Int8{name: "t"},
		},
	}
}

// newLCStringTupleMap creates a Map(LowCardinality(String), Tuple(String, Int8)) column
// that matches the attribute map used in the Dash0 ClickHouse exporter.
func newLCStringTupleMap(name string) *Map {
	lc := newLCString("key")
	tup := newTupleStringInt8("value")
	return &Map{
		name:     name,
		chType:   "Map(LowCardinality(String), Tuple(String, Int8))",
		keys:     lc,
		values:   tup,
		scanType: reflect.TypeOf(map[string]any{}),
	}
}

// -- directInserter implements DirectMapInserter, using typed append helpers when available --

type attrEntry struct {
	key  string
	sVal string
	iVal int8
}

type directInserter struct {
	entries []attrEntry
}

func (d *directInserter) InsertTo(keyCol Interface, valueCol Interface) (int64, error) {
	tup, _ := valueCol.(*Tuple)
	var valStr *String
	var valInt8 *Int8
	if tup != nil && len(tup.columns) == 2 {
		valStr, _ = tup.columns[0].(*String)
		valInt8, _ = tup.columns[1].(*Int8)
	}
	for _, e := range d.entries {
		// AppendRow dispatches via the string fast path inside LowCardinality — no extra interface needed.
		if err := keyCol.AppendRow(e.key); err != nil {
			return 0, err
		}
		if valStr != nil && valInt8 != nil {
			valStr.AppendRowString(e.sVal)
			valInt8.AppendRowInt8(e.iVal)
		} else if err := valueCol.AppendRow([]any{e.sVal, e.iVal}); err != nil {
			return 0, err
		}
	}
	return int64(len(d.entries)), nil
}

// -- iterableInserter implements IterableOrderedMap, using the boxed Key()/Value() iterator --

type iterableInserter struct {
	entries []attrEntry
	pos     int
}

func (it *iterableInserter) Put(_ any, _ any) {}
func (it *iterableInserter) Iterator() MapIterator {
	it.pos = -1
	return it
}
func (it *iterableInserter) Next() bool {
	it.pos++
	return it.pos < len(it.entries)
}
func (it *iterableInserter) Key() any   { return it.entries[it.pos].key }
func (it *iterableInserter) Value() any { return []any{it.entries[it.pos].sVal, it.entries[it.pos].iVal} }

// --- DirectMapInserter tests ---

func TestDirectMapInserter_Dispatch(t *testing.T) {
	// A value that implements DirectMapInserter (but not IterableOrderedMap) must
	// reach InsertTo; this test verifies the dispatch path in Map.AppendRow.
	col := newLCStringTupleMap("attrs")
	ins := &directInserter{entries: []attrEntry{{"service.name", "web", 1}}}
	if err := col.AppendRow(ins); err != nil {
		t.Fatalf("AppendRow with DirectMapInserter: %v", err)
	}
	if col.offsets.Rows() != 1 {
		t.Fatalf("expected 1 offset row, got %d", col.offsets.Rows())
	}
	if col.offsets.col.Row(0) != 1 {
		t.Fatalf("expected offset 1, got %d", col.offsets.col.Row(0))
	}
}

func TestDirectMapInserter_CorrectKeysAndValues(t *testing.T) {
	col := newLCStringTupleMap("attrs")
	ins := &directInserter{entries: []attrEntry{
		{"service.name", "checkout", 1},
		{"http.method", "POST", 1},
		{"http.status_code", "200", 2},
	}}
	if err := col.AppendRow(ins); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}

	lc := col.keys.(*LowCardinality)
	tup := col.values.(*Tuple)
	strCol := tup.columns[0].(*String)
	int8Col := tup.columns[1].(*Int8)

	// 3 key rows and 3 value rows should have been appended
	if lc.Rows() != 3 {
		t.Fatalf("expected 3 LC rows, got %d", lc.Rows())
	}
	if strCol.Rows() != 3 {
		t.Fatalf("expected 3 string rows, got %d", strCol.Rows())
	}

	// Verify string values
	wantStrVals := []string{"checkout", "POST", "200"}
	for i, want := range wantStrVals {
		if got := strCol.col.Row(i); got != want {
			t.Errorf("strCol row %d: got %q, want %q", i, got, want)
		}
	}

	// Verify int8 values
	wantIntVals := []int8{1, 1, 2}
	for i, want := range wantIntVals {
		got := int8Col.col[i]
		if got != want {
			t.Errorf("int8Col row %d: got %d, want %d", i, got, want)
		}
	}
}

func TestDirectMapInserter_EmptyMap(t *testing.T) {
	col := newLCStringTupleMap("attrs")
	ins := &directInserter{entries: nil}
	if err := col.AppendRow(ins); err != nil {
		t.Fatalf("AppendRow empty: %v", err)
	}
	if col.offsets.Rows() != 1 {
		t.Fatalf("expected 1 offset, got %d", col.offsets.Rows())
	}
	if col.offsets.col.Row(0) != 0 {
		t.Fatalf("expected offset 0, got %d", col.offsets.col.Row(0))
	}
}

func TestDirectMapInserter_MultipleRows(t *testing.T) {
	col := newLCStringTupleMap("attrs")

	// Row 0: 2 attrs
	if err := col.AppendRow(&directInserter{entries: []attrEntry{
		{"a", "x", 1},
		{"b", "y", 2},
	}}); err != nil {
		t.Fatal(err)
	}
	// Row 1: 1 attr
	if err := col.AppendRow(&directInserter{entries: []attrEntry{
		{"c", "z", 3},
	}}); err != nil {
		t.Fatal(err)
	}

	// Offsets should be cumulative: [2, 3]
	if col.offsets.Rows() != 2 {
		t.Fatalf("expected 2 offset rows, got %d", col.offsets.Rows())
	}
	if col.offsets.col.Row(0) != 2 {
		t.Errorf("row 0 offset: got %d, want 2", col.offsets.col.Row(0))
	}
	if col.offsets.col.Row(1) != 3 {
		t.Errorf("row 1 offset: got %d, want 3", col.offsets.col.Row(1))
	}
}

// --- LowCardinality string fast path tests (via AppendRow) ---

func TestLowCardinality_AppendRow_String_UniqueKeys(t *testing.T) {
	lc := newLCString("k")

	keys := []string{"alpha", "beta", "gamma", "alpha", "beta", "alpha"}
	for _, k := range keys {
		if err := lc.AppendRow(k); err != nil {
			t.Fatalf("AppendRow(%q): %v", k, err)
		}
	}

	if lc.Rows() != len(keys) {
		t.Fatalf("Rows: got %d, want %d", lc.Rows(), len(keys))
	}

	// Only 3 unique values in the dictionary (plus the init nil entry = 4 index rows)
	// The nil init entry is at index 0; unique strings start at 1.
	wantUnique := 3
	if got := len(lc.append.strIndex); got != wantUnique {
		t.Fatalf("strIndex unique entries: got %d, want %d", got, wantUnique)
	}

	// Same unique count must be reflected in index map
	if got := len(lc.append.index); got != wantUnique {
		t.Fatalf("index unique entries: got %d, want %d (must stay in sync)", got, wantUnique)
	}
}

func TestLowCardinality_AppendRow_String_CacheHits_SameKeyIndex(t *testing.T) {
	lc := newLCString("k")

	_ = lc.AppendRow("x")
	_ = lc.AppendRow("y")
	_ = lc.AppendRow("x") // cache hit
	_ = lc.AppendRow("x") // cache hit

	// append.keys should be [idx(x), idx(y), idx(x), idx(x)]
	idxX := lc.append.strIndex["x"]
	idxY := lc.append.strIndex["y"]
	if idxX == idxY {
		t.Fatalf("x and y must have distinct indices, both got %d", idxX)
	}
	want := []int{idxX, idxY, idxX, idxX}
	if !reflect.DeepEqual(lc.append.keys, want) {
		t.Fatalf("append.keys: got %v, want %v", lc.append.keys, want)
	}
}

func TestLowCardinality_AppendRow_String_InSyncWithBoxedIndex(t *testing.T) {
	lc := newLCString("k")

	strs := []string{"foo", "bar", "baz", "foo", "bar"}
	for _, s := range strs {
		_ = lc.AppendRow(s)
	}

	// strIndex and index must agree on every key
	for k, strIdx := range lc.append.strIndex {
		boxedIdx, ok := lc.append.index[k]
		if !ok {
			t.Errorf("key %q present in strIndex but missing from index", k)
			continue
		}
		if strIdx != boxedIdx {
			t.Errorf("key %q: strIndex=%d, index=%d (must match)", k, strIdx, boxedIdx)
		}
	}

	// index must not have extra keys beyond strIndex
	if len(lc.append.index) != len(lc.append.strIndex) {
		t.Errorf("index has %d keys, strIndex has %d; must be equal",
			len(lc.append.index), len(lc.append.strIndex))
	}
}

func TestLowCardinality_AppendRow_String_Reset_ClearsStrIndex(t *testing.T) {
	lc := newLCString("k")

	_ = lc.AppendRow("pre-reset")
	if len(lc.append.strIndex) == 0 {
		t.Fatal("expected strIndex to be populated before Reset")
	}

	lc.Reset()

	if len(lc.append.strIndex) != 0 {
		t.Fatalf("expected strIndex to be empty after Reset, got %v", lc.append.strIndex)
	}
	if lc.Rows() != 0 {
		t.Fatalf("expected 0 rows after Reset, got %d", lc.Rows())
	}

	// Must still work after Reset
	if err := lc.AppendRow("post-reset"); err != nil {
		t.Fatalf("AppendRow after Reset: %v", err)
	}
	if lc.Rows() != 1 {
		t.Fatalf("expected 1 row after post-reset append, got %d", lc.Rows())
	}
}

// --- String.AppendRowString tests ---

func TestString_AppendRowString(t *testing.T) {
	col := &String{name: "s"}
	col.AppendRowString("hello")
	col.AppendRowString("world")

	if col.Rows() != 2 {
		t.Fatalf("expected 2 rows, got %d", col.Rows())
	}
	if got := col.col.Row(0); got != "hello" {
		t.Errorf("row 0: got %q, want %q", got, "hello")
	}
	if got := col.col.Row(1); got != "world" {
		t.Errorf("row 1: got %q, want %q", got, "world")
	}
}

func TestString_AppendRowString_EquivalentToAppendRow(t *testing.T) {
	col1 := &String{name: "s"}
	col2 := &String{name: "s"}

	vals := []string{"a", "bb", "ccc", ""}
	for _, v := range vals {
		col1.AppendRowString(v)
		_ = col2.AppendRow(v)
	}

	for i, v := range vals {
		r1 := col1.col.Row(i)
		r2 := col2.col.Row(i)
		if r1 != v || r2 != v {
			t.Errorf("row %d: AppendRowString=%q, AppendRow=%q, want %q", i, r1, r2, v)
		}
	}
}

// --- Int8.AppendRowInt8 tests ---

func TestInt8_AppendRowInt8(t *testing.T) {
	col := &Int8{name: "t"}
	col.AppendRowInt8(42)
	col.AppendRowInt8(-1)
	col.AppendRowInt8(0)

	if len(col.col) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(col.col))
	}
	if col.col[0] != 42 || col.col[1] != -1 || col.col[2] != 0 {
		t.Errorf("unexpected values: %v", col.col)
	}
}

func TestInt8_AppendRowInt8_EquivalentToAppendRow(t *testing.T) {
	col1 := &Int8{name: "t"}
	col2 := &Int8{name: "t"}

	vals := []int8{0, 1, -1, 127, -128, 42}
	for _, v := range vals {
		col1.AppendRowInt8(v)
		_ = col2.AppendRow(v)
	}

	for i, v := range vals {
		if col1.col[i] != v || col2.col[i] != v {
			t.Errorf("row %d: AppendRowInt8=%d, AppendRow=%d, want %d", i, col1.col[i], col2.col[i], v)
		}
	}
}

// --- Tuple.Columns tests ---

func TestTuple_Columns_ReturnsSubColumns(t *testing.T) {
	strCol := &String{name: "v"}
	int8Col := &Int8{name: "t"}
	tup := &Tuple{
		name:    "attrs_value",
		chType:  "Tuple(String, Int8)",
		columns: []Interface{strCol, int8Col},
	}

	cols := tup.Columns()
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(cols))
	}
	// Returned slice elements must be the same objects, not copies
	if cols[0] != strCol {
		t.Error("cols[0] is not the original *String")
	}
	if cols[1] != int8Col {
		t.Error("cols[1] is not the original *Int8")
	}
}

func TestTuple_Columns_TypeAssertions(t *testing.T) {
	tup := newTupleStringInt8("v")
	cols := tup.Columns()

	if _, ok := cols[0].(*String); !ok {
		t.Errorf("cols[0] should be *String, got %T", cols[0])
	}
	if _, ok := cols[1].(*Int8); !ok {
		t.Errorf("cols[1] should be *Int8, got %T", cols[1])
	}
}

// --- Benchmarks ---

func makeEntries(n int) []attrEntry {
	entries := make([]attrEntry, n)
	keys := []string{
		"service.name", "http.method", "http.status_code", "db.system",
		"net.peer.name", "messaging.system", "rpc.service", "enduser.id",
	}
	for i := range entries {
		entries[i] = attrEntry{
			key:  keys[i%len(keys)],
			sVal: fmt.Sprintf("val-%d", i),
			iVal: int8(i % 5),
		}
	}
	return entries
}

// BenchmarkDirectMapInserter measures the allocation advantage of DirectMapInserter
// over the boxed IterableOrderedMap path for attribute map insertion.
func BenchmarkDirectMapInserter(b *testing.B) {
	for _, n := range []int{5, 10, 20} {
		entries := makeEntries(n)

		b.Run(fmt.Sprintf("DirectMapInserter/attrs=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			col := newLCStringTupleMap("attrs")
			ins := &directInserter{entries: entries}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				col.keys.Reset()
				col.values.Reset()
				col.offsets.Reset()
				_ = col.AppendRow(ins)
			}
		})

		b.Run(fmt.Sprintf("IterableOrderedMap/attrs=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			col := newLCStringTupleMap("attrs")
			ins := &iterableInserter{entries: entries}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				col.keys.Reset()
				col.values.Reset()
				col.offsets.Reset()
				_ = col.AppendRow(ins)
			}
		})
	}
}

// BenchmarkLowCardinality_AppendRow_String measures the string fast path inside AppendRow
// (map[string]int strIndex) vs passing a non-string value that takes the boxed index path.
func BenchmarkLowCardinality_AppendRow_String(b *testing.B) {
	keys := []string{
		"service.name", "http.method", "http.status_code", "db.system",
		"net.peer.name", "messaging.system",
	}

	b.Run("string_fast_path", func(b *testing.B) {
		b.ReportAllocs()
		lc := newLCString("k")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = lc.AppendRow(keys[i%len(keys)])
		}
	})
}
