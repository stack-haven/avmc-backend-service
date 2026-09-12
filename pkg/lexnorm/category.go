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

// Category is the formal capability classification of a Processor.
//
// Categories describe WHAT KIND of normalization a Processor performs,
// independent of any concrete algorithm or language. They are metadata
// only: they never influence execution order (user-declared Pipeline
// order is authoritative) and never gate behavior.
//
// # The Eight Categories
//
//	Normalization        — unify base text representation
//	Noise                — remove text with no semantic value
//	Canonicalization     — unify valid alternative expressions
//	Deterministic        — correct stable, explicit error mappings
//	Phonetic             — correct pronunciation-based errors
//	Approximate          — correct fuzzy / edit-distance candidates
//	Contextual           — disambiguate via context or relations
//	Semantic             — model-driven / generative refinement
//
// # Business Neutrality
//
// Category names deliberately avoid business concepts (ASR / OCR /
// Meeting / HR). A "transcript correction" need is served by combining
// Noise + Canonicalization + Phonetic, not by a business-named category.
type Category string

const (
	// CategoryNormalization covers base text-representation unification:
	// Unicode forms, fullwidth/halfwidth, whitespace, punctuation,
	// control characters, newline unification. It never expresses
	// semantic correction.
	CategoryNormalization Category = "normalization"

	// CategoryNoise covers text lacking semantic value: filler words,
	// modal particles, stutter repeats, repeated phrases, and other
	// speech noise. Processors in this category must NOT delete tokens
	// that may carry meaning in context by unconditional substitution.
	CategoryNoise Category = "noise"

	// CategoryCanonicalization maps multiple legitimate expressions of
	// the same concept onto one canonical form: aliases, nicknames,
	// abbreviations, acronyms, short names, forms of address, variants.
	// The original expression is not "wrong" — it is non-canonical.
	CategoryCanonical Category = "canonicalization"

	// CategoryDeterministic corrects errors with explicit, stable
	// mappings: fixed correction rules, confirmed lexical error maps.
	// Every change must trace to a rule or lexicon entry source.
	CategoryDeterministic Category = "deterministic"

	// CategoryPhonetic corrects errors grounded in pronunciation:
	// homophones, near-homophones, syllable matching, phonetic candidate
	// ranking. The category name is algorithm-neutral; Chinese Pinyin is
	// one implementation, not the definition.
	CategoryPhonetic Category = "phonetic"

	// CategoryApproximate recalls and ranks fuzzy candidates: edit
	// distance, n-gram overlap, character/token similarity. Processors
	// in this category should prefer Suggest over Apply unless the
	// candidate confidence is high.
	CategoryApproximate Category = "approximate"

	// CategoryContextual resolves normalization that requires context,
	// relations, or discourse: candidate ranking, name/organization
	// disambiguation, multi-candidate decisions.
	CategoryContextual Category = "contextual"

	// CategorySemantic performs high-level semantic refinement via
	// models (LLM or local). Characterized by lower certainty, higher
	// mutation risk, higher cost, and possible non-determinism; such
	// Processors belong at the tail of a Pipeline by default.
	CategorySemantic Category = "semantic"
)

// DefaultOrder returns the canonical position of the category in the
// default Pipeline (spec §7):
//
//	Normalization → Noise → Canonicalization → Deterministic
//	→ Phonetic → Approximate → Contextual → Semantic
//
// Unknown categories sort last. This value is for default Pipeline
// construction and documentation display only; it never reorders a
// user-declared Pipeline.
func (c Category) DefaultOrder() int {
	switch c {
	case CategoryNormalization:
		return 1
	case CategoryNoise:
		return 2
	case CategoryCanonical:
		return 3
	case CategoryDeterministic:
		return 4
	case CategoryPhonetic:
		return 5
	case CategoryApproximate:
		return 6
	case CategoryContextual:
		return 7
	case CategorySemantic:
		return 8
	}
	return 99
}
