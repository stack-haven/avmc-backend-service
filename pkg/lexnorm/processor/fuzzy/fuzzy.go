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

// Package fuzzy implements the Fuzzy Processor.
//
// # Purpose
//
// Replaces fuzzy / approximate matches (variants that differ from
// canonical by edit distance) with their canonical forms, using
// Aho-Corasick over Variant{Approximate} entries in the Lexicon.
//
// # Algorithm
//
//  1. At construction, build an Aho-Corasick automaton over all
//     Variant{Approximate} texts in the Lexicon.
//  2. At Process time, scan the Original text for matches.
//  3. For each match, look up confidence and Apply / Suggest / Skip
//     per Config thresholds.
//
// # Decision Thresholds
//
//   - confidence ≥ AutoApplyThreshold → Apply
//   - SuggestThreshold ≤ confidence < AutoApplyThreshold → Suggest
//   - confidence < SuggestThreshold → Skip
//
// # Variant.Confidence Default (Important)
//
// Variant.Confidence is a float64. Its zero value is 0.0, NOT 1.0.
// A Variant{Approximate} with Confidence unset (left at Go's zero
// value) will never reach Apply or Suggest — it falls into the Skip
// branch because 0.0 < SuggestThreshold (default 0.65).
//
// This is silent: Fuzzy will simply produce zero Changes for that
// Variant. To get Apply/Suggest behavior, callers MUST set
// Variant.Confidence explicitly per Variant:
//
//	Variants: []lexicon.Variant{
//	    {Text: "周莉群", Kind: lexicon.VariantApproximate, Confidence: 0.95},
//	    {Text: "周里群", Kind: lexicon.VariantApproximate, Confidence: 0.85},
//	}
//
// If you want a Variant to be treated as "high confidence by default",
// set Confidence explicitly (e.g., 0.95). Fuzzy does NOT auto-promote
// 0.0 to any default; the engine philosophy is "explicit beats implicit".
//
// # Difference From Alias
//
// Alias uses Variant{Alias} (synonyms, high certainty). Fuzzy uses
// Variant{Approximate} (typos with edit distance, lower certainty).
//
// # Order in Standard Pipeline
//
// Fuzzy runs AFTER Pinyin (catches exact pinyin matches first) and
// BEFORE Context (which uses surrounding context to disambiguate).
package fuzzy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "fuzzy"

	// Version is the semantic version of this Processor.
	Version = "v1"
)

// Processor replaces fuzzy / approximate variants with canonical forms.
type Processor struct {
	lex           lexicon.Lexicon
	matcher       *lexicon.Matcher
	canonicalFor  map[string]string          // variant text → canonical text
	entryFor      map[string]lexicon.EntryID // variant text → Entry ID (audit)
	confidenceFor map[string]float64         // variant text → confidence
}

// New constructs a Fuzzy Processor from the given Lexicon.
//
// If lex is nil, the returned Processor is a no-op.
func New(lex lexicon.Lexicon) *Processor {
	p := &Processor{lex: lex}
	if lex == nil {
		return p
	}

	var patterns []string
	canonicalFor := make(map[string]string)
	entryFor := make(map[string]lexicon.EntryID)
	confidenceFor := make(map[string]float64)

	lex.All(func(e lexicon.Entry) bool {
		for _, v := range e.Variants {
			if v.Kind != lexicon.VariantApproximate {
				continue
			}
			if !v.IsValid() {
				continue
			}
			if v.Text == e.Text {
				continue
			}
			patterns = append(patterns, v.Text)
			canonicalFor[v.Text] = e.Text
			entryFor[v.Text] = e.ID
			confidenceFor[v.Text] = v.Confidence
		}
		return true
	})

	if len(patterns) == 0 {
		return p
	}

	p.matcher = lexicon.NewMatcher(patterns)
	p.canonicalFor = canonicalFor
	p.entryFor = entryFor
	p.confidenceFor = confidenceFor
	return p
}

// Name implements lexnorm.Processor.
func (p *Processor) Name() string { return Name }

// Version implements lexnorm.Versioner.
func (p *Processor) Version() string { return Version }

// Certainty implements lexnorm.CertaintyReporter.
//
// Fuzzy is medium-certainty: approximate matches are heuristic.
func (p *Processor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyMedium }

// Process implements lexnorm.Processor.
//
// Walks Original via the Aho-Corasick matcher; for each match, decides
// Apply / Suggest / Skip per confidence vs. Config thresholds.
func (p *Processor) Process(_ context.Context, s *lexnorm.State) error {
	if p.matcher == nil {
		return nil
	}

	matches := p.matcher.Match(s.Original())
	autoApply := s.Config().AutoApplyThreshold
	suggest := s.Config().SuggestThreshold

	for _, m := range matches {
		confidence := p.confidenceFor[m.Pattern]
		canonical := p.canonicalFor[m.Pattern]
		if canonical == "" {
			continue
		}

		span := lexnorm.Span{Start: m.Start, End: m.End}
		meta := lexnorm.ChangeMeta{
			Source:     Name,
			Confidence: confidence,
			RuleID:     "approximate",
			EntryID:    string(p.entryFor[m.Pattern]),
			Reason:     fmt.Sprintf("fuzzy: %s → %s (conf %.2f)", m.Pattern, canonical, confidence),
		}

		if confidence >= autoApply {
			_ = s.Replace(span, canonical, meta)
		} else if confidence >= suggest {
			_ = s.Suggest(span, canonical, meta)
		}
		// else: Skip
	}
	return nil
}

// Descriptor is the Registry Descriptor for this Processor.
//
// See alias.Descriptor for notes on Lexicon binding.
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyMedium,
	New: func(_ json.RawMessage) (lexnorm.Processor, error) {
		return New(nil), nil
	},
	Default: func() any { return nil },
}
