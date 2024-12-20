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

// Code generated by make codegen DO NOT EDIT.
// source: lib/column/codegen/column.tpl

package column

import (
	"math/big"
	"reflect"
	"strings"
	"fmt"
	"time"
	"net"
	"github.com/google/uuid"
	"github.com/paulmach/orb"
	"github.com/shopspring/decimal"
	"database/sql"
	"database/sql/driver"
	"github.com/ClickHouse/ch-go/proto"
	"github.com/ClickHouse/clickhouse-go/v2/lib/chcol"
)

func (t Type) Column(name string, tz *time.Location) (Interface, error) {
	switch t {
{{- range . }}
	case "{{ .ChType }}":
		return &{{ .ChType }}{name: name}, nil
{{- end }}
	case "Int128":
		return &BigInt{
			size:   16,
			chType: t,
			name:   name,
			signed: true,
			col: &proto.ColInt128{},
		}, nil
	case "UInt128":
		return &BigInt{
			size:   16,
			chType: t,
			name:   name,
			signed: false,
			col: &proto.ColUInt128{},
		}, nil
	case "Int256":
		return &BigInt{
			size:   32,
			chType: t,
			name:   name,
			signed: true,
			col: &proto.ColInt256{},
		}, nil
	case "UInt256":
		return &BigInt{
			size:   32,
			chType: t,
			name:   name,
			signed: false,
			col: &proto.ColUInt256{},
		}, nil
	case "IPv4":
		return &IPv4{name: name}, nil
	case "IPv6":
		return &IPv6{name: name}, nil
	case "Bool", "Boolean":
		return &Bool{name: name}, nil
	case "Date":
		return &Date{name: name, location: tz}, nil
	case "Date32":
		return &Date32{name: name, location: tz}, nil
	case "UUID":
		return &UUID{name: name}, nil
	case "Nothing":
		return &Nothing{name: name}, nil
	case "Ring":
		set, err := (&Array{name: name}).parse("Array(Point)", tz)
        if err != nil {
            return nil, err
        }
        set.chType = "Ring"
        return &Ring{
            set:  set,
            name: name,
        }, nil
	case "Polygon":
		set, err := (&Array{name: name}).parse("Array(Ring)", tz)
        if err != nil {
            return nil, err
        }
        set.chType = "Polygon"
        return &Polygon{
            set:  set,
            name: name,
        }, nil
	case "MultiPolygon":
		set, err := (&Array{name: name}).parse("Array(Polygon)", tz)
        if err != nil {
            return nil, err
        }
        set.chType = "MultiPolygon"
        return &MultiPolygon{
            set:  set,
            name: name,
        }, nil
	case "Point":
		return &Point{name: name}, nil
	case "String":
		return &String{name: name, col: colStrProvider()}, nil
    case "SharedVariant":
        return &SharedVariant{name: name}, nil
	case "Object('json')":
	    return &JSONObject{name: name, root: true, tz: tz}, nil
	}

	switch strType := string(t); {
	case strings.HasPrefix(string(t), "Map("):
		return (&Map{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "Tuple("):
		return (&Tuple{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "Variant("):
		return (&Variant{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "Dynamic"):
		return (&Dynamic{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "JSON"):
		return (&JSON{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "Decimal("):
		return (&Decimal{name: name}).parse(t)
	case strings.HasPrefix(strType, "Nested("):
		return (&Nested{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "Array("):
		return (&Array{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "Interval"):
		return (&Interval{name: name}).parse(t)
	case strings.HasPrefix(string(t), "Nullable"):
		return (&Nullable{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "FixedString"):
		return (&FixedString{name: name}).parse(t)
	case strings.HasPrefix(string(t), "LowCardinality"):
		return (&LowCardinality{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "SimpleAggregateFunction"):
		return (&SimpleAggregateFunction{name: name}).parse(t, tz)
	case strings.HasPrefix(string(t), "Enum8") || strings.HasPrefix(string(t), "Enum16"):
		return Enum(t, name)
	case strings.HasPrefix(string(t), "DateTime64"):
		return (&DateTime64{name: name}).parse(t, tz)
	case strings.HasPrefix(strType, "DateTime") && !strings.HasPrefix(strType, "DateTime64"):
		return (&DateTime{name: name}).parse(t, tz)
	}
	return nil, &UnsupportedColumnTypeError{
		t: t,
	}
}

type (
{{- range . }}
	{{ .ChType }} struct {
	    name string
	    col proto.Col{{ .ChType }}
	}
{{- end }}
)

var (
{{- range . }}
	_ Interface = (*{{ .ChType }})(nil)
{{- end }}
)

var (
	{{- range . }}
		scanType{{ .ChType }} = reflect.TypeOf({{ .GoType }}(0))
	{{- end }}
		scanTypeIP      = reflect.TypeOf(net.IP{})
		scanTypeBool    = reflect.TypeOf(true)
		scanTypeByte    = reflect.TypeOf([]byte{})
		scanTypeUUID    = reflect.TypeOf(uuid.UUID{})
		scanTypeTime    = reflect.TypeOf(time.Time{})
		scanTypeRing    = reflect.TypeOf(orb.Ring{})
		scanTypePoint   = reflect.TypeOf(orb.Point{})
		scanTypeSlice   = reflect.TypeOf([]any{})
		scanTypeMap	    = reflect.TypeOf(map[string]any{})
		scanTypeBigInt  = reflect.TypeOf(&big.Int{})
		scanTypeString  = reflect.TypeOf("")
		scanTypePolygon = reflect.TypeOf(orb.Polygon{})
		scanTypeDecimal = reflect.TypeOf(decimal.Decimal{})
		scanTypeMultiPolygon = reflect.TypeOf(orb.MultiPolygon{})
		scanTypeVariant = reflect.TypeOf(chcol.Variant{})
		scanTypeDynamic = reflect.TypeOf(chcol.Dynamic{})
        scanTypeJSON    = reflect.TypeOf(chcol.JSON{})
	)

{{- range . }}

func (col *{{ .ChType }}) Name() string {
	return col.name
}

func (col *{{ .ChType }}) Type() Type {
	return "{{ .ChType }}"
}

func (col *{{ .ChType }}) ScanType() reflect.Type {
	return scanType{{ .ChType }}
}

func (col *{{ .ChType }}) Rows() int {
	return col.col.Rows()
}

func (col *{{ .ChType }}) Reset() {
    col.col.Reset()
}

func (col *{{ .ChType }}) ScanRow(dest any, row int) error {
	value := col.col.Row(row)
	switch d := dest.(type) {
	case *{{ .GoType }}:
		*d = value
	case **{{ .GoType }}:
		*d = new({{ .GoType }})
		**d = value
	{{- if eq .ChType "Int64" }}
	case *time.Duration:
		*d = time.Duration(value)
	{{- end }}
	{{- if or (eq .ChType "Int64") (eq .ChType "Int32") (eq .ChType "Int16") (eq .ChType "Float64") }}
	case *sql.Null{{ .ChType }}:
		return d.Scan(value)
	{{- end }}
    {{- if or (eq .ChType "Int8") (eq .ChType "UInt8")  }}
	case *bool:
		switch value {
		case 0:
			*d = false
		default:
			*d = true
		}
    {{- end }}
	default:
		if scan, ok := dest.(sql.Scanner); ok {
			return scan.Scan(value)
		}
		return &ColumnConverterError{
			Op:   "ScanRow",
			To:   fmt.Sprintf("%T", dest),
			From: "{{ .ChType }}",
			Hint: fmt.Sprintf("try using *%s", scanType{{ .ChType }}),
		}
	}
	return nil
}

func (col *{{ .ChType }}) Row(i int, ptr bool) any {
	value := col.col.Row(i)
	if ptr {
		return &value
	}
	return value
}

func (col *{{ .ChType }}) Append(v any) (nulls []uint8,err error) {
	switch v := v.(type) {
	case []{{ .GoType }}:
		nulls =  make([]uint8, len(v))
		for i := range v {
			col.col.Append(v[i])
		}
	case []*{{ .GoType }}:
		nulls = make([]uint8, len(v))
		for i := range v {
			switch {
			case v[i] != nil:
				col.col.Append(*v[i])
			default:
				col.col.Append(0)
				nulls[i] = 1
			}
		}
	{{- if or (eq .ChType "Int64") (eq .ChType "Int32") (eq .ChType "Int16") (eq .ChType "Float64") }}
    case []sql.Null{{ .ChType }}:
        nulls = make([]uint8, len(v))
        for i := range v {
            col.AppendRow(v[i])
        }
    case []*sql.Null{{ .ChType }}:
        nulls = make([]uint8, len(v))
        for i := range v {
            if v[i] == nil {
                nulls[i] = 1
            }
            col.AppendRow(v[i])
        }
	{{- end }}
	{{- if eq .ChType "Int8" }}
	case []bool:
		nulls = make([]uint8, len(v))
		for i := range v {
			val := int8(0)
			if v[i] {
				val = 1
			}
			col.col.Append(val)
		}
	case []*bool:
		nulls = make([]uint8, len(v))
		for i := range v {
			val := int8(0)
			if v[i] == nil {
				nulls[i] = 1
			} else if *v[i] {
				val = 1
			}
			col.col.Append(val)
		}
	{{- end }}
	default:

	    if valuer, ok := v.(driver.Valuer); ok {
            val, err := valuer.Value()
            if err != nil {
                return nil, &ColumnConverterError{
                    Op:   "Append",
                    To:   "{{ .ChType }}",
                    From: fmt.Sprintf("%T", v),
                    Hint: "could not get driver.Valuer value",
                }
            }
            return col.Append(val)
        }

		return nil, &ColumnConverterError{
			Op:   "Append",
			To:   "{{ .ChType }}",
			From: fmt.Sprintf("%T", v),
		}
	}
	return
}

func (col *{{ .ChType }}) AppendRow(v any) error {
	switch v := v.(type) {
	case {{ .GoType }}:
		col.col.Append(v)
	case *{{ .GoType }}:
		switch {
        case v != nil:
            col.col.Append(*v)
        default:
            col.col.Append(0)
        }
	case nil:
		col.col.Append(0)
	{{- if eq .ChType "UInt8" }}
	case bool:
		var t uint8
		if v {
			t = 1
		}
		col.col.Append(t)
	{{- end }}
    {{- if or (eq .ChType "Int64") (eq .ChType "Int32") (eq .ChType "Int16") (eq .ChType "Float64") }}
    case sql.Null{{ .ChType }}:
        switch v.Valid {
        case true:
            col.col.Append(v.{{ .ChType }})
        default:
            col.col.Append(0)
        }
    case *sql.Null{{ .ChType }}:
        switch v.Valid {
        case true:
            col.col.Append(v.{{ .ChType }})
        default:
            col.col.Append(0)
        }
    {{- end }}
	{{- if eq .ChType "Int64" }}
    case time.Duration:
        col.col.Append(int64(v))
    case *time.Duration:
        col.col.Append(int64(*v))
	{{- end }}
	{{- if eq .ChType "Int8" }}
    case bool:
        val := int8(0)
        if v {
            val = 1
        }
        col.col.Append(val)
    case *bool:
        val := int8(0)
        if *v {
            val = 1
        }
        col.col.Append(val)
	{{- end }}
	default:

	    if valuer, ok := v.(driver.Valuer); ok {
            val, err := valuer.Value()
            if err != nil {
                return &ColumnConverterError{
                    Op:   "AppendRow",
                    To:   "{{ .ChType }}",
                    From: fmt.Sprintf("%T", v),
                    Hint: "could not get driver.Valuer value",
                }
            }
            return col.AppendRow(val)
        }

		if rv := reflect.ValueOf(v); rv.Kind() == col.ScanType().Kind() || rv.CanConvert(col.ScanType()) {
			col.col.Append(rv.Convert(col.ScanType()).Interface().({{ .GoType }}))
		} else {
			return &ColumnConverterError{
				Op:   "AppendRow",
				To:   "{{ .ChType }}",
				From: fmt.Sprintf("%T", v),
			}
		}
	}
	return nil
}

func (col *{{ .ChType }}) Decode(reader *proto.Reader, rows int) error {
	return col.col.DecodeColumn(reader, rows)
}

func (col *{{ .ChType }}) Encode(buffer *proto.Buffer) {
	col.col.EncodeColumn(buffer)
}

{{- end }}
