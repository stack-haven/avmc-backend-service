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

// Package interval provides a sorted, non-overlapping interval set used by
// lexnorm's State to track Locked regions in O(log n) time.
//
// # Invariants
//
//   - Intervals are half-open: [Start, End).
//   - Intervals are sorted by Start (ascending).
//   - Intervals are non-overlapping: no two intervals share any byte.
//   - Adjacent intervals (e.g., [0,3) and [3,5)) are allowed and kept
//     as distinct entries.
//
// # Complexity
//
//   - Add:        O(n) due to slice insertion (rare operation).
//   - Contains:   O(log n) binary search.
//   - Overlaps:   O(log n + k) where k is the number of overlapping
//     intervals (typically 0 or 1).
//   - Len:        O(1).
//
// # Concurrency
//
// Set is NOT safe for concurrent use. State owns one Set per call and
// does not share it across goroutines.
package interval

import "sort"

// Interval is a half-open range [Start, End).
type Interval struct {
	Start int
	End   int
}

// IsValid reports whether the interval is well-formed (Start ≥ 0 and
// End ≥ Start).
func (i Interval) IsValid() bool {
	return i.Start >= 0 && i.End >= i.Start
}

// Contains reports whether pos is in [Start, End).
func (i Interval) Contains(pos int) bool {
	return pos >= i.Start && pos < i.End
}

// Overlaps reports whether two intervals have non-empty intersection.
//
// Returns false for adjacent (touching but not overlapping) intervals:
// [0,3) does not overlap [3,5).
func (i Interval) Overlaps(other Interval) bool {
	return i.Start < other.End && other.Start < i.End
}

// Set is a sorted, non-overlapping collection of Intervals.
type Set struct {
	intervals []Interval
}

// New returns an empty Set.
func New() *Set {
	return &Set{}
}

// NewWithCapacity returns an empty Set with pre-allocated capacity.
func NewWithCapacity(n int) *Set {
	return &Set{intervals: make([]Interval, 0, n)}
}

// Len returns the number of intervals in the set.
func (s *Set) Len() int {
	return len(s.intervals)
}

// Add inserts the interval [start, end). Returns true on success,
// false if:
//
//   - end < start (invalid interval), or
//   - the interval overlaps an existing entry.
//
// Adjacent intervals are allowed: adding [0,3) after [3,5) succeeds and
// both intervals coexist.
func (s *Set) Add(start, end int) bool {
	if end < start {
		return false
	}

	// Binary search for first interval with Start >= newStart.
	idx := sort.Search(len(s.intervals), func(i int) bool {
		return s.intervals[i].Start >= start
	})

	// Check predecessor: overlap if it ends past newStart.
	if idx > 0 && s.intervals[idx-1].End > start {
		return false
	}

	// Check successor: overlap if it starts before newEnd.
	if idx < len(s.intervals) && s.intervals[idx].Start < end {
		return false
	}

	// Insert at idx.
	s.intervals = append(s.intervals, Interval{})
	copy(s.intervals[idx+1:], s.intervals[idx:])
	s.intervals[idx] = Interval{Start: start, End: end}
	return true
}

// Contains reports whether pos is contained in any interval.
func (s *Set) Contains(pos int) bool {
	// Binary search for first interval with Start > pos.
	idx := sort.Search(len(s.intervals), func(i int) bool {
		return s.intervals[i].Start > pos
	})
	// Predecessor (if any) is the candidate.
	if idx == 0 {
		return false
	}
	return s.intervals[idx-1].Contains(pos)
}

// Overlaps reports whether any interval overlaps [start, end).
//
// Returns false for adjacent intervals: [0,3) does not overlap [3,5).
// Returns false for an empty query [x, x).
func (s *Set) Overlaps(start, end int) bool {
	if end <= start {
		return false
	}
	// Binary search for first interval with End > start (i.e., potential
	// overlap starting at or before query.End).
	idx := sort.Search(len(s.intervals), func(i int) bool {
		return s.intervals[i].End > start
	})
	for i := idx; i < len(s.intervals); i++ {
		if s.intervals[i].Start >= end {
			break // no more overlaps possible (sorted by Start)
		}
		return true
	}
	return false
}

// Intervals returns a copy of the intervals in sorted order.
//
// The returned slice is a defensive copy: callers may modify it without
// affecting the Set.
func (s *Set) Intervals() []Interval {
	out := make([]Interval, len(s.intervals))
	copy(out, s.intervals)
	return out
}

// Reset empties the set, retaining capacity for reuse.
func (s *Set) Reset() {
	s.intervals = s.intervals[:0]
}
