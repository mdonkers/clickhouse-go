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
	"reflect"
	"testing"
)

var appendStringCases = []string{
	"", "a", "service.name", "hello world",
	"with\x00\tcontrol", "unicode 世界 🚀", `"quotes"and\backslash`,
}

// TestString_AppendString verifies AppendString produces the same column contents
// as AppendRow for string inputs.
func TestString_AppendString(t *testing.T) {
	var typed, boxed String
	for _, v := range appendStringCases {
		if err := typed.AppendString(v); err != nil {
			t.Fatalf("AppendString(%q): %v", v, err)
		}
		if err := boxed.AppendRow(v); err != nil {
			t.Fatalf("AppendRow(%q): %v", v, err)
		}
	}
	if typed.Rows() != len(appendStringCases) {
		t.Fatalf("Rows: got %d, want %d", typed.Rows(), len(appendStringCases))
	}
	for i, want := range appendStringCases {
		if got := typed.col.Row(i); got != want {
			t.Errorf("AppendString row %d: got %q, want %q", i, got, want)
		}
		if got := typed.col.Row(i); got != boxed.col.Row(i) {
			t.Errorf("row %d: AppendString %q != AppendRow %q", i, got, boxed.col.Row(i))
		}
	}
}

// TestLowCardinality_AppendString verifies AppendString drives the dictionary
// exactly like AppendRow's string fast path, keeping strIndex and the boxed index
// (which Encode reads for the dictionary size) in sync.
func TestLowCardinality_AppendString(t *testing.T) {
	keys := []string{"alpha", "beta", "alpha", "gamma", "beta", "alpha"}

	typed := newLCString("k")
	boxed := newLCString("k")
	for _, k := range keys {
		if err := typed.AppendString(k); err != nil {
			t.Fatalf("AppendString(%q): %v", k, err)
		}
		if err := boxed.AppendRow(k); err != nil {
			t.Fatalf("AppendRow(%q): %v", k, err)
		}
	}

	if typed.Rows() != len(keys) {
		t.Fatalf("Rows: got %d, want %d", typed.Rows(), len(keys))
	}
	// 3 unique dictionary entries.
	if got := len(typed.append.strIndex); got != 3 {
		t.Fatalf("strIndex unique: got %d, want 3", got)
	}
	// index must stay in sync with strIndex; Encode writes len(index) as the
	// additional-keys count, so a divergence would corrupt the wire format.
	if len(typed.append.index) != len(typed.append.strIndex) {
		t.Fatalf("index (%d) and strIndex (%d) must match",
			len(typed.append.index), len(typed.append.strIndex))
	}
	// Resolved key indices must match the boxed AppendRow path exactly.
	if !reflect.DeepEqual(typed.append.keys, boxed.append.keys) {
		t.Fatalf("keys: AppendString %v != AppendRow %v", typed.append.keys, boxed.append.keys)
	}
}

func TestLowCardinality_AppendString_Reset(t *testing.T) {
	lc := newLCString("k")
	_ = lc.AppendString("pre")
	lc.Reset()
	if lc.Rows() != 0 || len(lc.append.strIndex) != 0 || len(lc.append.index) != 0 {
		t.Fatalf("state not cleared after Reset: rows=%d strIndex=%d index=%d",
			lc.Rows(), len(lc.append.strIndex), len(lc.append.index))
	}
	if err := lc.AppendString("post"); err != nil {
		t.Fatalf("AppendString after Reset: %v", err)
	}
	if lc.Rows() != 1 {
		t.Fatalf("Rows after post-reset append: got %d, want 1", lc.Rows())
	}
}

var benchAppendKeys = []string{
	"service.name", "http.method", "http.status_code", "db.system",
	"net.peer.name", "messaging.system",
}

// BenchmarkString_Append compares the boxed AppendRow(any) path against the typed
// AppendString(string) path; the latter avoids the per-row runtime.convTstring alloc.
func BenchmarkString_Append(b *testing.B) {
	b.Run("AppendRow_boxed", func(b *testing.B) {
		b.ReportAllocs()
		var col String
		for i := 0; i < b.N; i++ {
			_ = col.AppendRow(benchAppendKeys[i%len(benchAppendKeys)])
		}
	})
	b.Run("AppendString_typed", func(b *testing.B) {
		b.ReportAllocs()
		var col String
		for i := 0; i < b.N; i++ {
			_ = col.AppendString(benchAppendKeys[i%len(benchAppendKeys)])
		}
	})
}

// BenchmarkLowCardinality_Append compares boxed AppendRow against typed AppendString
// on a warm dictionary (cache hits), where AppendString is allocation-free.
func BenchmarkLowCardinality_Append(b *testing.B) {
	b.Run("AppendRow_boxed", func(b *testing.B) {
		b.ReportAllocs()
		lc := newLCString("k")
		for i := 0; i < b.N; i++ {
			_ = lc.AppendRow(benchAppendKeys[i%len(benchAppendKeys)])
		}
	})
	b.Run("AppendString_typed", func(b *testing.B) {
		b.ReportAllocs()
		lc := newLCString("k")
		for i := 0; i < b.N; i++ {
			_ = lc.AppendString(benchAppendKeys[i%len(benchAppendKeys)])
		}
	})
}
