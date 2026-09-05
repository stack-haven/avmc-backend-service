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

package lexnorm

import "fmt"

// Span represents a half-open byte range [Start, End) in the Original text.
//
// # Offset Semantics (Architecture Invariant)
//
// Span offsets are UTF-8 byte offsets in the **Original** text passed to
// Engine.Normalize — NOT offsets in the current State.Text(). State
// maintains an internal Original→Text offset mapping so that Processors
// always reason about Original positions.
//
// # Bounds
//
// A valid Span must satisfy 0 ≤ Start ≤ End ≤ len(Original). The methods
// Len, IsValid, Contains, and Overlaps do not panic on out-of-range
// values; callers should validate via IsValid before use.
//
// # Half-Open Semantics
//
// End is exclusive: a Span with Start=0, End=3 covers bytes 0, 1, 2 (the
// first 3 bytes of Original). An empty Span has Start == End.
type Span struct {
	// Start is the inclusive lower bound (byte offset in Original).
	Start int
	// End is the exclusive upper bound (byte offset in Original).
	End int
}

// IsZero reports whether s is the zero value (Span{0, 0}).
func (s Span) IsZero() bool {
	return s.Start == 0 && s.End == 0
}

// IsEmpty reports whether s covers zero bytes (Start == End).
//
// Note: an invalid Span (End < Start) returns true for IsEmpty via Len,
// but IsEmpty explicitly checks Start == End.
func (s Span) IsEmpty() bool {
	return s.Start == s.End
}

// Len returns the byte length of s (End - Start).
//
// Returns 0 if End < Start (invalid span), without panicking.
func (s Span) Len() int {
	if s.End < s.Start {
		return 0
	}
	return s.End - s.Start
}

// IsValid reports whether s is well-formed: Start ≥ 0 and End ≥ Start.
//
// IsValid does NOT check that End ≤ len(Original); that comparison
// requires the Original text, which Span does not carry. Callers
// (specifically State.Replace/Suggest/Lock) perform the length check
// and return ErrInvalidSpan on failure.
func (s Span) IsValid() bool {
	return s.Start >= 0 && s.End >= s.Start
}

// Contains reports whether pos is within s.
//
// pos is treated as a byte offset into Original. Returns false if pos is
// outside the half-open range or if s is invalid.
func (s Span) Contains(pos int) bool {
	return pos >= s.Start && pos < s.End
}

// Overlaps reports whether s and other have non-empty intersection.
//
// Returns false for adjacent (touching but not overlapping) spans:
// {0,3} does not overlap {3,5}.
func (s Span) Overlaps(other Span) bool {
	return s.Start < other.End && other.Start < s.End
}

// String returns a debug-friendly representation "Span[Start,End)".
func (s Span) String() string {
	return fmt.Sprintf("Span[%d,%d)", s.Start, s.End)
}
