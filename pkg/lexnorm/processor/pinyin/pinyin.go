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

// Package pinyin implements the Phonetic Processor (Chinese Pinyin).
//
// # Purpose
//
// Replaces phonetic variants (homophones) with their canonical forms.
// The algorithm is intentionally split from the category: Category-
// Phonetic is the architecture concept, Chinese Pinyin via a pluggable
// lexicon.PinyinConverter is one implementation. Other phonetic
// similarity algorithms can be added as sibling implementations.
//
// # Matching (two complementary paths)
//
//  1. Whole-word path: an Aho-Corasick automaton over every
//     Variant{Homophone}.Text. A whole-word match replaces the full
//     span with the canonical form. This is the precise path and the
//     primary consumer of homophone variants (previously unconsumed).
//  2. Per-character path: scans CJK characters, computes their pinyin,
//     and looks the form up in the pinyin index of SINGLE-CHARACTER
//     entries only. Multi-character entries are unreachable here BY
//     DESIGN: replacing a one-character span with a multi-character
//     canonical would corrupt the text. This path exists to fix
//     single-character homophones (e.g., 曾/赠 confusions) where the
//     entry's canonical form is one character.
//
// # Decision Thresholds
//
//   - confidence ≥ AutoApplyThreshold → Apply (State.Replace)
//   - SuggestThreshold ≤ confidence < AutoApplyThreshold → Suggest
//     (State.Suggest)
//   - confidence < SuggestThreshold → Skip
//
// # Order in Standard Pipeline
//
// Phonetic runs AFTER Alias / Deterministic (exact matches) and BEFORE
// Approximate/Fuzzy (which handles non-phonetic typos).
package pinyin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "pinyin"

	// Version is the semantic version of this Processor.
	Version = "v1"
)

// wordMatch maps one homophone variant text to its replacement.
type wordMatch struct {
	canonical  string
	entryID    lexicon.EntryID
	confidence float64
}

// Processor matches phonetic variants to canonical entries.
type Processor struct {
	lex           lexicon.Lexicon
	converter     lexicon.PinyinConverter
	wordMatcher   *lexicon.Matcher         // Aho-Corasick over Variant{Homophone}.Text
	wordFor       map[string]wordMatch     // variant text → replacement info
	pinyinToEntry map[string]lexicon.Entry // pinyin form → single-char Entry
	confidenceFor map[string]float64       // pinyin form → confidence
}

// New constructs a Pinyin (Phonetic) Processor.
//
// Both lex and converter must be non-nil for the per-character path to
// be functional; the whole-word path only requires lex. With lex nil,
// the Processor is a no-op.
func New(lex lexicon.Lexicon, converter lexicon.PinyinConverter) *Processor {
	p := &Processor{lex: lex, converter: converter}
	if lex == nil {
		return p
	}

	wordFor := make(map[string]wordMatch)
	var wordPatterns []string
	seen := make(map[string]bool)

	lex.All(func(e lexicon.Entry) bool {
		for _, v := range e.Variants {
			if v.Kind != lexicon.VariantHomophone {
				continue
			}
			if !v.IsValid() || v.Text == e.Text {
				continue
			}
			// A homophone variant that is a substring of its canonical
			// form would corrupt text on match (the canonical is present
			// around it). Skip here; lexicon.Builder.Validate rejects it
			// in strict mode.
			if strings.Contains(e.Text, v.Text) {
				continue
			}
			if seen[v.Text] {
				continue
			}
			seen[v.Text] = true
			conf := v.Confidence
			if conf == 0 {
				conf = defaultConfidence(e)
			}
			wordPatterns = append(wordPatterns, v.Text)
			wordFor[v.Text] = wordMatch{
				canonical:  e.Text,
				entryID:    e.ID,
				confidence: conf,
			}
		}
		return true
	})

	if len(wordPatterns) > 0 {
		p.wordMatcher = lexicon.NewMatcher(wordPatterns)
		p.wordFor = wordFor
	}

	if converter == nil {
		return p
	}

	// Per-character path: index single-character entries only.
	pinyinToEntry := make(map[string]lexicon.Entry)
	confidenceFor := make(map[string]float64)

	lex.All(func(e lexicon.Entry) bool {
		if utf8.RuneCountInString(e.Text) != 1 {
			return true
		}
		for _, form := range converter.ToPinyin(e.Text) {
			if _, exists := pinyinToEntry[form]; exists {
				continue // first-wins for duplicate pinyin
			}
			pinyinToEntry[form] = e
			confidenceFor[form] = defaultConfidence(e)
		}
		return true
	})

	p.pinyinToEntry = pinyinToEntry
	p.confidenceFor = confidenceFor
	return p
}

// defaultConfidence returns the per-Entry default confidence for
// phonetic matching. If the Entry has at least one Variant{Homophone},
// we use the maximum confidence; otherwise 0.85 (high-certainty
// default).
func defaultConfidence(e lexicon.Entry) float64 {
	maxConf := 0.0
	for _, v := range e.Variants {
		if v.Kind == lexicon.VariantHomophone {
			if v.Confidence > maxConf {
				maxConf = v.Confidence
			}
		}
	}
	if maxConf > 0 {
		return maxConf
	}
	return 0.85
}

// Name implements lexnorm.Processor.
func (p *Processor) Name() string { return Name }

// Version implements lexnorm.Versioner.
func (p *Processor) Version() string { return Version }

// Certainty implements lexnorm.CertaintyReporter.
//
// Phonetic is medium-certainty: homophones are inherently ambiguous
// (multiple characters share the same pronunciation). It sits
// BELOW Alias / Deterministic and ABOVE Approximate / Contextual in
// the certainty hierarchy.
func (p *Processor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyMedium }

// Process implements lexnorm.Processor.
//
// First applies whole-word homophone matches (precise), then scans
// remaining characters for single-character phonetic fixes. Apply /
// Suggest / Skip per confidence vs. Config thresholds.
func (p *Processor) Process(_ context.Context, s *lexnorm.State) error {
	autoApply := s.Config().AutoApplyThreshold
	suggest := s.Config().SuggestThreshold

	// Track spans consumed by whole-word matches so the per-character
	// scan does not emit redundant suggestions inside them.
	type span struct{ start, end int }
	var consumed []span
	isConsumed := func(start, end int) bool {
		for _, c := range consumed {
			if start < c.end && end > c.start {
				return true
			}
		}
		return false
	}

	decide := func(span lexnorm.Span, to string, conf float64, rule string, entryID lexicon.EntryID, pattern string) {
		meta := lexnorm.ChangeMeta{
			Source:     Name,
			Confidence: conf,
			RuleID:     rule,
			EntryID:    string(entryID),
			Reason:     fmt.Sprintf("pinyin %s → %s", pattern, to),
		}
		if conf >= autoApply {
			_ = s.Replace(span, to, meta)
		} else if conf >= suggest {
			_ = s.Suggest(span, to, meta)
		}
		// else: Skip
	}

	// 1. Whole-word homophone path.
	if p.wordMatcher != nil {
		for _, m := range p.wordMatcher.Match(s.Original()) {
			wm, ok := p.wordFor[m.Pattern]
			if !ok {
				continue
			}
			decide(
				lexnorm.Span{Start: m.Start, End: m.End},
				wm.canonical, wm.confidence, "homophone", wm.entryID, m.Pattern,
			)
			consumed = append(consumed, span{start: m.Start, end: m.End})
		}
	}

	// 2. Per-character path (single-character entries only).
	if len(p.pinyinToEntry) == 0 {
		return nil
	}
	original := s.Original()
	for i := 0; i < len(original); {
		r, size := utf8.DecodeRuneInString(original[i:])
		if !isLikelyCJK(r) {
			i += size
			continue
		}
		if isConsumed(i, i+size) {
			i += size
			continue
		}

		forms := p.converter.ToPinyin(string(r))
		var bestEntry lexicon.Entry
		var bestConf float64
		var bestForm string
		for _, form := range forms {
			entry, ok := p.pinyinToEntry[form]
			if !ok {
				continue
			}
			conf := p.confidenceFor[form]
			if conf > bestConf {
				bestEntry = entry
				bestConf = conf
				bestForm = form
			}
		}
		if bestEntry.ID == "" {
			i += size
			continue
		}

		decide(
			lexnorm.Span{Start: i, End: i + size},
			bestEntry.Text, bestConf, "homophone", bestEntry.ID, bestForm,
		)
		i += size
	}
	return nil
}

// isLikelyCJK returns true if r is in the CJK Unified Ideographs block
// (basic CJK, no extensions). This is a coarse filter to skip Latin /
// digits / punctuation in the per-character scan.
//
// The filter is conservative: characters outside the basic CJK block
// are skipped to avoid spurious phonetic matching on, e.g., emoji or
// Latin punctuation. For non-CJK input, use Alias / Deterministic /
// Approximate instead.
func isLikelyCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// Descriptor is the Registry Descriptor for this Processor.
//
// See alias.Descriptor for notes on Lexicon binding. Phonetic
// additionally requires a PinyinConverter for the per-character path;
// the Descriptor returns a Processor without one (whole-word path
// still active when a Lexicon is injected by the Engine).
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyMedium,
	New: func(_ json.RawMessage) (lexnorm.Processor, error) {
		return New(nil, nil), nil
	},
	Default: func() any { return nil },

	Version:           Version,
	Category:          lexnorm.CategoryPhonetic,
	MutatesText:       true,
	SupportsSuggest:   true,
	SupportsProtected: true,
	Deterministic:     true,
	Determinism:       lexnorm.DeterministicTrue,
	DefaultOrder:      5,
	Description:       "Phonetic correction via pluggable converter; Chinese Pinyin is one implementation.",
}

// Descriptor implements lexnorm.DescribedProcessor: it exposes the
// capability metadata of this Processor (category, mutation mode,
// determinism) for Registry queries, audit tooling, and docs.
func (p *Processor) Descriptor() lexnorm.Descriptor { return Descriptor }
