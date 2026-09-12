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

// Package ctxproc implements the Context Processor.
//
// # M9 Status: Skeleton (No-Op)
//
// M9 provides the Context Processor interface and identity, but not
// the actual disambiguation logic. The reason: real context-aware
// correction requires application-specific resources that the core
// package cannot bundle:
//
//   - **LLM-based disambiguation** (D1: LLM is an optional extension).
//   - **Domain-specific rules** (medical, legal, financial, ...).
//   - **Statistical / ML models** (BERT-style classifiers, ...).
//   - **User-feedback loops** (active learning from corrections).
//
// The core package does not bundle any of these. Application code
// should provide a custom Context Processor that wraps the desired
// resource.
//
// # Package Name
//
// The package is named `ctxproc` (not `context`) to avoid collision
// with the Go standard library `context` package. The directory is
// `processor/ctxproc/`.
//
// # Interface
//
// The default Context Processor is a no-op: it preserves the State
// unchanged. Application code can either:
//
//  1. Wrap this default and add custom logic in a Middleware or Hook
//     to inspect / modify the State.
//  2. Provide a fully custom Processor (via the Processor interface)
//     that uses the desired LLM / ML / rule-based system.
//
// # Order in Standard Pipeline
//
// Context runs LAST in the Standard Pipeline, after Pinyin and Fuzzy.
// Its purpose is to disambiguate among multiple Suggestions emitted
// by earlier Processors (e.g., choosing between 同音 candidates based
// on surrounding context).
//
// # Future Enhancements
//
// When the Lexicon provides a context-aware scoring function
// (e.g., per-domain weights), M12 may upgrade this Processor to use
// that scoring. For now, it remains a placeholder.
package ctxproc

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "context"

	// Version is the semantic version of this Processor.
	Version = "v1"
)

// Candidate is one contextual resolution candidate: a surface form
// (Variant{Contextual}.Text) that may resolve to a canonical entry.
type Candidate struct {
	// EntryID of the canonical form.
	EntryID lexicon.EntryID

	// Canonical text this candidate would replace the surface form with.
	Canonical string

	// Confidence declared on the Variant{Contextual}.
	Confidence float64
}

// Scorer re-ranks contextual candidates for one surface occurrence.
//
// Application code implements scoring using surrounding context (the
// full original text and the match position). Returning an empty slice
// or fewer candidates than received prunes the list. A nil Scorer
// keeps declaration order (lexicon ID order).
type Scorer func(ctx context.Context, original string, start, end int, cands []Candidate) []Candidate

// Processor is the Contextual Correction Processor.
//
// It scans the text for Variant{Contextual} forms. These are surface
// forms whose canonical target depends on context (e.g., the same
// nickname mapped to different people). Resolution policy:
//
//   - exactly one candidate → Suggest (never applied in v1)
//   - multiple candidates   → run the Scorer; if a unique best remains
//     → Suggest; otherwise → Skip
//
// v1 deliberately never calls State.Replace: contextual resolution is
// heuristic, and the spec requires uncertainty to degrade to Suggest
// or Skip.
type Processor struct {
	lex      lexicon.Lexicon
	matcher  *lexicon.Matcher
	candsFor map[string][]Candidate // surface form → candidates
	scorer   Scorer
}

// New returns a no-op Context Processor (kept for backward
// compatibility). Use NewWithLexicon for the functional implementation.
func New() *Processor { return &Processor{} }

// NewWithLexicon constructs a functional Contextual Processor from the
// given Lexicon. If lex is nil, the Processor is a no-op.
func NewWithLexicon(lex lexicon.Lexicon) *Processor {
	p := &Processor{lex: lex}
	if lex == nil {
		return p
	}
	candsFor := make(map[string][]Candidate)
	var patterns []string
	seen := make(map[string]bool)

	lex.All(func(e lexicon.Entry) bool {
		for _, v := range e.Variants {
			if v.Kind != lexicon.VariantContextual {
				continue
			}
			if !v.IsValid() || v.Text == e.Text {
				continue
			}
			if strings.Contains(e.Text, v.Text) {
				continue // would corrupt; also rejected by Builder.Validate
			}
			if !seen[v.Text] {
				seen[v.Text] = true
				patterns = append(patterns, v.Text)
			}
			conf := v.Confidence
			if conf == 0 {
				conf = 0.7 // default: above Suggest, below AutoApply
			}
			candsFor[v.Text] = append(candsFor[v.Text], Candidate{
				EntryID:    e.ID,
				Canonical:  e.Text,
				Confidence: conf,
			})
		}
		return true
	})

	if len(patterns) == 0 {
		return p
	}
	// Deterministic candidate order (lexicon iteration is already
	// ID-sorted; keep it explicit for stability under scorer pruning).
	for k := range candsFor {
		c := candsFor[k]
		sort.Slice(c, func(i, j int) bool { return c[i].EntryID < c[j].EntryID })
		candsFor[k] = c
	}
	p.matcher = lexicon.NewMatcher(patterns)
	p.candsFor = candsFor
	return p
}

// WithScorer attaches a custom candidate scorer. Returns the Processor
// for chaining.
func (p *Processor) WithScorer(sc Scorer) *Processor {
	p.scorer = sc
	return p
}

// Name implements lexnorm.Processor.
func (p *Processor) Name() string { return Name }

// Version implements lexnorm.Versioner.
func (p *Processor) Version() string { return Version }

// Certainty implements lexnorm.CertaintyReporter.
//
// Context is low-certainty by design: it operates on context-dependent
// candidates.
func (p *Processor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyLow }

// Process implements lexnorm.Processor.
//
// Scans Original for Variant{Contextual} forms and records Suggestions
// for uniquely resolvable ones. Never applies changes in v1.
func (p *Processor) Process(ctx context.Context, s *lexnorm.State) error {
	if p.matcher == nil {
		return nil
	}
	for _, m := range p.matcher.Match(s.Original()) {
		cands, ok := p.candsFor[m.Pattern]
		if !ok || len(cands) == 0 {
			continue
		}
		if len(cands) > 1 && p.scorer != nil {
			cands = p.scorer(ctx, s.Original(), m.Start, m.End, cands)
		}
		if len(cands) != 1 {
			// Ambiguous even after scoring → Skip (spec: 无法确定时
			// Suggest 或 Skip; multi-candidate ambiguity is Skip).
			continue
		}
		best := cands[0]
		_ = s.Suggest(
			lexnorm.Span{Start: m.Start, End: m.End},
			best.Canonical,
			lexnorm.ChangeMeta{
				Source:     Name,
				Confidence: best.Confidence,
				RuleID:     "contextual",
				EntryID:    string(best.EntryID),
				Reason:     "contextual resolution: " + m.Pattern + " → " + best.Canonical,
			},
		)
	}
	return nil
}

// Descriptor is the Registry Descriptor for this Processor.
//
// NewWithLexicon is not reachable via the Registry's config-based New
// (the Lexicon is injected by the Engine, not by JSON config); the
// Descriptor therefore constructs the no-op variant, mirroring the
// other Lexicon-bound processors.
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyLow,
	New: func(_ json.RawMessage) (lexnorm.Processor, error) {
		return New(), nil
	},
	Default: func() any { return nil },

	Version:           Version,
	Category:          lexnorm.CategoryContextual,
	MutatesText:       false,
	SupportsSuggest:   true,
	SupportsProtected: false,
	Deterministic:     true,
	Determinism:       lexnorm.DeterministicTrue,
	DefaultOrder:      7,
	Description:       "Contextual disambiguation over Variant{Contextual} forms; v1 suggests, never applies.",
}

// Descriptor implements lexnorm.DescribedProcessor: it exposes the
// capability metadata of this Processor (category, mutation mode,
// determinism) for Registry queries, audit tooling, and docs.
func (p *Processor) Descriptor() lexnorm.Descriptor { return Descriptor }
