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

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/stack-haven/lexnorm/internal/interval"
	"github.com/stack-haven/lexnorm/lexicon"
)

// State is the per-request working area for a single Normalize call.
//
// # Invariants (Architecture)
//
//   - State is exclusive to a single goroutine (invariant I4).
//   - State.Text() returns the current text after all applied changes.
//   - State.Original() returns the input text (immutable).
//   - State methods never expose internal slice references.
//
// # Span Semantics
//
// All Span arguments are interpreted as Original UTF-8 byte offsets,
// NOT current Text offsets. State maintains an internal Original→Text
// offset map (via the sorted replacements list).
//
// # Error Returns (1.2 Decision)
//
// All mutating methods (Replace, Suggest, Lock) return an error so
// callers can detect invalid input without inspecting return values
// (D2 consistency).
type State struct {
	ctx context.Context

	original []byte // immutable after construction
	text     []byte // current text (modified by Replace)

	// replacements is sorted by origStart (ascending). Used to map
	// Original positions to current Text positions.
	replacements []replacement

	locked  *interval.Set
	changes []Change
	steps   []StepTiming // populated by Engine during runProcessors
	lexicon lexicon.Lexicon
	config  Config

	// ctxCanceled is set when ctx.Err() is non-nil at the time of a
	// mutating call. We don't actively poll ctx in every method, but we
	// record the first cancellation so the Engine can surface it.
	ctxCanceled bool
}

// replacement records one applied Replace so we can map Original
// positions to current Text positions.
type replacement struct {
	origStart int // Original Start
	origEnd   int // Original End (exclusive)
	newLen    int // length of replacement text in Text
}

// NewState creates a State for the given Original text.
//
// Returns ErrInvalidConfig (wrapped) when:
//   - ctx is nil
//   - cfg.Validate() fails
//   - len(original) > cfg.MaxTextBytes (when MaxTextBytes > 0)
//
// On success, the returned State is ready to receive Replace / Suggest /
// Lock calls.
func NewState(ctx context.Context, original string, lex lexicon.Lexicon, cfg Config) (*State, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil context: %w", ErrInvalidConfig)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.MaxTextBytes > 0 && len(original) > cfg.MaxTextBytes {
		return nil, fmt.Errorf(
			"text length %d exceeds MaxTextBytes %d: %w",
			len(original), cfg.MaxTextBytes, ErrInvalidConfig,
		)
	}
	return &State{
		ctx:      ctx,
		original: []byte(original),
		text:     []byte(original),
		locked:   interval.New(),
		lexicon:  lex,
		config:   cfg,
	}, nil
}

// Original returns the input text (immutable).
func (s *State) Original() string {
	return string(s.original)
}

// Text returns the current text. Initially equal to Original; modified
// by Replace calls.
func (s *State) Text() string {
	return string(s.text)
}

// Lexicon returns the Lexicon associated with this State (may be nil).
func (s *State) Lexicon() lexicon.Lexicon {
	return s.lexicon
}

// Config returns the Config used to construct this State.
func (s *State) Config() Config {
	return s.config
}

// Changes returns a defensive copy of the accumulated Changes.
func (s *State) Changes() []Change {
	out := make([]Change, len(s.changes))
	copy(out, s.changes)
	return out
}

// Steps returns the per-Processor step timings populated by the Engine.
//
// Returns a defensive copy. Returns nil if Engine has not run on this State.
func (s *State) Steps() []StepTiming {
	if len(s.steps) == 0 {
		return nil
	}
	out := make([]StepTiming, len(s.steps))
	copy(out, s.steps)
	return out
}

// IsLocked reports whether any Locked region overlaps span.
//
// Returns false if span is invalid or out of range. Locked regions
// are in Original coordinates, so this check uses Original bounds.
func (s *State) IsLocked(span Span) bool {
	if !span.IsValid() || span.End > len(s.original) {
		return false
	}
	return s.locked.Overlaps(span.Start, span.End)
}

// Lock marks the Span [Start, End) in Original as protected from future
// Replace calls.
//
// Returns:
//
//   - ErrInvalidSpan (wrapped) when span is invalid or out of range.
//   - ErrConflict (wrapped) when span overlaps an already-Locked region.
//
// Per spec (05 §6), Lock does NOT conflict with previously-Replaced
// regions (since those are "consumed" and not subject to re-protection).
func (s *State) Lock(span Span) error {
	if !span.IsValid() {
		return fmt.Errorf("invalid span %v: %w", span, ErrInvalidSpan)
	}
	if span.End > len(s.original) {
		return fmt.Errorf("span end %d > original length %d: %w",
			span.End, len(s.original), ErrInvalidSpan)
	}
	if !s.locked.Add(span.Start, span.End) {
		return fmt.Errorf("span %v overlaps existing locked region: %w",
			span, ErrConflict)
	}
	return nil
}

// Replace applies a text substitution at the given Span (Original
// coordinates).
//
// # Behavior
//
// On success:
//   - Text() returns the modified text.
//   - A new Change with Applied=true is appended.
//
// Returns:
//
//   - ErrInvalidSpan (wrapped) when span is invalid or out of Original range.
//   - ErrConflict (wrapped) when span overlaps a Locked region, or when
//     span (partially or fully) lies inside a previously-Replaced region.
//   - ErrInvalidConfig (wrapped) when meta.Confidence is NaN/Inf or
//     outside [0, 1].
//
// # Idempotency
//
// Two successive Replace calls on the same Original span will both
// succeed; the second one overrides the first. The corresponding Change
// records both operations (audit trail).
func (s *State) Replace(span Span, to string, meta ChangeMeta) error {
	if err := s.validateConfidence(meta.Confidence); err != nil {
		return err
	}
	if err := s.validateSpan(span); err != nil {
		return err
	}
	if s.locked.Overlaps(span.Start, span.End) {
		return fmt.Errorf("span %v overlaps locked region: %w", span, ErrConflict)
	}

	// Map Original to Text positions.
	textStart, ok1 := s.origToText(span.Start)
	textEnd, ok2 := s.origToText(span.End)
	if !ok1 || !ok2 {
		return fmt.Errorf(
			"span %v inside a previously replaced region: %w", span, ErrConflict,
		)
	}

	// Splice Text: text[:textStart] + to + text[textEnd:]
	newText := make([]byte, 0, len(s.text)-(textEnd-textStart)+len(to))
	newText = append(newText, s.text[:textStart]...)
	newText = append(newText, to...)
	newText = append(newText, s.text[textEnd:]...)
	s.text = newText

	// Record replacement (sorted).
	s.replacements = append(s.replacements, replacement{
		origStart: span.Start,
		origEnd:   span.End,
		newLen:    len(to),
	})
	sort.SliceStable(s.replacements, func(i, j int) bool {
		return s.replacements[i].origStart < s.replacements[j].origStart
	})

	// Record Change.
	s.changes = append(s.changes, Change{
		Span:             span,
		From:             string(s.original[span.Start:span.End]),
		To:               to,
		Action:           ActionReplace,
		Kind:             ChangeReplace,
		Applied:          true,
		Source:           meta.Source,
		Confidence:       meta.Confidence,
		RuleID:           meta.RuleID,
		EntryID:          meta.EntryID,
		Reason:           meta.Reason,
		Processor:        "", // filled by Engine when running through Pipeline
		ProcessorVersion: "",
	})
	return nil
}

// Rewrite replaces the entire Text with the given content, recording
// a Change for the full Original span BUT without adding a replacement
// record to the Original→Text mapping.
//
// # When to Use
//
// Rewrite is intended for pre-processing steps (such as the Normalize
// Processor) that perform input-level transformations (whitespace
// cleanup, control character removal, fullwidth → halfwidth). These
// transformations change the byte stream that downstream Processors
// see, but they should NOT consume the Original offsets — downstream
// Processors still match against the original byte positions of
// meaningful content.
//
// # Behavior Difference vs. Replace(Span{0, len(original)}, ...)
//
// If a pre-processor used Replace with Span{0, len(original)}, the
// entire Original would be marked as replaced, and any subsequent
// Span would be rejected as "inside a previously replaced region".
// Rewrite avoids this by recording a Change but not a replacement.
//
// # Change Span Semantics
//
// The Change recorded by Rewrite has Span covering the full Original
// (Span{0, len(original)}), with From = original and To = the new
// content. Downstream Processor Changes that follow will have their
// usual Original-relative Spans; the Result aggregator treats them
// independently.
func (s *State) Rewrite(text string, meta ChangeMeta) error {
	if err := s.validateConfidence(meta.Confidence); err != nil {
		return err
	}
	s.text = []byte(text)
	s.changes = append(s.changes, Change{
		Span:             Span{Start: 0, End: len(s.original)},
		From:             string(s.original),
		To:               text,
		Action:           ActionReplace,
		Kind:             ChangeRewrite,
		Applied:          true,
		Source:           meta.Source,
		Confidence:       meta.Confidence,
		RuleID:           meta.RuleID,
		EntryID:          meta.EntryID,
		Reason:           meta.Reason,
		Processor:        "",
		ProcessorVersion: "",
	})
	return nil
}

// Suggest records a proposed change WITHOUT applying it.
//
// # Behavior
//
//   - Text() is unchanged.
//   - A new Change with Applied=false is appended to Changes.
//
// Returns:
//
//   - ErrInvalidSpan (wrapped) when span is invalid or out of range.
//   - ErrInvalidConfig (wrapped) when meta.Confidence is NaN/Inf or
//     outside [0, 1].
//
// Per spec (05 §5), Suggest does NOT conflict with Locked regions:
// suggestions are advisory and may target any region.
func (s *State) Suggest(span Span, to string, meta ChangeMeta) error {
	if err := s.validateConfidence(meta.Confidence); err != nil {
		return err
	}
	if err := s.validateSpan(span); err != nil {
		return err
	}

	s.changes = append(s.changes, Change{
		Span:             span,
		From:             string(s.original[span.Start:span.End]),
		To:               to,
		Action:           ActionSuggest,
		Kind:             ChangeSuggest,
		Applied:          false,
		Source:           meta.Source,
		Confidence:       meta.Confidence,
		RuleID:           meta.RuleID,
		EntryID:          meta.EntryID,
		Reason:           meta.Reason,
		Processor:        "",
		ProcessorVersion: "",
	})
	return nil
}

// validateSpan checks span bounds against Original. Returns
// ErrInvalidSpan on failure.
func (s *State) validateSpan(span Span) error {
	if !span.IsValid() {
		return fmt.Errorf("invalid span %v: %w", span, ErrInvalidSpan)
	}
	if span.End > len(s.original) {
		return fmt.Errorf("span end %d > original length %d: %w",
			span.End, len(s.original), ErrInvalidSpan)
	}
	return nil
}

// validateConfidence rejects NaN, ±Inf, and values outside [0, 1].
func (s *State) validateConfidence(c float64) error {
	if math.IsNaN(c) {
		return fmt.Errorf("confidence is NaN: %w", ErrInvalidConfig)
	}
	if math.IsInf(c, 0) {
		return fmt.Errorf("confidence is %v: %w", c, ErrInvalidConfig)
	}
	if c < 0 || c > 1 {
		return fmt.Errorf("confidence %v out of [0,1]: %w", c, ErrInvalidConfig)
	}
	return nil
}

// origToText maps an Original position to the corresponding Text
// position, given the accumulated replacements.
//
// # Position Semantics
//
// A position is a "point" between bytes (text-edit model):
//   - Position 0 is before the first byte.
//   - Position N is after the last byte of an N-byte text.
//
// For an empty replacement {X, X, 0}, the point at original
// position X is the same as the text point at the replacement start.
//
// Returns (textPos, true) for positions outside any replacement.
// Returns (0, false) for positions strictly inside a replacement (the
// original byte at that position was removed and has no clear
// mapping).
//
// # Boundary Case
//
// A position equal to a replacement's origStart (e.g., 16 for {16, 19})
// is treated as the "post-removal" boundary (i.e., the first valid text
// position past the removed range). This matters for Span.End, which
// is the exclusive upper bound of a Span and may land on a removal
// boundary.
func (s *State) origToText(origPos int) (int, bool) {
	textPos := origPos
	for _, r := range s.replacements {
		if origPos <= r.origStart {
			break
		}
		if origPos < r.origEnd {
			return 0, false
		}
		origLen := r.origEnd - r.origStart
		textPos += r.newLen - origLen
	}
	return textPos, true
}

// Context returns the State-bound context (for cancellation checks).
func (s *State) Context() context.Context {
	return s.ctx
}
