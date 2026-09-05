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

// Package deterministic implements the Deterministic Processor.
//
// # Purpose
//
// Replaces deterministic ASR / OCR errors with their canonical forms
// using a pre-built Aho-Corasick automaton over Variant{Correction}
// entries in the Lexicon.
//
// # Example
//
// Lexicon contains:
//
//	Entry{ID: "e1", Text: "周丽群",
//	      Variants: [{Text: "周丽裙", Kind: Correction},     // common typo
//	                 {Text: "周丽郡", Kind: Correction}]}    // common typo
//
// For input "周丽裙明天来", the processor replaces "周丽裙" with "周丽群".
//
// # Difference From Alias
//
// Both Alias and Deterministic replace variants with canonical forms.
// The semantic distinction:
//
//   - Alias: synonyms / interchangeable forms (high certainty).
//   - Deterministic: known typos / OCR errors (also high certainty,
//     but typically smaller scope, only used for clearly-known mistakes).
//
// # Order in Standard Pipeline
//
// Deterministic runs AFTER Alias (catches anything Alias missed) and
// BEFORE Pinyin / Fuzzy / Context (which deal with less-certain
// phonetic matches).
//
// # Certainty
//
// Deterministic is high-certainty: corrections are explicit typos
// registered in the Lexicon.
package deterministic

import (
	"context"
	"encoding/json"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "deterministic"

	// Version is the semantic version of this Processor.
	Version = "v1"
)

// Processor replaces deterministic correction variants with canonical forms.
type Processor struct {
	lex           lexicon.Lexicon
	matcher       *lexicon.Matcher
	canonicalFor  map[string]string
	entryFor      map[string]lexicon.EntryID
	confidenceFor map[string]float64
}

// New constructs a Deterministic Processor from the given Lexicon.
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
			if v.Kind != lexicon.VariantCorrection {
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
func (p *Processor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyHigh }

// Process implements lexnorm.Processor.
//
// Walks the Original text for correction matches; replaces each with
// the canonical form. Overlapping replaces are silently skipped.
func (p *Processor) Process(_ context.Context, s *lexnorm.State) error {
	if p.matcher == nil {
		return nil
	}

	matches := p.matcher.Match(s.Original())
	for _, m := range matches {
		canonical, ok := p.canonicalFor[m.Pattern]
		if !ok {
			continue
		}

		confidence := p.confidenceFor[m.Pattern]
		if confidence == 0 {
			confidence = 1.0
		}

		err := s.Replace(
			lexnorm.Span{Start: m.Start, End: m.End},
			canonical,
			lexnorm.ChangeMeta{
				Source:     Name,
				Confidence: confidence,
				RuleID:     "deterministic",
				EntryID:    string(p.entryFor[m.Pattern]),
				Reason:     "deterministic correction: " + m.Pattern + " → " + canonical,
			},
		)
		_ = err
	}
	return nil
}

// Descriptor is the Registry Descriptor for this Processor.
//
// See alias.Descriptor for notes on Lexicon binding.
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyHigh,
	New: func(_ json.RawMessage) (lexnorm.Processor, error) {
		return New(nil), nil
	},
	Default: func() any { return nil },
}
