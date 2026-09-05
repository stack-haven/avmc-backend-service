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
	"context"
	"errors"
	"math"
	"testing"

	"github.com/stack-haven/lexnorm"
)

// ----------------------------------------------------------------------------
// Construction
// ----------------------------------------------------------------------------

func TestNewState_Valid(t *testing.T) {
	s, err := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	if err != nil {
		t.Fatalf("NewState returned error: %v", err)
	}
	if s.Text() != "hello world" {
		t.Errorf("Text() = %q, want %q", s.Text(), "hello world")
	}
	if s.Original() != "hello world" {
		t.Errorf("Original() = %q, want %q", s.Original(), "hello world")
	}
	if s.Config().AutoApplyThreshold != 0.95 {
		t.Errorf("Config not preserved")
	}
}

func TestNewState_NilContext(t *testing.T) {
	_, err := lexnorm.NewState(nil, "text", nil, lexnorm.DefaultConfig())
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("nil context must return ErrInvalidConfig, got %v", err)
	}
}

func TestNewState_InvalidConfig(t *testing.T) {
	cfg := lexnorm.DefaultConfig()
	cfg.AutoApplyThreshold = -0.1
	_, err := lexnorm.NewState(context.Background(), "text", nil, cfg)
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("invalid config must return ErrInvalidConfig, got %v", err)
	}
}

func TestNewState_ExceedsMaxTextBytes(t *testing.T) {
	cfg := lexnorm.DefaultConfig()
	cfg.MaxTextBytes = 5
	_, err := lexnorm.NewState(context.Background(), "longer than 5 bytes", nil, cfg)
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("text > MaxTextBytes must return ErrInvalidConfig, got %v", err)
	}
}

func TestNewState_EmptyOriginal(t *testing.T) {
	// Empty original is valid (no-op for normalize).
	s, err := lexnorm.NewState(context.Background(), "", nil, lexnorm.DefaultConfig())
	if err != nil {
		t.Errorf("empty original must succeed, got %v", err)
	}
	if s.Text() != "" {
		t.Error("Text() of empty state must be empty")
	}
}

// ----------------------------------------------------------------------------
// Replace
// ----------------------------------------------------------------------------

func TestState_Replace_Basic(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	err := s.Replace(lexnorm.Span{0, 5}, "HI", lexnorm.ChangeMeta{Confidence: 1.0})
	if err != nil {
		t.Fatalf("Replace failed: %v", err)
	}
	if got := s.Text(); got != "HI world" {
		t.Errorf("Text() = %q, want %q", got, "HI world")
	}
	changes := s.Changes()
	if len(changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(changes))
	}
	c := changes[0]
	if c.From != "hello" || c.To != "HI" {
		t.Errorf("Change From/To mismatch: %q/%q", c.From, c.To)
	}
	if !c.Applied {
		t.Error("Change must be Applied=true")
	}
	if c.Action != lexnorm.ActionReplace {
		t.Errorf("Action = %v, want ActionReplace", c.Action)
	}
}

func TestState_Replace_Multiple(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "0123456789", nil, lexnorm.DefaultConfig())
	// Replace [0,3) → "AAA" (lengthDelta = 0)
	// Replace [5,7) → "BB"  (lengthDelta = -1)
	// Replace [8,10) → "CC" (lengthDelta = -1)
	if err := s.Replace(lexnorm.Span{0, 3}, "AAA", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(lexnorm.Span{5, 7}, "BB", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(lexnorm.Span{8, 10}, "CC", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	// Expected Text: AAA 34 BB 7 CC
	if got := s.Text(); got != "AAA34BB7CC" {
		t.Errorf("Text() = %q, want %q", got, "AAA34BB7CC")
	}
	if got := len(s.Changes()); got != 3 {
		t.Errorf("len(Changes) = %d, want 3", got)
	}
}

func TestState_Replace_LongerAndShorter(t *testing.T) {
	// Replace a 3-byte span with a longer 5-byte string.
	s, _ := lexnorm.NewState(context.Background(), "abcXYZ", nil, lexnorm.DefaultConfig())
	if err := s.Replace(lexnorm.Span{0, 3}, "AAAAA", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "AAAAAXYZ" {
		t.Errorf("Text() = %q, want %q", got, "AAAAAXYZ")
	}

	// Then replace [3,8) (covering "AAXYZ" in Text) with shorter "Z".
	// Original span = [3,6) (covering "XYZ").
	if err := s.Replace(lexnorm.Span{3, 6}, "Z", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "AAAAAZ" {
		t.Errorf("Text() = %q, want %q", got, "AAAAAZ")
	}
}

func TestState_Replace_UTF8Multibyte(t *testing.T) {
	// "你好世界" = 4 runes × 3 bytes = 12 bytes.
	// Span {3, 6} covers "好" (the second rune).
	s, _ := lexnorm.NewState(context.Background(), "你好世界", nil, lexnorm.DefaultConfig())
	if err := s.Replace(lexnorm.Span{3, 6}, "✓", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	// Expected: "你" (3 bytes) + "✓" (3 bytes UTF-8) + "世界" (6 bytes) = "你✓世界"
	if got := s.Text(); got != "你✓世界" {
		t.Errorf("Text() = %q, want %q", got, "你✓世界")
	}
}

func TestState_Replace_OverlappingReplaced(t *testing.T) {
	// Per spec: Replaced regions are "consumed"; re-Replace on overlapping
	// Original span returns ErrConflict.
	s, _ := lexnorm.NewState(context.Background(), "0123456789", nil, lexnorm.DefaultConfig())
	if err := s.Replace(lexnorm.Span{3, 6}, "XX", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	// Now try Replace on an Original span that overlaps the previous.
	err := s.Replace(lexnorm.Span{4, 7}, "YY", lexnorm.ChangeMeta{Confidence: 1.0})
	if !errors.Is(err, lexnorm.ErrConflict) {
		t.Errorf("overlapping replace must return ErrConflict, got %v", err)
	}
}

func TestState_Replace_InvalidSpan_NegativeStart(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())
	err := s.Replace(lexnorm.Span{-1, 2}, "x", lexnorm.ChangeMeta{Confidence: 1.0})
	if !errors.Is(err, lexnorm.ErrInvalidSpan) {
		t.Errorf("negative Start must return ErrInvalidSpan, got %v", err)
	}
}

func TestState_Replace_InvalidSpan_EndBeforeStart(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())
	err := s.Replace(lexnorm.Span{3, 1}, "x", lexnorm.ChangeMeta{Confidence: 1.0})
	if !errors.Is(err, lexnorm.ErrInvalidSpan) {
		t.Errorf("End < Start must return ErrInvalidSpan, got %v", err)
	}
}

func TestState_Replace_InvalidSpan_OutOfRange(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())
	err := s.Replace(lexnorm.Span{0, 100}, "x", lexnorm.ChangeMeta{Confidence: 1.0})
	if !errors.Is(err, lexnorm.ErrInvalidSpan) {
		t.Errorf("End > len(original) must return ErrInvalidSpan, got %v", err)
	}
}

func TestState_Replace_InvalidConfidence(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())

	tests := []struct {
		name string
		c    float64
	}{
		{"negative", -0.01},
		{"above 1", 1.01},
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"-Inf", math.Inf(-1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := s.Replace(lexnorm.Span{0, 1}, "x", lexnorm.ChangeMeta{Confidence: tc.c})
			if !errors.Is(err, lexnorm.ErrInvalidConfig) {
				t.Errorf("Confidence=%v must return ErrInvalidConfig, got %v", tc.c, err)
			}
		})
	}
}

func TestState_Replace_EmptyToString(t *testing.T) {
	// Replace with "" is a deletion (still Action=ActionReplace but To="").
	s, _ := lexnorm.NewState(context.Background(), "abcdef", nil, lexnorm.DefaultConfig())
	err := s.Replace(lexnorm.Span{2, 4}, "", lexnorm.ChangeMeta{Confidence: 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "abef" {
		t.Errorf("Text() = %q, want %q", got, "abef")
	}
}

// ----------------------------------------------------------------------------
// Suggest
// ----------------------------------------------------------------------------

func TestState_Suggest_Basic(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	err := s.Suggest(lexnorm.Span{0, 5}, "HI", lexnorm.ChangeMeta{Confidence: 0.7})
	if err != nil {
		t.Fatalf("Suggest failed: %v", err)
	}
	// Text must NOT change.
	if got := s.Text(); got != "hello world" {
		t.Errorf("Text() after Suggest = %q, want %q (unchanged)", got, "hello world")
	}
	// Change must be recorded with Applied=false.
	changes := s.Changes()
	if len(changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(changes))
	}
	if changes[0].Applied {
		t.Error("Suggest must produce Applied=false")
	}
	if changes[0].Action != lexnorm.ActionSuggest {
		t.Errorf("Action = %v, want ActionSuggest", changes[0].Action)
	}
}

func TestState_Suggest_AllowedOnLocked(t *testing.T) {
	// Per spec: Suggest does NOT conflict with Locked regions.
	s, _ := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	if err := s.Lock(lexnorm.Span{0, 5}); err != nil {
		t.Fatal(err)
	}
	if err := s.Suggest(lexnorm.Span{0, 5}, "HI", lexnorm.ChangeMeta{Confidence: 0.5}); err != nil {
		t.Errorf("Suggest on Locked must succeed, got %v", err)
	}
}

func TestState_Suggest_InvalidSpan(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())
	err := s.Suggest(lexnorm.Span{0, 100}, "x", lexnorm.ChangeMeta{Confidence: 0.5})
	if !errors.Is(err, lexnorm.ErrInvalidSpan) {
		t.Errorf("invalid span must return ErrInvalidSpan, got %v", err)
	}
}

func TestState_Suggest_InvalidConfidence(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())
	err := s.Suggest(lexnorm.Span{0, 1}, "x", lexnorm.ChangeMeta{Confidence: math.NaN()})
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("NaN confidence must return ErrInvalidConfig, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// Lock
// ----------------------------------------------------------------------------

func TestState_Lock_Basic(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	if err := s.Lock(lexnorm.Span{0, 5}); err != nil {
		t.Fatal(err)
	}
	if !s.IsLocked(lexnorm.Span{0, 5}) {
		t.Error("IsLocked(self) must be true")
	}
	if !s.IsLocked(lexnorm.Span{2, 4}) {
		t.Error("IsLocked(inside) must be true")
	}
	if !s.IsLocked(lexnorm.Span{0, 3}) {
		t.Error("IsLocked(partial-left) must be true (overlap)")
	}
	if s.IsLocked(lexnorm.Span{5, 8}) {
		t.Error("IsLocked(after) must be false")
	}
	if s.IsLocked(lexnorm.Span{8, 11}) {
		t.Error("IsLocked(disjoint) must be false")
	}
}

func TestState_Lock_ConflictsWithExisting(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	if err := s.Lock(lexnorm.Span{0, 5}); err != nil {
		t.Fatal(err)
	}
	err := s.Lock(lexnorm.Span{3, 8})
	if !errors.Is(err, lexnorm.ErrConflict) {
		t.Errorf("overlapping Lock must return ErrConflict, got %v", err)
	}
}

func TestState_Lock_Adjacent(t *testing.T) {
	// Adjacent (touching but not overlapping) is allowed.
	s, _ := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	if err := s.Lock(lexnorm.Span{0, 3}); err != nil {
		t.Fatal(err)
	}
	if err := s.Lock(lexnorm.Span{3, 5}); err != nil {
		t.Errorf("adjacent Lock must succeed, got %v", err)
	}
}

func TestState_Lock_InvalidSpan(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())
	tests := []struct {
		name string
		span lexnorm.Span
	}{
		{"negative", lexnorm.Span{-1, 2}},
		{"end before start", lexnorm.Span{3, 1}},
		{"out of range", lexnorm.Span{0, 100}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := s.Lock(tc.span)
			if !errors.Is(err, lexnorm.ErrInvalidSpan) {
				t.Errorf("must return ErrInvalidSpan, got %v", err)
			}
		})
	}
}

func TestState_IsLocked_InvalidSpanReturnsFalse(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())
	if s.IsLocked(lexnorm.Span{-1, 2}) {
		t.Error("IsLocked with invalid span must return false")
	}
	if s.IsLocked(lexnorm.Span{0, 100}) {
		t.Error("IsLocked with out-of-range span must return false")
	}
}

// ----------------------------------------------------------------------------
// Replace vs Lock conflict
// ----------------------------------------------------------------------------

func TestState_Replace_OnLockedRegion(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	if err := s.Lock(lexnorm.Span{0, 5}); err != nil {
		t.Fatal(err)
	}
	err := s.Replace(lexnorm.Span{0, 5}, "HI", lexnorm.ChangeMeta{Confidence: 1.0})
	if !errors.Is(err, lexnorm.ErrConflict) {
		t.Errorf("Replace on Locked must return ErrConflict, got %v", err)
	}
	// Text must be unchanged.
	if got := s.Text(); got != "hello world" {
		t.Errorf("Text after failed Replace = %q, want unchanged", got)
	}
	// No Change recorded.
	if len(s.Changes()) != 0 {
		t.Errorf("failed Replace must not record Change, got %d", len(s.Changes()))
	}
}

func TestState_Replace_PartialOverlapWithLocked(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	if err := s.Lock(lexnorm.Span{0, 5}); err != nil {
		t.Fatal(err)
	}
	err := s.Replace(lexnorm.Span{3, 8}, "XX", lexnorm.ChangeMeta{Confidence: 1.0})
	if !errors.Is(err, lexnorm.ErrConflict) {
		t.Errorf("partial overlap with Locked must return ErrConflict, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// Offset stability
// ----------------------------------------------------------------------------

func TestState_Replace_OffsetStability(t *testing.T) {
	// After multiple Replace calls, Span in subsequent calls must still
	// refer to Original positions (not current Text positions).
	s, _ := lexnorm.NewState(context.Background(), "ABCDEFGHIJ", nil, lexnorm.DefaultConfig())
	// Original {2,5} = "CDE" → "X"
	if err := s.Replace(lexnorm.Span{2, 5}, "X", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	// Text = "ABXFGHIJ" (length 8).
	// Original {6,8} = "GH" → "Y" (Original-based, NOT Text-based).
	if err := s.Replace(lexnorm.Span{6, 8}, "Y", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatalf("Replace after offset shift: %v", err)
	}
	if got := s.Text(); got != "ABXFYIJ" {
		t.Errorf("Text() = %q, want %q", got, "ABXFYIJ")
	}
}

func TestState_OrigToText_BoundaryCases(t *testing.T) {
	// Document the origToText mapping behavior across many replacements.
	s, _ := lexnorm.NewState(context.Background(), "0123456789", nil, lexnorm.DefaultConfig())
	// Insert "AA" at {0,0} (insertion at start, lengthDelta=+2).
	if err := s.Replace(lexnorm.Span{0, 0}, "AA", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	// Text = "AA0123456789" (length 12).
	if got := s.Text(); got != "AA0123456789" {
		t.Errorf("Text after insertion = %q, want %q", got, "AA0123456789")
	}
	// Original {5,7} = "56" (Original position, despite Text being shifted).
	if err := s.Replace(lexnorm.Span{5, 7}, "Z", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	// Original[5,7] = "56" maps to Text[7,9] (after +2 shift from insertion).
	// Replacing Text[7,9] with "Z" → "AA01234" + "Z" + "789" = "AA01234Z789".
	if got := s.Text(); got != "AA01234Z789" {
		t.Errorf("Text after second replace = %q, want %q", got, "AA01234Z789")
	}
}

// ----------------------------------------------------------------------------
// Changes accumulation
// ----------------------------------------------------------------------------

func TestState_Changes_DefensiveCopy(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())
	if err := s.Replace(lexnorm.Span{0, 1}, "X", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}

	changes := s.Changes()
	changes[0].From = "modified"

	// Internal state must be unaffected.
	changes2 := s.Changes()
	if changes2[0].From == "modified" {
		t.Error("Changes() must return defensive copy")
	}
}

func TestState_Changes_Order(t *testing.T) {
	// Changes must be recorded in call order.
	s, _ := lexnorm.NewState(context.Background(), "01234", nil, lexnorm.DefaultConfig())
	if err := s.Replace(lexnorm.Span{0, 1}, "A", lexnorm.ChangeMeta{Confidence: 0.9}); err != nil {
		t.Fatal(err)
	}
	if err := s.Suggest(lexnorm.Span{2, 3}, "C", lexnorm.ChangeMeta{Confidence: 0.5}); err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(lexnorm.Span{4, 5}, "E", lexnorm.ChangeMeta{Confidence: 0.8}); err != nil {
		t.Fatal(err)
	}

	changes := s.Changes()
	if len(changes) != 3 {
		t.Fatalf("len(Changes) = %d, want 3", len(changes))
	}
	if changes[0].To != "A" || !changes[0].Applied {
		t.Error("first change should be A applied")
	}
	if changes[1].To != "C" || changes[1].Applied {
		t.Error("second change should be C suggested")
	}
	if changes[2].To != "E" || !changes[2].Applied {
		t.Error("third change should be E applied")
	}
}

// ----------------------------------------------------------------------------
// Concurrent safety
// ----------------------------------------------------------------------------

func TestState_NotConcurrentSafe_Documented(t *testing.T) {
	// State is documented as single-goroutine exclusive.
	// This test documents the contract: concurrent Replace on the same
	// State has undefined behavior (we don't test that here).
	// What we DO test: independent State objects are independent.
	s1, _ := lexnorm.NewState(context.Background(), "abc", nil, lexnorm.DefaultConfig())
	s2, _ := lexnorm.NewState(context.Background(), "xyz", nil, lexnorm.DefaultConfig())

	if err := s1.Replace(lexnorm.Span{0, 1}, "A", lexnorm.ChangeMeta{Confidence: 1.0}); err != nil {
		t.Fatal(err)
	}
	if s2.Text() != "xyz" {
		t.Error("independent States must not affect each other")
	}
}

// ----------------------------------------------------------------------------
// Context handling
// ----------------------------------------------------------------------------

func TestState_Context_Preserved(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "value")
	s, err := lexnorm.NewState(ctx, "text", nil, lexnorm.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if s.Context().Value(ctxKey{}) != "value" {
		t.Error("State must preserve context")
	}
}
