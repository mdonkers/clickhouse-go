// SPDX-FileCopyrightText: Copyright 2024-2026 Dash0 Inc.

package column

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strconv"
	"testing"
)

// buildFloat64ArrayColumn constructs an Array(Float64) column with the given
// number of rows, each containing arrSize elements. This simulates what
// clickhouse-go receives from the wire after decoding.
func buildFloat64ArrayColumn(rows, arrSize int) *Array {
	valuesCol := &Float64{name: "test"}
	scanType := reflect.SliceOf(valuesCol.ScanType())
	offsetCol := &offset{
		values:   UInt64{},
		scanType: scanType,
	}

	cumOffset := uint64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < arrSize; j++ {
			valuesCol.col = append(valuesCol.col, rand.Float64())
		}
		cumOffset += uint64(arrSize)
		offsetCol.values.col.Append(cumOffset)
	}

	return &Array{
		depth:    1,
		chType:   "Array(Float64)",
		values:   valuesCol,
		offsets:  []*offset{offsetCol},
		scanType: scanType,
	}
}

func buildStringArrayColumn(rows, arrSize int) *Array {
	valuesCol := &String{name: "test"}
	scanType := reflect.SliceOf(valuesCol.ScanType())
	offsetCol := &offset{
		values:   UInt64{},
		scanType: scanType,
	}

	cumOffset := uint64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < arrSize; j++ {
			valuesCol.col.Append("val-" + strconv.Itoa(r*arrSize+j))
		}
		cumOffset += uint64(arrSize)
		offsetCol.values.col.Append(cumOffset)
	}

	return &Array{
		depth:    1,
		chType:   "Array(String)",
		values:   valuesCol,
		offsets:  []*offset{offsetCol},
		scanType: scanType,
	}
}

// TestArrayScanRowPlain_EmptyArray verifies that scanning an empty array returns
// a non-nil empty slice ([]T{}), not nil. This matches the reflection path which
// always uses reflect.MakeSlice.
func TestArrayScanRowPlain_EmptyArray(t *testing.T) {
	valuesCol := &Float64{name: "test"}
	scanType := reflect.SliceOf(valuesCol.ScanType())
	offsetCol := &offset{
		values:   UInt64{},
		scanType: scanType,
	}
	offsetCol.values.col.Append(0) // row 0: empty array

	col := &Array{
		depth:    1,
		chType:   "Array(Float64)",
		values:   valuesCol,
		offsets:  []*offset{offsetCol},
		scanType: scanType,
	}

	var dest []float64
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	if dest == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(dest) != 0 {
		t.Fatalf("expected empty slice, got %v", dest)
	}
}

// TestArrayScanRowPlain_EmptyStringArray is the same as above but for the
// scanRowPlainString path.
func TestArrayScanRowPlain_EmptyStringArray(t *testing.T) {
	valuesCol := &String{name: "test"}
	scanType := reflect.SliceOf(valuesCol.ScanType())
	offsetCol := &offset{
		values:   UInt64{},
		scanType: scanType,
	}
	offsetCol.values.col.Append(0)

	col := &Array{
		depth:    1,
		chType:   "Array(String)",
		values:   valuesCol,
		offsets:  []*offset{offsetCol},
		scanType: scanType,
	}

	var dest []string
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	if dest == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(dest) != 0 {
		t.Fatalf("expected empty slice, got %v", dest)
	}
}

// TestArrayScanRowPlain_NoAliasing verifies that scanning multiple rows into the
// same variable does not corrupt previously saved results. Each scan must return
// an independent slice with its own backing array.
func TestArrayScanRowPlain_NoAliasing(t *testing.T) {
	col := buildFloat64ArrayColumn(3, 4)

	var dest []float64

	// Scan row 0 and save a reference
	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	saved0 := dest
	copy0 := make([]float64, len(saved0))
	copy(copy0, saved0)

	// Scan row 1 into the same variable
	if err := col.ScanRow(&dest, 1); err != nil {
		t.Fatal(err)
	}
	saved1 := dest

	// Scan row 2 into the same variable
	if err := col.ScanRow(&dest, 2); err != nil {
		t.Fatal(err)
	}

	// saved0 must still contain the original row 0 data
	if !reflect.DeepEqual(saved0, copy0) {
		t.Fatalf("saved reference to row 0 was corrupted: got %v, want %v", saved0, copy0)
	}

	// saved1 must not equal dest (row 2) unless the data happens to match
	if len(saved1) != 4 {
		t.Fatalf("saved1 length changed: got %d, want 4", len(saved1))
	}
}

// TestArrayScanRowPlain_NoAliasingString is the same as above but exercises the
// scanRowPlainString code path.
func TestArrayScanRowPlain_NoAliasingString(t *testing.T) {
	col := buildStringArrayColumn(3, 4)

	var dest []string

	if err := col.ScanRow(&dest, 0); err != nil {
		t.Fatal(err)
	}
	saved0 := dest
	copy0 := make([]string, len(saved0))
	copy(copy0, saved0)

	if err := col.ScanRow(&dest, 1); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saved0, copy0) {
		t.Fatalf("saved reference to row 0 was corrupted: got %v, want %v", saved0, copy0)
	}
}

// TestArrayScanRowPlain_NumericConversion verifies that scanning a column of one
// numeric type into a destination slice of a different numeric type works correctly
// via the reflection-free conversion path.
func TestArrayScanRowPlain_NumericConversion(t *testing.T) {
	t.Run("Int64_to_Float64", func(t *testing.T) {
		valuesCol := &Int64{name: "test"}
		scanType := reflect.SliceOf(valuesCol.ScanType())
		offsetCol := &offset{values: UInt64{}, scanType: scanType}

		valuesCol.col = append(valuesCol.col, 10, 20, 30) // row 0
		offsetCol.values.col.Append(3)
		valuesCol.col = append(valuesCol.col, -5, 100) // row 1
		offsetCol.values.col.Append(5)

		col := &Array{
			depth: 1, chType: "Array(Int64)", values: valuesCol,
			offsets: []*offset{offsetCol}, scanType: scanType,
		}

		var dest []float64
		if err := col.ScanRow(&dest, 0); err != nil {
			t.Fatal(err)
		}
		expected := []float64{10, 20, 30}
		if !reflect.DeepEqual(dest, expected) {
			t.Fatalf("row 0: got %v, want %v", dest, expected)
		}

		if err := col.ScanRow(&dest, 1); err != nil {
			t.Fatal(err)
		}
		expected = []float64{-5, 100}
		if !reflect.DeepEqual(dest, expected) {
			t.Fatalf("row 1: got %v, want %v", dest, expected)
		}
	})

	t.Run("Float64_to_Int32", func(t *testing.T) {
		valuesCol := &Float64{name: "test"}
		scanType := reflect.SliceOf(valuesCol.ScanType())
		offsetCol := &offset{values: UInt64{}, scanType: scanType}

		valuesCol.col = append(valuesCol.col, 1.9, 2.1, 3.0)
		offsetCol.values.col.Append(3)

		col := &Array{
			depth: 1, chType: "Array(Float64)", values: valuesCol,
			offsets: []*offset{offsetCol}, scanType: scanType,
		}

		var dest []int32
		if err := col.ScanRow(&dest, 0); err != nil {
			t.Fatal(err)
		}
		// Go numeric conversion truncates toward zero
		expected := []int32{1, 2, 3}
		if !reflect.DeepEqual(dest, expected) {
			t.Fatalf("got %v, want %v", dest, expected)
		}
	})

	t.Run("UInt32_to_Int64", func(t *testing.T) {
		valuesCol := &UInt32{name: "test"}
		scanType := reflect.SliceOf(valuesCol.ScanType())
		offsetCol := &offset{values: UInt64{}, scanType: scanType}

		valuesCol.col = append(valuesCol.col, 0, 42, 4294967295) // max uint32
		offsetCol.values.col.Append(3)

		col := &Array{
			depth: 1, chType: "Array(UInt32)", values: valuesCol,
			offsets: []*offset{offsetCol}, scanType: scanType,
		}

		var dest []int64
		if err := col.ScanRow(&dest, 0); err != nil {
			t.Fatal(err)
		}
		expected := []int64{0, 42, 4294967295}
		if !reflect.DeepEqual(dest, expected) {
			t.Fatalf("got %v, want %v", dest, expected)
		}
	})

	t.Run("Int8_to_Float32", func(t *testing.T) {
		valuesCol := &Int8{name: "test"}
		scanType := reflect.SliceOf(valuesCol.ScanType())
		offsetCol := &offset{values: UInt64{}, scanType: scanType}

		valuesCol.col = append(valuesCol.col, -128, 0, 127)
		offsetCol.values.col.Append(3)

		col := &Array{
			depth: 1, chType: "Array(Int8)", values: valuesCol,
			offsets: []*offset{offsetCol}, scanType: scanType,
		}

		var dest []float32
		if err := col.ScanRow(&dest, 0); err != nil {
			t.Fatal(err)
		}
		expected := []float32{-128, 0, 127}
		if !reflect.DeepEqual(dest, expected) {
			t.Fatalf("got %v, want %v", dest, expected)
		}
	})

	t.Run("EmptyArray_Conversion", func(t *testing.T) {
		valuesCol := &Int64{name: "test"}
		scanType := reflect.SliceOf(valuesCol.ScanType())
		offsetCol := &offset{values: UInt64{}, scanType: scanType}
		offsetCol.values.col.Append(0)

		col := &Array{
			depth: 1, chType: "Array(Int64)", values: valuesCol,
			offsets: []*offset{offsetCol}, scanType: scanType,
		}

		var dest []float64
		if err := col.ScanRow(&dest, 0); err != nil {
			t.Fatal(err)
		}
		if dest == nil {
			t.Fatal("expected non-nil empty slice, got nil")
		}
		if len(dest) != 0 {
			t.Fatalf("expected empty slice, got %v", dest)
		}
	})

	t.Run("Bool_column_no_conversion", func(t *testing.T) {
		// Bool columns should NOT be convertible to numeric slices via the fast path;
		// they should fall through to the reflection path.
		valuesCol := &Bool{name: "test"}
		scanType := reflect.SliceOf(valuesCol.ScanType())
		offsetCol := &offset{values: UInt64{}, scanType: scanType}

		valuesCol.col = append(valuesCol.col, true, false)
		offsetCol.values.col.Append(2)

		col := &Array{
			depth: 1, chType: "Array(Bool)", values: valuesCol,
			offsets: []*offset{offsetCol}, scanType: scanType,
		}

		// Scanning Bool into *[]uint8 should fail in the plain path
		// (and fall through to reflection in ScanRow)
		var dest []uint8
		err := col.scanRowPlain(&dest, 0)
		if err == nil {
			t.Fatal("expected error for Bool→uint8 conversion in plain path")
		}
	})
}

func BenchmarkArrayScanRow(b *testing.B) {
	for _, arrSize := range []int{10, 50, 160, 500} {
		rows := 1000
		col := buildFloat64ArrayColumn(rows, arrSize)

		b.Run(fmt.Sprintf("Reflection/size=%d", arrSize), func(b *testing.B) {
			b.ReportAllocs()
			var dest []float64
			for i := 0; i < b.N; i++ {
				row := i % rows
				// Force reflection path by going through scan() directly
				val, err := col.scan(col.ScanType(), row)
				if err != nil {
					b.Fatal(err)
				}
				dest = val.Interface().([]float64)
			}
			_ = dest
		})

		b.Run(fmt.Sprintf("FastPath/size=%d", arrSize), func(b *testing.B) {
			b.ReportAllocs()
			dest := make([]float64, 0, arrSize)
			for i := 0; i < b.N; i++ {
				row := i % rows
				if err := col.scanRowPlain(&dest, row); err != nil {
					b.Fatal(err)
				}
			}
			_ = dest
		})

		b.Run(fmt.Sprintf("ScanRow/size=%d", arrSize), func(b *testing.B) {
			b.ReportAllocs()
			dest := make([]float64, 0, arrSize)
			for i := 0; i < b.N; i++ {
				row := i % rows
				if err := col.ScanRow(&dest, row); err != nil {
					b.Fatal(err)
				}
			}
			_ = dest
		})
	}
}

// BenchmarkArrayScanRowInt32 benchmarks scanning Array(Int32) to verify the
// fast-path works for integer types too.
func BenchmarkArrayScanRowInt32(b *testing.B) {
	arrSize := 160
	rows := 1000

	valuesCol := &Int32{name: "test"}
	scanType := reflect.SliceOf(valuesCol.ScanType())
	offsetCol := &offset{
		values:   UInt64{},
		scanType: scanType,
	}
	cumOffset := uint64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < arrSize; j++ {
			valuesCol.col = append(valuesCol.col, rand.Int32())
		}
		cumOffset += uint64(arrSize)
		offsetCol.values.col.Append(cumOffset)
	}
	col := &Array{
		depth:    1,
		chType:   "Array(Int32)",
		values:   valuesCol,
		offsets:  []*offset{offsetCol},
		scanType: scanType,
	}

	b.Run("Reflection", func(b *testing.B) {
		b.ReportAllocs()
		var dest []int32
		for i := 0; i < b.N; i++ {
			row := i % rows
			val, err := col.scan(col.ScanType(), row)
			if err != nil {
				b.Fatal(err)
			}
			dest = val.Interface().([]int32)
		}
		_ = dest
	})

	b.Run("FastPath", func(b *testing.B) {
		b.ReportAllocs()
		dest := make([]int32, 0, arrSize)
		for i := 0; i < b.N; i++ {
			row := i % rows
			if err := col.scanRowPlain(&dest, row); err != nil {
				b.Fatal(err)
			}
		}
		_ = dest
	})
}

// BenchmarkArrayScanUInt64 benchmarks scanning Array(UInt64) which is used for
// offset columns internally. Verifies fast-path works for the common uint64 case.
func BenchmarkArrayScanUInt64(b *testing.B) {
	arrSize := 160
	rows := 1000

	valuesCol := &UInt64{name: "test"}
	scanType := reflect.SliceOf(valuesCol.ScanType())
	offsetCol := &offset{
		values:   UInt64{},
		scanType: scanType,
	}
	cumOffset := uint64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < arrSize; j++ {
			valuesCol.col = append(valuesCol.col, rand.Uint64())
		}
		cumOffset += uint64(arrSize)
		offsetCol.values.col.Append(cumOffset)
	}
	col := &Array{
		depth:    1,
		chType:   "Array(UInt64)",
		values:   valuesCol,
		offsets:  []*offset{offsetCol},
		scanType: scanType,
	}

	b.Run("Reflection", func(b *testing.B) {
		b.ReportAllocs()
		var dest []uint64
		for i := 0; i < b.N; i++ {
			row := i % rows
			val, err := col.scan(col.ScanType(), row)
			if err != nil {
				b.Fatal(err)
			}
			dest = val.Interface().([]uint64)
		}
		_ = dest
	})

	b.Run("FastPath", func(b *testing.B) {
		b.ReportAllocs()
		dest := make([]uint64, 0, arrSize)
		for i := 0; i < b.N; i++ {
			row := i % rows
			if err := col.scanRowPlain(&dest, row); err != nil {
				b.Fatal(err)
			}
		}
		_ = dest
	})
}

// BenchmarkArrayScanBool benchmarks scanning Array(Bool).
func BenchmarkArrayScanBool(b *testing.B) {
	arrSize := 160
	rows := 1000

	valuesCol := &Bool{name: "test"}
	scanType := reflect.SliceOf(valuesCol.ScanType())
	offsetCol := &offset{
		values:   UInt64{},
		scanType: scanType,
	}
	cumOffset := uint64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < arrSize; j++ {
			valuesCol.col = append(valuesCol.col, rand.IntN(2) == 1)
		}
		cumOffset += uint64(arrSize)
		offsetCol.values.col.Append(cumOffset)
	}
	col := &Array{
		depth:    1,
		chType:   "Array(Bool)",
		values:   valuesCol,
		offsets:  []*offset{offsetCol},
		scanType: scanType,
	}

	b.Run("Reflection", func(b *testing.B) {
		b.ReportAllocs()
		var dest []bool
		for i := 0; i < b.N; i++ {
			row := i % rows
			val, err := col.scan(col.ScanType(), row)
			if err != nil {
				b.Fatal(err)
			}
			dest = val.Interface().([]bool)
		}
		_ = dest
	})

	b.Run("FastPath", func(b *testing.B) {
		b.ReportAllocs()
		dest := make([]bool, 0, arrSize)
		for i := 0; i < b.N; i++ {
			row := i % rows
			if err := col.scanRowPlain(&dest, row); err != nil {
				b.Fatal(err)
			}
		}
		_ = dest
	})
}

// BenchmarkArrayScanString benchmarks scanning Array(String).
func BenchmarkArrayScanString(b *testing.B) {
	arrSize := 160
	rows := 1000

	valuesCol := &String{name: "test"}
	scanType := reflect.SliceOf(valuesCol.ScanType())
	offsetCol := &offset{
		values:   UInt64{},
		scanType: scanType,
	}
	cumOffset := uint64(0)
	for r := 0; r < rows; r++ {
		for j := 0; j < arrSize; j++ {
			valuesCol.col.Append("value-" + strconv.Itoa(r*arrSize+j))
		}
		cumOffset += uint64(arrSize)
		offsetCol.values.col.Append(cumOffset)
	}
	col := &Array{
		depth:    1,
		chType:   "Array(String)",
		values:   valuesCol,
		offsets:  []*offset{offsetCol},
		scanType: scanType,
	}

	b.Run("Reflection", func(b *testing.B) {
		b.ReportAllocs()
		var dest []string
		for i := 0; i < b.N; i++ {
			row := i % rows
			val, err := col.scan(col.ScanType(), row)
			if err != nil {
				b.Fatal(err)
			}
			dest = val.Interface().([]string)
		}
		_ = dest
	})

	b.Run("FastPath", func(b *testing.B) {
		b.ReportAllocs()
		dest := make([]string, 0, arrSize)
		for i := 0; i < b.N; i++ {
			row := i % rows
			if err := col.scanRowPlain(&dest, row); err != nil {
				b.Fatal(err)
			}
		}
		_ = dest
	})
}

// BenchmarkArrayScanAllocation focuses on measuring allocation behavior
// specifically for the fast-path with pre-allocated buffers vs fresh allocations.
func BenchmarkArrayScanAllocation(b *testing.B) {
	arrSize := 160
	rows := 1000
	col := buildFloat64ArrayColumn(rows, arrSize)

	b.Run("FastPath/PreAllocated", func(b *testing.B) {
		b.ReportAllocs()
		dest := make([]float64, 0, arrSize)
		for i := 0; i < b.N; i++ {
			row := i % rows
			if err := col.scanRowPlain(&dest, row); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("FastPath/FreshAlloc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			row := i % rows
			var dest []float64
			if err := col.scanRowPlain(&dest, row); err != nil {
				b.Fatal(err)
			}
		}
	})
}
