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

package interval_test

import (
	"testing"

	"github.com/stack-haven/lexnorm/internal/interval"
)

func TestInterval_IsValid(t *testing.T) {
	tests := []struct {
		i     interval.Interval
		valid bool
	}{
		{interval.Interval{0, 0}, true}, // empty but valid
		{interval.Interval{0, 3}, true},
		{interval.Interval{5, 10}, true},
		{interval.Interval{-1, 3}, false}, // negative start
		{interval.Interval{5, 3}, false},  // end < start
	}
	for _, tc := range tests {
		if got := tc.i.IsValid(); got != tc.valid {
			t.Errorf("Interval{%d,%d}.IsValid() = %v, want %v",
				tc.i.Start, tc.i.End, got, tc.valid)
		}
	}
}

func TestInterval_Overlaps(t *testing.T) {
	tests := []struct {
		i, j     interval.Interval
		overlaps bool
	}{
		{interval.Interval{0, 5}, interval.Interval{0, 5}, true},  // identical
		{interval.Interval{0, 5}, interval.Interval{3, 8}, true},  // partial
		{interval.Interval{0, 10}, interval.Interval{3, 5}, true}, // containment
		{interval.Interval{0, 3}, interval.Interval{3, 5}, false}, // adjacent (touching)
		{interval.Interval{0, 3}, interval.Interval{5, 8}, false}, // disjoint
	}
	for _, tc := range tests {
		if got := tc.i.Overlaps(tc.j); got != tc.overlaps {
			t.Errorf("Interval{%d,%d}.Overlaps(Interval{%d,%d}) = %v, want %v",
				tc.i.Start, tc.i.End, tc.j.Start, tc.j.End, got, tc.overlaps)
		}
	}
}

func TestSet_New_Empty(t *testing.T) {
	s := interval.New()
	if got := s.Len(); got != 0 {
		t.Errorf("empty Set.Len() = %d, want 0", got)
	}
	if s.Contains(0) {
		t.Error("empty Set.Contains(0) = true, want false")
	}
	if s.Overlaps(0, 5) {
		t.Error("empty Set.Overlaps(0,5) = true, want false")
	}
}

func TestSet_Add_Single(t *testing.T) {
	s := interval.New()
	if !s.Add(0, 5) {
		t.Fatal("Add(0,5) failed")
	}
	if got := s.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
	if !s.Contains(0) || !s.Contains(4) {
		t.Error("Contains should be true for positions inside the interval")
	}
	if s.Contains(5) || s.Contains(-1) {
		t.Error("Contains should be false for positions outside")
	}
	if !s.Overlaps(3, 8) {
		t.Error("Overlaps(3,8) should be true")
	}
	if s.Overlaps(5, 8) {
		t.Error("Overlaps(5,8) should be false (adjacent)")
	}
}

func TestSet_Add_Disjoint(t *testing.T) {
	s := interval.New()
	if !s.Add(0, 3) {
		t.Fatal("Add(0,3) failed")
	}
	if !s.Add(5, 8) {
		t.Fatal("Add(5,8) failed")
	}
	if !s.Add(10, 15) {
		t.Fatal("Add(10,15) failed")
	}
	if got := s.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
}

func TestSet_Add_Adjacent(t *testing.T) {
	// Adjacent intervals are allowed (touching but not overlapping).
	s := interval.New()
	if !s.Add(0, 3) {
		t.Fatal("Add(0,3) failed")
	}
	if !s.Add(3, 5) {
		t.Fatal("Add(3,5) should succeed (adjacent)")
	}
	if got := s.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2 (adjacent should not merge)", got)
	}
}

func TestSet_Add_Overlap(t *testing.T) {
	s := interval.New()
	if !s.Add(0, 5) {
		t.Fatal("Add(0,5) failed")
	}
	tests := []struct {
		start, end int
	}{
		{0, 3},  // exact prefix
		{3, 8},  // partial right
		{-2, 3}, // partial left
		{2, 4},  // inside
		{0, 5},  // identical
		{-1, 6}, // superset
	}
	for _, tc := range tests {
		if s.Add(tc.start, tc.end) {
			t.Errorf("Add(%d,%d) should fail (overlaps existing)", tc.start, tc.end)
		}
	}
	// Original interval still there.
	if got := s.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1 (failed inserts should not affect state)", got)
	}
}

func TestSet_Add_Invalid(t *testing.T) {
	s := interval.New()
	if s.Add(5, 3) {
		t.Error("Add(5,3) with end < start should fail")
	}
	if s.Add(0, -1) {
		t.Error("Add(0,-1) with negative end should fail")
	}
	if got := s.Len(); got != 0 {
		t.Errorf("Len() = %d after invalid inserts, want 0", got)
	}
}

func TestSet_Add_InsertionOrder(t *testing.T) {
	// Insertion in any order should produce a sorted result.
	s := interval.New()
	s.Add(10, 15)
	s.Add(0, 3)
	s.Add(5, 8)

	got := s.Intervals()
	want := []interval.Interval{
		{0, 3}, {5, 8}, {10, 15},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Intervals()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestSet_Contains_Many(t *testing.T) {
	s := interval.New()
	s.Add(0, 3)
	s.Add(10, 15)
	s.Add(20, 25)

	tests := []struct {
		pos      int
		contains bool
	}{
		{0, true}, {2, true}, {3, false},
		{5, false}, {10, true}, {14, true}, {15, false},
		{18, false}, {20, true}, {25, false},
	}
	for _, tc := range tests {
		if got := s.Contains(tc.pos); got != tc.contains {
			t.Errorf("Contains(%d) = %v, want %v", tc.pos, got, tc.contains)
		}
	}
}

func TestSet_Overlaps_EmptyQuery(t *testing.T) {
	s := interval.New()
	s.Add(0, 5)
	if s.Overlaps(3, 3) {
		t.Error("Overlaps(3,3) (empty) should be false")
	}
}

func TestSet_Intervals_CopyIsIndependent(t *testing.T) {
	s := interval.New()
	s.Add(0, 5)

	got := s.Intervals()
	got[0] = interval.Interval{99, 99} // modify the returned slice

	// Internal state must be unchanged.
	if !s.Contains(2) {
		t.Error("modifying returned slice must not affect internal state")
	}
}

func TestSet_Reset(t *testing.T) {
	s := interval.New()
	s.Add(0, 5)
	s.Add(10, 15)
	s.Reset()
	if got := s.Len(); got != 0 {
		t.Errorf("after Reset: Len() = %d, want 0", got)
	}
	if s.Contains(2) {
		t.Error("after Reset: Contains should be false")
	}
	// Should be reusable.
	if !s.Add(0, 5) {
		t.Error("after Reset: Add should work")
	}
}

func TestSet_LargeScale(t *testing.T) {
	// Stress test: 1000 disjoint intervals.
	s := interval.NewWithCapacity(1000)
	for i := 0; i < 1000; i++ {
		start := i * 10
		end := start + 5
		if !s.Add(start, end) {
			t.Fatalf("Add(%d,%d) failed at iteration %d", start, end, i)
		}
	}
	if got := s.Len(); got != 1000 {
		t.Errorf("Len() = %d, want 1000", got)
	}
	// Spot-check overlaps.
	if !s.Overlaps(0, 100) {
		t.Error("Overlaps(0,100) should be true")
	}
	if s.Overlaps(6, 9) {
		t.Error("Overlaps(6,9) should be false (gap between [0,5) and [10,15))")
	}
}
