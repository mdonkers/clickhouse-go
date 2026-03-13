package column

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"

	"github.com/ClickHouse/ch-go/proto"
)

// https://github.com/ClickHouse/ClickHouse/blob/master/src/Columns/ColumnMap.cpp
type Map struct {
	keys     Interface
	values   Interface
	chType   Type
	offsets  Int64
	scanType reflect.Type
	name     string
}

type OrderedMap interface {
	Get(key any) (any, bool)
	Put(key any, value any)
	Keys() <-chan any
}

type MapIterator interface {
	Next() bool
	Key() any
	Value() any
}

type IterableOrderedMap interface {
	Put(key any, value any)
	Iterator() MapIterator
}

func (col *Map) Reset() {
	col.keys.Reset()
	col.values.Reset()
	col.offsets.Reset()
}

func (col *Map) Name() string {
	return col.name
}

func (col *Map) parse(t Type, sc *ServerContext) (_ Interface, err error) {
	col.chType = t
	types := make([]string, 2)
	typeParams := t.params()
	idx := strings.Index(typeParams, ",")
	if strings.HasPrefix(typeParams, "Enum") {
		idx = strings.Index(typeParams, "),") + 1
	}
	if idx > 0 {
		types[0] = typeParams[:idx]
		types[1] = typeParams[idx+1:]
	}
	if types[0] != "" && types[1] != "" {
		if col.keys, err = Type(strings.TrimSpace(types[0])).Column(col.name, sc); err != nil {
			return nil, err
		}
		if col.values, err = Type(strings.TrimSpace(types[1])).Column(col.name, sc); err != nil {
			return nil, err
		}

		if col.keys.ScanType().Comparable() {
			col.scanType = reflect.MapOf(
				col.keys.ScanType(),
				col.values.ScanType(),
			)
			return col, nil
		}
	}
	return nil, &UnsupportedColumnTypeError{
		t: t,
	}
}

func (col *Map) Type() Type {
	return col.chType
}

func (col *Map) ScanType() reflect.Type {
	return col.scanType
}

func (col *Map) Rows() int {
	return col.offsets.col.Rows()
}

func (col *Map) Row(i int, ptr bool) any {
	return col.row(i).Interface()
}

func (col *Map) ScanRow(dest any, i int) error {
	if scanner, ok := dest.(sql.Scanner); ok {
		return scanner.Scan(col.row(i).Interface())
	}
	// Fast path: reflection-free scan for common map types
	if err := col.scanRowPlain(dest, i); err == nil {
		return nil
	}
	if om, ok := dest.(IterableOrderedMap); ok {
		keys, values := col.orderedRow(i)
		for i := range keys {
			om.Put(keys[i], values[i])
		}
		return nil
	}
	if om, ok := dest.(OrderedMap); ok {
		keys, values := col.orderedRow(i)
		for i := range keys {
			om.Put(keys[i], values[i])
		}
		return nil
	}
	value := reflect.Indirect(reflect.ValueOf(dest))
	if value.Type() == col.scanType {
		value.Set(col.row(i))
		return nil
	}
	return &ColumnConverterError{
		Op:   "ScanRow",
		To:   fmt.Sprintf("%T", dest),
		From: string(col.chType),
		Hint: fmt.Sprintf("try using %s", col.scanType),
	}
}

func (col *Map) Append(v any) (nulls []uint8, err error) {
	value := reflect.Indirect(reflect.ValueOf(v))
	if value.Kind() != reflect.Slice {
		if valuer, ok := v.(driver.Valuer); ok {
			val, err := valuer.Value()
			if err != nil {
				return nil, &ColumnConverterError{
					Op:   "Append",
					To:   string(col.chType),
					From: fmt.Sprintf("%T", v),
					Hint: fmt.Sprintf("could not get driver.Valuer value, try using %s", col.scanType),
				}
			}
			return col.Append(val)
		}
		return nil, &ColumnConverterError{
			Op:   "Append",
			To:   string(col.chType),
			From: fmt.Sprintf("%T", v),
			Hint: fmt.Sprintf("try using %s", col.scanType),
		}
	}
	for i := 0; i < value.Len(); i++ {
		if err := col.AppendRow(value.Index(i).Interface()); err != nil {
			return nil, err
		}
	}
	return
}

func (col *Map) AppendRow(v any) error {
	if v == nil {
		// NOTE: successful Map.parse() make sure we have
		// valid col.scanType
		v = reflect.Zero(col.scanType).Interface()
	}

	value := reflect.Indirect(reflect.ValueOf(v))
	if value.Type() == col.scanType {
		var (
			size int64
			iter = value.MapRange()
		)
		for iter.Next() {
			size++
			if err := col.keys.AppendRow(iter.Key().Interface()); err != nil {
				return err
			}
			if err := col.values.AppendRow(iter.Value().Interface()); err != nil {
				return err
			}
		}
		var prev int64
		if n := col.offsets.Rows(); n != 0 {
			prev = col.offsets.col.Row(n - 1)
		}
		col.offsets.col.Append(prev + size)
		return nil
	}

	if orderedMap, ok := v.(IterableOrderedMap); ok {
		var size int64
		iter := orderedMap.Iterator()
		for iter.Next() {
			key, value := iter.Key(), iter.Value()
			size++
			if err := col.keys.AppendRow(key); err != nil {
				return err
			}
			if err := col.values.AppendRow(value); err != nil {
				return err
			}
		}
		var prev int64
		if n := col.offsets.Rows(); n != 0 {
			prev = col.offsets.col.Row(n - 1)
		}
		col.offsets.col.Append(prev + size)
		return nil
	}

	if orderedMap, ok := v.(OrderedMap); ok {
		var size int64
		for key := range orderedMap.Keys() {
			value, ok := orderedMap.Get(key)
			if !ok {
				return fmt.Errorf("ordered map has key %v but no corresponding value", key)
			}
			size++
			if err := col.keys.AppendRow(key); err != nil {
				return err
			}
			if err := col.values.AppendRow(value); err != nil {
				return err
			}
		}
		var prev int64
		if n := col.offsets.Rows(); n != 0 {
			prev = col.offsets.col.Row(n - 1)
		}
		col.offsets.col.Append(prev + size)
		return nil
	}

	if valuer, ok := v.(driver.Valuer); ok {
		val, err := valuer.Value()
		if err != nil {
			return &ColumnConverterError{
				Op:   "AppendRow",
				To:   string(col.chType),
				From: fmt.Sprintf("%T", v),
				Hint: fmt.Sprintf("could not get driver.Valuer value, try using %s", col.scanType),
			}
		}
		return col.AppendRow(val)
	}

	return &ColumnConverterError{
		Op:   "AppendRow",
		To:   string(col.chType),
		From: fmt.Sprintf("%T", v),
		Hint: fmt.Sprintf("try using %s", col.scanType),
	}

}

func (col *Map) Decode(reader *proto.Reader, rows int) error {
	if err := col.offsets.col.DecodeColumn(reader, rows); err != nil {
		return err
	}
	if i := col.offsets.Rows(); i != 0 {
		size := int(col.offsets.col.Row(i - 1))
		if err := col.keys.Decode(reader, size); err != nil {
			return err
		}
		return col.values.Decode(reader, size)
	}
	return nil
}

func (col *Map) Encode(buffer *proto.Buffer) {
	col.offsets.col.EncodeColumn(buffer)
	col.keys.Encode(buffer)
	col.values.Encode(buffer)
}

func (col *Map) ReadStatePrefix(reader *proto.Reader) error {
	if serialize, ok := col.keys.(CustomSerialization); ok {
		if err := serialize.ReadStatePrefix(reader); err != nil {
			return err
		}
	}
	if serialize, ok := col.values.(CustomSerialization); ok {
		if err := serialize.ReadStatePrefix(reader); err != nil {
			return err
		}
	}
	return nil
}

func (col *Map) WriteStatePrefix(encoder *proto.Buffer) error {
	if serialize, ok := col.keys.(CustomSerialization); ok {
		if err := serialize.WriteStatePrefix(encoder); err != nil {
			return err
		}
	}
	if serialize, ok := col.values.(CustomSerialization); ok {
		if err := serialize.WriteStatePrefix(encoder); err != nil {
			return err
		}
	}
	return nil
}

var errMapFastPathUnsupported = fmt.Errorf("unsupported type for map fast path")

// scanRowPlain is a reflection-free scan for common map types.
// It dispatches on the key column type, then the value column type,
// then verifies the destination type matches.
func (col *Map) scanRowPlain(dest any, i int) error {
	switch kc := col.keys.(type) {
	case *String:
		return scanMapStringKey(col, dest, i, &kc.col)
	case *Float32:
		return scanMapTypedKey(col, dest, i, []float32(kc.col))
	case *Float64:
		return scanMapTypedKey(col, dest, i, []float64(kc.col))
	case *Int8:
		return scanMapTypedKey(col, dest, i, []int8(kc.col))
	case *Int16:
		return scanMapTypedKey(col, dest, i, []int16(kc.col))
	case *Int32:
		return scanMapTypedKey(col, dest, i, []int32(kc.col))
	case *Int64:
		return scanMapTypedKey(col, dest, i, []int64(kc.col))
	case *UInt8:
		return scanMapTypedKey(col, dest, i, []uint8(kc.col))
	case *UInt16:
		return scanMapTypedKey(col, dest, i, []uint16(kc.col))
	case *UInt32:
		return scanMapTypedKey(col, dest, i, []uint32(kc.col))
	case *UInt64:
		return scanMapTypedKey(col, dest, i, []uint64(kc.col))
	case *Bool:
		return scanMapTypedKey(col, dest, i, []bool(kc.col))
	}
	return errMapFastPathUnsupported
}

// mapRange returns the start (inclusive) and end (exclusive) indices
// in the flattened key/value arrays for map row i.
func (col *Map) mapRange(i int) (int, int) {
	end := int(col.offsets.col.Row(i))
	start := 0
	if i > 0 {
		start = int(col.offsets.col.Row(i - 1))
	}
	return start, end
}

// scanMapStringKey handles maps with String keys. String keys need special
// handling because proto.ColStr stores data in columnar format and requires
// Row(i) to access individual values, unlike numeric types which are Go slices.
func scanMapStringKey(col *Map, dest any, i int, keys *proto.ColStr) error {
	start, end := col.mapRange(i)
	switch vc := col.values.(type) {
	case *String:
		d, ok := dest.(*map[string]string)
		if !ok {
			return errMapFastPathUnsupported
		}
		m := make(map[string]string, end-start)
		for j := start; j < end; j++ {
			m[keys.Row(j)] = vc.col.Row(j)
		}
		*d = m
		return nil
	case *Float32:
		d, ok := dest.(*map[string]float32)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []float32(vc.col))
		return nil
	case *Float64:
		d, ok := dest.(*map[string]float64)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []float64(vc.col))
		return nil
	case *Int8:
		d, ok := dest.(*map[string]int8)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []int8(vc.col))
		return nil
	case *Int16:
		d, ok := dest.(*map[string]int16)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []int16(vc.col))
		return nil
	case *Int32:
		d, ok := dest.(*map[string]int32)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []int32(vc.col))
		return nil
	case *Int64:
		d, ok := dest.(*map[string]int64)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []int64(vc.col))
		return nil
	case *UInt8:
		d, ok := dest.(*map[string]uint8)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []uint8(vc.col))
		return nil
	case *UInt16:
		d, ok := dest.(*map[string]uint16)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []uint16(vc.col))
		return nil
	case *UInt32:
		d, ok := dest.(*map[string]uint32)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []uint32(vc.col))
		return nil
	case *UInt64:
		d, ok := dest.(*map[string]uint64)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []uint64(vc.col))
		return nil
	case *Bool:
		d, ok := dest.(*map[string]bool)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapStringKeyTypedVal(d, start, end, keys, []bool(vc.col))
		return nil
	}
	return errMapFastPathUnsupported
}

func scanMapStringKeyTypedVal[V any](dest *map[string]V, start, end int, keys *proto.ColStr, vals []V) {
	m := make(map[string]V, end-start)
	for j := start; j < end; j++ {
		m[keys.Row(j)] = vals[j]
	}
	*dest = m
}

// scanMapTypedKey handles maps with typed (numeric/bool) keys backed by Go slices.
func scanMapTypedKey[K comparable](col *Map, dest any, i int, keys []K) error {
	start, end := col.mapRange(i)
	switch vc := col.values.(type) {
	case *String:
		d, ok := dest.(*map[K]string)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyStringVal(d, start, end, keys, &vc.col)
		return nil
	case *Float32:
		d, ok := dest.(*map[K]float32)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []float32(vc.col))
		return nil
	case *Float64:
		d, ok := dest.(*map[K]float64)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []float64(vc.col))
		return nil
	case *Int8:
		d, ok := dest.(*map[K]int8)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []int8(vc.col))
		return nil
	case *Int16:
		d, ok := dest.(*map[K]int16)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []int16(vc.col))
		return nil
	case *Int32:
		d, ok := dest.(*map[K]int32)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []int32(vc.col))
		return nil
	case *Int64:
		d, ok := dest.(*map[K]int64)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []int64(vc.col))
		return nil
	case *UInt8:
		d, ok := dest.(*map[K]uint8)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []uint8(vc.col))
		return nil
	case *UInt16:
		d, ok := dest.(*map[K]uint16)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []uint16(vc.col))
		return nil
	case *UInt32:
		d, ok := dest.(*map[K]uint32)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []uint32(vc.col))
		return nil
	case *UInt64:
		d, ok := dest.(*map[K]uint64)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []uint64(vc.col))
		return nil
	case *Bool:
		d, ok := dest.(*map[K]bool)
		if !ok {
			return errMapFastPathUnsupported
		}
		scanMapTypedKeyTypedVal(d, start, end, keys, []bool(vc.col))
		return nil
	}
	return errMapFastPathUnsupported
}

func scanMapTypedKeyTypedVal[K comparable, V any](dest *map[K]V, start, end int, keys []K, vals []V) {
	m := make(map[K]V, end-start)
	for j := start; j < end; j++ {
		m[keys[j]] = vals[j]
	}
	*dest = m
}

func scanMapTypedKeyStringVal[K comparable](dest *map[K]string, start, end int, keys []K, vals *proto.ColStr) {
	m := make(map[K]string, end-start)
	for j := start; j < end; j++ {
		m[keys[j]] = vals.Row(j)
	}
	*dest = m
}

func (col *Map) row(n int) reflect.Value {
	var (
		prev  int64
		value = reflect.MakeMap(col.scanType)
	)
	if n != 0 {
		prev = col.offsets.col.Row(n - 1)
	}
	var (
		size = int(col.offsets.col.Row(n) - prev)
		from = int(prev)
	)
	for next := 0; next < size; next++ {
		mapValue := col.values.Row(from+next, false)
		var mapReflectValue reflect.Value
		if mapValue == nil {
			// Convert any nil to typed nil (such as nil *string) to preserve map element
			// https://github.com/ClickHouse/clickhouse-go/issues/1515
			mapReflectValue = reflect.New(value.Type().Elem()).Elem()
		} else {
			mapReflectValue = reflect.ValueOf(mapValue)
		}

		value.SetMapIndex(
			reflect.ValueOf(col.keys.Row(from+next, false)),
			mapReflectValue,
		)
	}
	return value
}

func (col *Map) orderedRow(n int) ([]any, []any) {
	var prev int64
	if n != 0 {
		prev = col.offsets.col.Row(n - 1)
	}
	var (
		size = int(col.offsets.col.Row(n) - prev)
		from = int(prev)
	)
	keys := make([]any, size)
	values := make([]any, size)
	for next := 0; next < size; next++ {
		keys[next] = col.keys.Row(from+next, false)
		values[next] = col.values.Row(from+next, false)
	}
	return keys, values
}

var (
	_ Interface           = (*Map)(nil)
	_ CustomSerialization = (*Map)(nil)
)
