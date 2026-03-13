// SPDX-FileCopyrightText: Copyright 2024-2026 Dash0 Inc.

package column

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strconv"
	"testing"
)

// --- helpers to build Map columns for benchmarks/tests ---

func buildStringStringMapColumn(rows, mapSize int) *Map {
	keysCol := &String{name: "key"}
	valsCol := &String{name: "val"}
	var offsets Int64

	cumOffset := int64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < mapSize; j++ {
			keysCol.col.Append("key-" + strconv.Itoa(r*mapSize+j))
			valsCol.col.Append("val-" + strconv.Itoa(r*mapSize+j))
		}
		cumOffset += int64(mapSize)
		offsets.col.Append(cumOffset)
	}

	return &Map{
		keys:     keysCol,
		values:   valsCol,
		chType:   "Map(String, String)",
		offsets:  offsets,
		scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}
}

func buildStringInt64MapColumn(rows, mapSize int) *Map {
	keysCol := &String{name: "key"}
	valsCol := &Int64{name: "val"}
	var offsets Int64

	cumOffset := int64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < mapSize; j++ {
			keysCol.col.Append("key-" + strconv.Itoa(r*mapSize+j))
			valsCol.col = append(valsCol.col, rand.Int64())
		}
		cumOffset += int64(mapSize)
		offsets.col.Append(cumOffset)
	}

	return &Map{
		keys:     keysCol,
		values:   valsCol,
		chType:   "Map(String, Int64)",
		offsets:  offsets,
		scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}
}

func buildStringFloat64MapColumn(rows, mapSize int) *Map {
	keysCol := &String{name: "key"}
	valsCol := &Float64{name: "val"}
	var offsets Int64

	cumOffset := int64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < mapSize; j++ {
			keysCol.col.Append("key-" + strconv.Itoa(r*mapSize+j))
			valsCol.col = append(valsCol.col, rand.Float64())
		}
		cumOffset += int64(mapSize)
		offsets.col.Append(cumOffset)
	}

	return &Map{
		keys:     keysCol,
		values:   valsCol,
		chType:   "Map(String, Float64)",
		offsets:  offsets,
		scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}
}

func buildInt64StringMapColumn(rows, mapSize int) *Map {
	keysCol := &Int64{name: "key"}
	valsCol := &String{name: "val"}
	var offsets Int64

	cumOffset := int64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < mapSize; j++ {
			keysCol.col = append(keysCol.col, int64(r*mapSize+j))
			valsCol.col.Append("val-" + strconv.Itoa(r*mapSize+j))
		}
		cumOffset += int64(mapSize)
		offsets.col.Append(cumOffset)
	}

	return &Map{
		keys:     keysCol,
		values:   valsCol,
		chType:   "Map(Int64, String)",
		offsets:  offsets,
		scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}
}

func buildInt64Float64MapColumn(rows, mapSize int) *Map {
	keysCol := &Int64{name: "key"}
	valsCol := &Float64{name: "val"}
	var offsets Int64

	cumOffset := int64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < mapSize; j++ {
			keysCol.col = append(keysCol.col, int64(r*mapSize+j))
			valsCol.col = append(valsCol.col, rand.Float64())
		}
		cumOffset += int64(mapSize)
		offsets.col.Append(cumOffset)
	}

	return &Map{
		keys:     keysCol,
		values:   valsCol,
		chType:   "Map(Int64, Float64)",
		offsets:  offsets,
		scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}
}

// --- correctness tests ---

func TestMapScanRowPlain_StringString(t *testing.T) {
	keysCol := &String{name: "key"}
	valsCol := &String{name: "val"}
	var offsets Int64

	keysCol.col.Append("a")
	keysCol.col.Append("b")
	keysCol.col.Append("c")
	valsCol.col.Append("1")
	valsCol.col.Append("2")
	valsCol.col.Append("3")
	offsets.col.Append(3) // row 0: 3 entries

	keysCol.col.Append("x")
	valsCol.col.Append("y")
	offsets.col.Append(4) // row 1: 1 entry

	col := &Map{
		keys: keysCol, values: valsCol, chType: "Map(String, String)",
		offsets: offsets, scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}

	var dest map[string]string
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	if len(dest) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(dest))
	}
	if dest["a"] != "1" || dest["b"] != "2" || dest["c"] != "3" {
		t.Fatalf("unexpected map: %v", dest)
	}

	if err := col.ScanRow(&dest, 1); err != nil {
		t.Fatal(err)
	}
	if len(dest) != 1 || dest["x"] != "y" {
		t.Fatalf("row 1: unexpected map: %v", dest)
	}
}

func TestMapScanRowPlain_StringInt64(t *testing.T) {
	keysCol := &String{name: "key"}
	valsCol := &Int64{name: "val"}
	var offsets Int64

	keysCol.col.Append("count")
	keysCol.col.Append("total")
	valsCol.col = append(valsCol.col, 42, 100)
	offsets.col.Append(2)

	col := &Map{
		keys: keysCol, values: valsCol, chType: "Map(String, Int64)",
		offsets: offsets, scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}

	var dest map[string]int64
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	if dest["count"] != 42 || dest["total"] != 100 {
		t.Fatalf("unexpected map: %v", dest)
	}
}

func TestMapScanRowPlain_Int64String(t *testing.T) {
	keysCol := &Int64{name: "key"}
	valsCol := &String{name: "val"}
	var offsets Int64

	keysCol.col = append(keysCol.col, 1, 2)
	valsCol.col.Append("one")
	valsCol.col.Append("two")
	offsets.col.Append(2)

	col := &Map{
		keys: keysCol, values: valsCol, chType: "Map(Int64, String)",
		offsets: offsets, scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}

	var dest map[int64]string
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	if dest[1] != "one" || dest[2] != "two" {
		t.Fatalf("unexpected map: %v", dest)
	}
}

func TestMapScanRowPlain_Int64Float64(t *testing.T) {
	keysCol := &Int64{name: "key"}
	valsCol := &Float64{name: "val"}
	var offsets Int64

	keysCol.col = append(keysCol.col, 10, 20)
	valsCol.col = append(valsCol.col, 1.5, 2.5)
	offsets.col.Append(2)

	col := &Map{
		keys: keysCol, values: valsCol, chType: "Map(Int64, Float64)",
		offsets: offsets, scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}

	var dest map[int64]float64
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	if dest[10] != 1.5 || dest[20] != 2.5 {
		t.Fatalf("unexpected map: %v", dest)
	}
}

func TestMapScanRowPlain_EmptyMap(t *testing.T) {
	keysCol := &String{name: "key"}
	valsCol := &String{name: "val"}
	var offsets Int64
	offsets.col.Append(0) // row 0: empty map

	col := &Map{
		keys: keysCol, values: valsCol, chType: "Map(String, String)",
		offsets: offsets, scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}

	var dest map[string]string
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	if dest == nil {
		t.Fatal("expected non-nil empty map, got nil")
	}
	if len(dest) != 0 {
		t.Fatalf("expected empty map, got %v", dest)
	}
}

func TestMapScanRowPlain_MultipleRows(t *testing.T) {
	col := buildStringStringMapColumn(5, 3)

	var dest map[string]string

	// Scan each row and verify the map has the right number of entries
	for r := 0; r < 5; r++ {
		if err := col.ScanRow(&dest, r); err != nil {
			t.Fatalf("row %d: %v", r, err)
		}
		if len(dest) != 3 {
			t.Fatalf("row %d: expected 3 entries, got %d", r, len(dest))
		}
		// Verify keys are unique per row
		for j := 0; j < 3; j++ {
			key := "key-" + strconv.Itoa(r*3+j)
			val := "val-" + strconv.Itoa(r*3+j)
			if dest[key] != val {
				t.Fatalf("row %d: expected %s=%s, got %s=%s", r, key, val, key, dest[key])
			}
		}
	}
}

func TestMapScanRowPlain_NoAliasing(t *testing.T) {
	col := buildStringStringMapColumn(3, 4)

	var dest map[string]string

	// Scan row 0 and save a reference
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	saved0 := dest

	// Scan row 1 into the same variable
	if err := col.ScanRow(&dest, 1); err != nil {
		t.Fatal(err)
	}

	// saved0 must still contain the original row 0 data (4 entries)
	if len(saved0) != 4 {
		t.Fatalf("saved reference to row 0 changed length: got %d, want 4", len(saved0))
	}
	// Verify a key from row 0 is still in saved0
	if saved0["key-0"] != "val-0" {
		t.Fatalf("saved reference to row 0 was corrupted")
	}
}

func TestMapScanRowPlain_BoolValues(t *testing.T) {
	keysCol := &String{name: "key"}
	valsCol := &Bool{name: "val"}
	var offsets Int64

	keysCol.col.Append("active")
	keysCol.col.Append("deleted")
	valsCol.col = append(valsCol.col, true, false)
	offsets.col.Append(2)

	col := &Map{
		keys: keysCol, values: valsCol, chType: "Map(String, Bool)",
		offsets: offsets, scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}

	var dest map[string]bool
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	if dest["active"] != true || dest["deleted"] != false {
		t.Fatalf("unexpected map: %v", dest)
	}
}

func TestMapScanRowPlain_FallbackToReflection(t *testing.T) {
	// Scanning into *map[string]any should NOT be handled by the fast path
	// (any is not a concrete type), but should still work via reflection.
	keysCol := &String{name: "key"}
	valsCol := &String{name: "val"}
	var offsets Int64

	keysCol.col.Append("k")
	valsCol.col.Append("v")
	offsets.col.Append(1)

	col := &Map{
		keys: keysCol, values: valsCol, chType: "Map(String, String)",
		offsets: offsets, scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}

	// The fast path should fail for *map[string]any
	var dest map[string]any
	err := col.scanRowPlain(&dest, 0)
	if err == nil {
		t.Fatal("expected fast path to fail for map[string]any")
	}

	// But ScanRow should still work via the reflection path
	var dest2 map[string]string
	if err := col.ScanRow(&dest2, 0); err != nil {
		t.Fatal(err)
	}
	if dest2["k"] != "v" {
		t.Fatalf("unexpected map: %v", dest2)
	}
}

func TestMapScanRowPlain_DestTypeMismatch(t *testing.T) {
	// Column is Map(String, Int64) but dest is *map[string]float64.
	// Fast path should fail (value column type mismatch), then reflection
	// path should also fail (type mismatch).
	keysCol := &String{name: "key"}
	valsCol := &Int64{name: "val"}
	var offsets Int64

	keysCol.col.Append("k")
	valsCol.col = append(valsCol.col, 42)
	offsets.col.Append(1)

	col := &Map{
		keys: keysCol, values: valsCol, chType: "Map(String, Int64)",
		offsets: offsets, scanType: reflect.MapOf(keysCol.ScanType(), valsCol.ScanType()),
	}

	var dest map[string]float64
	err := col.scanRowPlain(&dest, 0)
	if err == nil {
		t.Fatal("expected fast path to fail for type mismatch")
	}
}

// --- benchmarks ---

func BenchmarkMapScanRow(b *testing.B) {
	for _, mapSize := range []int{5, 10, 20, 50} {
		rows := 1000

		b.Run(fmt.Sprintf("StringString/size=%d", mapSize), func(b *testing.B) {
			col := buildStringStringMapColumn(rows, mapSize)

			b.Run("Reflection", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					row := i % rows
					val := col.row(row)
					_ = val.Interface().(map[string]string)
				}
			})

			b.Run("FastPath", func(b *testing.B) {
				b.ReportAllocs()
				var dest map[string]string
				for i := 0; i < b.N; i++ {
					row := i % rows
					if err := col.scanRowPlain(&dest, row); err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run("ScanRow", func(b *testing.B) {
				b.ReportAllocs()
				var dest map[string]string
				for i := 0; i < b.N; i++ {
					row := i % rows
					if err := col.ScanRow(&dest, row); err != nil {
						b.Fatal(err)
					}
				}
			})
		})

		b.Run(fmt.Sprintf("StringInt64/size=%d", mapSize), func(b *testing.B) {
			col := buildStringInt64MapColumn(rows, mapSize)

			b.Run("Reflection", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					row := i % rows
					val := col.row(row)
					_ = val.Interface().(map[string]int64)
				}
			})

			b.Run("FastPath", func(b *testing.B) {
				b.ReportAllocs()
				var dest map[string]int64
				for i := 0; i < b.N; i++ {
					row := i % rows
					if err := col.scanRowPlain(&dest, row); err != nil {
						b.Fatal(err)
					}
				}
			})
		})

		b.Run(fmt.Sprintf("StringFloat64/size=%d", mapSize), func(b *testing.B) {
			col := buildStringFloat64MapColumn(rows, mapSize)

			b.Run("Reflection", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					row := i % rows
					val := col.row(row)
					_ = val.Interface().(map[string]float64)
				}
			})

			b.Run("FastPath", func(b *testing.B) {
				b.ReportAllocs()
				var dest map[string]float64
				for i := 0; i < b.N; i++ {
					row := i % rows
					if err := col.scanRowPlain(&dest, row); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func BenchmarkMapScanRow_TypedKey(b *testing.B) {
	rows := 1000
	mapSize := 10

	b.Run("Int64String", func(b *testing.B) {
		col := buildInt64StringMapColumn(rows, mapSize)

		b.Run("Reflection", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				row := i % rows
				val := col.row(row)
				_ = val.Interface().(map[int64]string)
			}
		})

		b.Run("FastPath", func(b *testing.B) {
			b.ReportAllocs()
			var dest map[int64]string
			for i := 0; i < b.N; i++ {
				row := i % rows
				if err := col.scanRowPlain(&dest, row); err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("Int64Float64", func(b *testing.B) {
		col := buildInt64Float64MapColumn(rows, mapSize)

		b.Run("Reflection", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				row := i % rows
				val := col.row(row)
				_ = val.Interface().(map[int64]float64)
			}
		})

		b.Run("FastPath", func(b *testing.B) {
			b.ReportAllocs()
			var dest map[int64]float64
			for i := 0; i < b.N; i++ {
				row := i % rows
				if err := col.scanRowPlain(&dest, row); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
