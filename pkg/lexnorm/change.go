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

// ChangeKind categorizes the semantic class of a Change.
//
// # 1.2 Renaming
//
// Earlier drafts (1.0) used `Change.Kind` of type `Kind`. 1.2 renames the
// type to `ChangeKind` to avoid name collision with `Action.Kind` (a
// future-proofing hook) and to make dot-qualified names self-describing.
//
// # Backward Compatibility
//
// Future ChangeKind values may be appended but existing constants must
// keep their numeric values.
type ChangeKind uint8

const (
	// ChangeReplace is a substitution: From → To.
	ChangeReplace ChangeKind = iota

	// ChangeRemove is a deletion: From → "".
	ChangeRemove

	// ChangeSuggest is a proposal: From → To, NOT applied.
	ChangeSuggest

	// ChangeRewrite is a full-text rewrite used by pre-processors
	// (e.g., Normalize). It records that the input was transformed
	// at the byte level (whitespace cleanup, control char removal)
	// without consuming Original offsets.
	ChangeRewrite
)

// String returns a stable lowercase identifier for the ChangeKind.
func (k ChangeKind) String() string {
	switch k {
	case ChangeReplace:
		return "replace"
	case ChangeRemove:
		return "remove"
	case ChangeSuggest:
		return "suggest"
	case ChangeRewrite:
		return "rewrite"
	}
	return fmt.Sprintf("ChangeKind(%d)", uint8(k))
}

// Change records a single text modification proposed or applied by a
// Processor.
//
// # Construction
//
// Most callers do NOT construct Change directly. Instead they call
// State.Replace / State.Suggest, which validate the Span, attach the
// running Processor name, and append to the State's change log. State
// then surfaces the accumulated Changes via Result.Changes.
//
// Direct construction (literal struct) is allowed for testing and for
// Processors that build Change values in batches before committing.
//
// # Value Semantics
//
// Change is a value type. Modifying a Change after it has been recorded
// in a Result has no effect on the Result.
type Change struct {
	// Span locates the modified range in the **Original** text
	// (byte offsets, UTF-8).
	Span Span

	// From is the original text covered by Span.
	From string

	// To is the replacement text. For ChangeRemove, To is "".
	To string

	// Action records WHAT was done (replace / remove / suggest).
	// Action is set by the Processor or by State on behalf of the
	// Processor.
	Action Action

	// Kind categorizes the semantic class (replace / remove / suggest).
	// For most Changes, Action and Kind are equal; they are kept
	// separate to allow future divergence (e.g., a Processor may record
	// a ChangeReplace with ActionSuggest).
	Kind ChangeKind

	// Source identifies the origin of this change (e.g., "human-curated",
	// "auto-mined", "third-party"). Free-form string; the engine does
	// not interpret the value (1.2 decision: 1.0 enum → 1.1+ string).
	Source string

	// RuleID identifies the rule that produced this change (for audit).
	// Optional: "" when no specific rule applies.
	RuleID string

	// EntryID identifies the Lexicon Entry that produced this change.
	// Optional: "" for rule-based (non-lexicon) changes.
	EntryID string

	// Processor is the Name() of the producing Processor.
	Processor string

	// ProcessorVersion is the Version() of the producing Processor, if
	// it implements the optional Versioner interface (empty otherwise).
	ProcessorVersion string

	// Confidence is the per-call confidence score [0.0, 1.0].
	// Construction with values outside this range is allowed; validation
	// happens at Config.Validate() time. NaN and ±Inf are illegal and
	// return ErrInvalidConfig.
	Confidence float64

	// Applied is true if the change was applied to State.Text, false if
	// it was only a suggestion.
	Applied bool

	// Reason is a human-readable explanation (for audit, debugging, UI).
	Reason string
}

// IsZero reports whether c is the zero value.
//
// A zero Change has Span=Span{0,0}, From="", To="", Applied=false. Useful
// for tests and "did the Processor emit any change?" checks.
func (c Change) IsZero() bool {
	return c.Span.IsZero() &&
		c.From == "" &&
		c.To == "" &&
		!c.Applied
}

// HasRule returns true if the change is associated with a specific rule.
func (c Change) HasRule() bool {
	return c.RuleID != ""
}

// HasLexiconEntry returns true if the change is associated with a
// Lexicon Entry.
func (c Change) HasLexiconEntry() bool {
	return c.EntryID != ""
}

// ChangeMeta is the metadata passed to State.Replace / State.Suggest.
//
// ChangeMeta is a subset of Change containing only the per-call metadata
// that a Processor knows at the moment it requests a modification. The
// Span, From, To, and Applied are determined by the State call itself.
type ChangeMeta struct {
	// Source identifies the origin (e.g., "human-curated", "auto-mined",
	// "third-party").
	Source string

	// Confidence is the per-call confidence score [0.0, 1.0].
	Confidence float64

	// RuleID identifies the rule that produced this change.
	RuleID string

	// EntryID identifies the Lexicon Entry that produced this change.
	EntryID string

	// Reason is a human-readable explanation.
	Reason string
}

// IsZero reports whether m is the zero value (no metadata).
func (m ChangeMeta) IsZero() bool {
	return m.Source == "" &&
		m.Confidence == 0 &&
		m.RuleID == "" &&
		m.EntryID == "" &&
		m.Reason == ""
}
