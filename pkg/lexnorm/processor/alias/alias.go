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

// Package alias implements the Alias Processor.
//
// # Purpose
//
// Replaces alias variants with their canonical forms using a pre-built
// Aho-Corasick automaton over Variant{Alias} entries in the Lexicon.
//
// # Example
//
// Lexicon contains:
//
//	Entry{ID: "e1", Text: "周丽群", Variants: [{Text: "周丽群", Kind: Alias}]}
//
// For input "周莉群" (which has Variant{Text: "周莉群", Kind: Alias}),
// the processor replaces "周莉群" with "周丽群".
//
// # Certainty
//
// Alias is high-certainty: aliases are unambiguous synonyms.
//
// # Order in Standard Pipeline
//
// Alias runs AFTER Normalize (whitespace consistent) and Disfluency
// (no filler noise), and BEFORE Deterministic (corrections) and Pinyin /
// Fuzzy (phonetic matching).
package alias

import (
	"context"
	"encoding/json"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "alias"

	// Version is the semantic version of this Processor.
	Version = "v1"
)

// Processor replaces alias variants with their canonical forms.
type Processor struct {
	lex          lexicon.Lexicon
	matcher      *lexicon.Matcher
	canonicalFor map[string]string          // variant text → canonical text
	entryFor     map[string]lexicon.EntryID // variant text → Entry ID (audit)
}

// New constructs an Alias Processor from the given Lexicon.
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

	lex.All(func(e lexicon.Entry) bool {
		for _, v := range e.Variants {
			if v.Kind != lexicon.VariantAlias {
				continue
			}
			if !v.IsValid() {
				continue
			}
			// Skip if variant equals canonical (no actual change).
			if v.Text == e.Text {
				continue
			}
			patterns = append(patterns, v.Text)
			canonicalFor[v.Text] = e.Text
			entryFor[v.Text] = e.ID
		}
		return true
	})

	if len(patterns) == 0 {
		return p
	}

	p.matcher = lexicon.NewMatcher(patterns)
	p.canonicalFor = canonicalFor
	p.entryFor = entryFor
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
// Walks the Original text for alias matches; replaces each with the
// canonical form. Overlapping replaces are silently skipped.
func (p *Processor) Process(_ context.Context, s *lexnorm.State) error {
	if p.matcher == nil {
		return nil
	}

	matches := p.matcher.Match(s.Original())
	for _, m := range matches {
		canonical, ok := p.canonicalFor[m.Pattern]
		if !ok {
			// Pattern matched but not in canonicalFor (shouldn't happen).
			continue
		}

		err := s.Replace(
			lexnorm.Span{Start: m.Start, End: m.End},
			canonical,
			lexnorm.ChangeMeta{
				Source:     Name,
				Confidence: 1.0,
				RuleID:     "alias",
				EntryID:    string(p.entryFor[m.Pattern]),
				Reason:     "alias → canonical: " + m.Pattern + " → " + canonical,
			},
		)
		// Silently ignore conflicts (overlapping replaces).
		_ = err
	}
	return nil
}

// Descriptor is the Registry Descriptor for this Processor.
//
// The Lexicon must be supplied via the caller's Lexicon binding (e.g.,
// via the Engine's WithLexicon or WithLexiconStore option). The
// Descriptor's New function returns a placeholder; the actual
// construction happens when the Engine injects the Lexicon.
//
// For configuration-driven usage, prefer constructing the Lexicon
// first and then New(lex).
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyHigh,
	New: func(_ json.RawMessage) (lexnorm.Processor, error) {
		// Without a Lexicon, the Processor is a no-op.
		return New(nil), nil
	},
	Default: func() any { return nil },
}
