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

// Package lexicon provides the knowledge container abstractions used by
// ark-lexnorm for normalization rules, variants, and relations.
//
// # M3 Scope
//
// This file declares the Lexicon interface and supporting value types.
// The actual implementations (memory Lexicon, Builder, LexiconSource,
// Compose, Store) are delivered in M6 and M7.
//
// # Lexicon Is Immutable at Runtime
//
// Once a Lexicon is built (via Builder), it must be treated as immutable
// for the duration of any Normalize call. Atomic swap of the entire
// Lexicon is the only allowed update path; see Store in M7.
//
// # Concurrency
//
// A built Lexicon is safe for concurrent read access by multiple
// goroutines (including Engine.Normalize calls in parallel).
package lexicon

// EntryID is a stable identifier for an Entry in a Lexicon.
//
// EntryID values are application-defined and must be unique within a
// single Lexicon. The engine does not interpret the value.
type EntryID string

// String returns the underlying string. Implements fmt.Stringer.
func (id EntryID) String() string { return string(id) }

// IsZero reports whether id is the empty string.
func (id EntryID) IsZero() bool { return id == "" }

// VariantKind categorizes the semantic relationship between a Variant
// and its parent Entry's Canonical text.
type VariantKind uint8

const (
	// VariantAlias indicates a synonym: the Variant and Canonical are
	// interchangeable in normal text.
	VariantAlias VariantKind = iota

	// VariantCorrection indicates a deterministic error: the Variant is
	// a common mistake that should be corrected to Canonical.
	VariantCorrection

	// VariantHomophone indicates a phonetic match (same sound,
	// different spelling).
	VariantHomophone

	// VariantApproximate indicates a fuzzy match (edit distance, n-gram
	// overlap, etc.).
	VariantApproximate
)

// String returns a stable lowercase identifier for the VariantKind.
func (k VariantKind) String() string {
	switch k {
	case VariantAlias:
		return "alias"
	case VariantCorrection:
		return "correction"
	case VariantHomophone:
		return "homophone"
	case VariantApproximate:
		return "approximate"
	}
	return "unknown"
}

// Variant represents one alternative form of an Entry.
type Variant struct {
	// Text is the variant form (must be non-empty).
	Text string

	// Kind categorizes the relationship to Canonical.
	Kind VariantKind

	// Confidence is the match confidence [0.0, 1.0].
	Confidence float64

	// Source identifies the origin (e.g., "human-curated", "auto-mined",
	// "third-party"). Free-form string.
	Source string
}

// IsValid reports whether v has a non-empty Text.
func (v Variant) IsValid() bool {
	return v.Text != ""
}

// Entry represents one canonical entry in a Lexicon.
type Entry struct {
	// ID is a stable identifier (unique within a Lexicon).
	ID EntryID

	// Text is the canonical (target) form.
	Text string

	// Variants are all known alternative forms pointing to this Entry.
	Variants []Variant

	// Meta holds arbitrary metadata (the engine does not interpret it).
	Meta map[string]any
}

// IsValid reports whether e has a non-empty ID.
func (e Entry) IsValid() bool {
	return !e.ID.IsZero()
}

// Relation connects two Entries (e.g., "see also", "antonym").
type Relation struct {
	// From is the source Entry.
	From EntryID

	// To is the target Entry.
	To EntryID

	// Kind categorizes the relationship.
	Kind VariantKind

	// Weight is the relation strength (interpretation depends on Kind).
	Weight float64
}

// Matcher is a pre-built multi-pattern matching engine.
//
// Matcher is produced by Builder from all Entry variants in the
// Lexicon and exposed via Lexicon.Matcher(). It is backed by an
// Aho-Corasick automaton (see ahocorasick.go).
//
// Matcher is safe for concurrent read access; multiple goroutines may
// call Match concurrently.
type Matcher struct {
	ac *AhoCorasick
}

// NewMatcher constructs a Matcher from the given pattern strings.
// This is the public constructor used by Builder.
func NewMatcher(patterns []string) *Matcher {
	return &Matcher{ac: NewAhoCorasick(patterns)}
}

// Match returns all pattern occurrences in text, sorted by Start position.
//
// Each returned Match carries:
//   - Start/End: byte offsets in text (UTF-8)
//   - Pattern:   the matched pattern string
//   - PatternIdx: index in the input slice passed to NewMatcher
//
// Returns nil when no patterns match.
func (m *Matcher) Match(text string) []Match {
	if m == nil || m.ac == nil {
		return nil
	}
	return m.ac.Match(text)
}

// PatternCount returns the number of patterns in the matcher.
func (m *Matcher) PatternCount() int {
	if m == nil || m.ac == nil {
		return 0
	}
	return m.ac.PatternCount()
}

// Lexicon is the immutable, queryable knowledge container used by
// Processors for normalization.
//
// # Thread Safety
//
// A Lexicon is safe for concurrent read access once built. Mutations
// are not permitted; use Builder to construct a new Lexicon and the
// Store (M7) to atomically swap.
//
// # Determinism
//
// All() must return entries in a deterministic order (typically sorted
// by Entry.Text or EntryID). Lookup / Entry must be O(1).
type Lexicon interface {
	// Entry returns the Entry with the given ID. The second return is
	// false if no such Entry exists.
	Entry(id EntryID) (Entry, bool)

	// Lookup returns the Entry whose canonical Text exactly matches
	// the given text. The second return is false if no match.
	Lookup(text string) (Entry, bool)

	// Relations returns all Relations whose From field matches the given
	// text. Empty slice if none.
	Relations(text string) []Relation

	// All iterates over every Entry in deterministic order.
	//
	// yield is called for each Entry. If yield returns false, iteration
	// stops early.
	All(yield func(Entry) bool)

	// Matcher returns the pre-built multi-pattern matching engine.
	Matcher() *Matcher

	// Len returns the number of Entries.
	Len() int

	// Version returns the Lexicon's version string (for audit).
	Version() string
}
