// Copyright 2024 The Ark Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lexnorm_test

import (
	"testing"

	"github.com/stack-haven/lexnorm"
)

func TestSpan_IsZero(t *testing.T) {
	if !(lexnorm.Span{Start: 0, End: 0}).IsZero() {
		t.Error("Span{0,0} must be zero")
	}
	if (lexnorm.Span{Start: 0, End: 1}).IsZero() {
		t.Error("Span{0,1} must NOT be zero")
	}
	if (lexnorm.Span{Start: 5, End: 10}).IsZero() {
		t.Error("Span{5,10} must NOT be zero")
	}
}

func TestSpan_IsEmpty(t *testing.T) {
	if !(lexnorm.Span{Start: 5, End: 5}).IsEmpty() {
		t.Error("Span{5,5} must be empty")
	}
	if (lexnorm.Span{Start: 0, End: 0}).IsEmpty() != true {
		t.Error("Span{0,0} must be empty")
	}
	if (lexnorm.Span{Start: 0, End: 3}).IsEmpty() {
		t.Error("Span{0,3} must NOT be empty")
	}
}

func TestSpan_Len(t *testing.T) {
	tests := []struct {
		s    lexnorm.Span
		want int
	}{
		{lexnorm.Span{0, 0}, 0},
		{lexnorm.Span{0, 3}, 3},
		{lexnorm.Span{5, 10}, 5},
		// Invalid span (End < Start) returns 0, no panic.
		{lexnorm.Span{10, 5}, 0},
	}
	for _, tc := range tests {
		if got := tc.s.Len(); got != tc.want {
			t.Errorf("Span{%d,%d}.Len() = %d, want %d", tc.s.Start, tc.s.End, got, tc.want)
		}
	}
}

func TestSpan_IsValid(t *testing.T) {
	tests := []struct {
		s     lexnorm.Span
		valid bool
	}{
		{lexnorm.Span{0, 0}, true},
		{lexnorm.Span{0, 3}, true},
		{lexnorm.Span{5, 10}, true},
		// Invalid: Start < 0
		{lexnorm.Span{-1, 3}, false},
		// Invalid: End < Start
		{lexnorm.Span{5, 3}, false},
		// Edge: Start == End (empty but valid)
		{lexnorm.Span{7, 7}, true},
	}
	for _, tc := range tests {
		if got := tc.s.IsValid(); got != tc.valid {
			t.Errorf("Span{%d,%d}.IsValid() = %v, want %v",
				tc.s.Start, tc.s.End, got, tc.valid)
		}
	}
}

func TestSpan_Contains(t *testing.T) {
	s := lexnorm.Span{Start: 5, End: 10}

	tests := []struct {
		pos      int
		contains bool
	}{
		{4, false},  // before
		{5, true},   // Start (inclusive)
		{7, true},   // middle
		{9, true},   // End-1 (last included)
		{10, false}, // End (exclusive)
		{15, false}, // after
		{-1, false},
	}
	for _, tc := range tests {
		if got := s.Contains(tc.pos); got != tc.contains {
			t.Errorf("Span{5,10}.Contains(%d) = %v, want %v", tc.pos, got, tc.contains)
		}
	}
}

func TestSpan_Overlaps(t *testing.T) {
	tests := []struct {
		s, other lexnorm.Span
		overlap  bool
	}{
		// Identical → overlap
		{lexnorm.Span{0, 5}, lexnorm.Span{0, 5}, true},
		// Adjacent (touching but not overlapping) → no overlap
		{lexnorm.Span{0, 3}, lexnorm.Span{3, 5}, false},
		// Partial overlap
		{lexnorm.Span{0, 5}, lexnorm.Span{3, 8}, true},
		// Containment
		{lexnorm.Span{0, 10}, lexnorm.Span{3, 5}, true},
		// Disjoint
		{lexnorm.Span{0, 3}, lexnorm.Span{5, 8}, false},
		// Reversed
		{lexnorm.Span{5, 8}, lexnorm.Span{0, 3}, false},
	}
	for _, tc := range tests {
		got := tc.s.Overlaps(tc.other)
		revGot := tc.other.Overlaps(tc.s)
		if got != tc.overlap {
			t.Errorf("Span{%d,%d}.Overlaps(Span{%d,%d}) = %v, want %v",
				tc.s.Start, tc.s.End, tc.other.Start, tc.other.End, got, tc.overlap)
		}
		if got != revGot {
			t.Errorf("Overlaps is not symmetric: Span{%d,%d}.Overlaps(Span{%d,%d}) = %v, reverse = %v",
				tc.s.Start, tc.s.End, tc.other.Start, tc.other.End, got, revGot)
		}
	}
}

func TestSpan_String(t *testing.T) {
	tests := []struct {
		s    lexnorm.Span
		want string
	}{
		{lexnorm.Span{0, 0}, "Span[0,0)"},
		{lexnorm.Span{0, 3}, "Span[0,3)"},
		{lexnorm.Span{5, 10}, "Span[5,10)"},
		{lexnorm.Span{100, 200}, "Span[100,200)"},
	}
	for _, tc := range tests {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Span{%d,%d}.String() = %q, want %q",
				tc.s.Start, tc.s.End, got, tc.want)
		}
	}
}

func TestSpan_UTF8Offsets_Documentation(t *testing.T) {
	// This test documents (not enforces) that Span offsets are UTF-8 byte
	// offsets, not rune indices. For "你好" (6 bytes, 2 runes):
	//   Span covering "好" is Span{3, 6}, NOT Span{1, 2}.
	text := "你好世界"
	goodSpan := lexnorm.Span{Start: 3, End: 6} // "好" (bytes 3..6)
	if goodSpan.Len() != 3 {
		t.Errorf("'好' is 3 UTF-8 bytes, got Len() = %d", goodSpan.Len())
	}
	if !goodSpan.IsValid() {
		t.Error("Span{3,6} must be valid for '你好世界'")
	}
	// Total length of "你好世界" is 12 bytes (4 runes × 3 bytes each).
	if len(text) != 12 {
		t.Errorf("'你好世界' must be 12 UTF-8 bytes, got %d", len(text))
	}
}
