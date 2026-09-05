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

// Package pinyin implements the Pinyin Processor.
//
// # Purpose
//
// Replaces characters that are homophones of canonical forms, using
// a Lexicon and a user-provided PinyinConverter.
//
// # Algorithm
//
//  1. For each character in Original, compute its pinyin forms via
//     the PinyinConverter.
//  2. Look up matching Entries via the Lexicon's PinyinIndex.
//  3. For each match, find the Variant{Homophone} confidence and
//     decide Apply / Suggest / Skip per Config thresholds.
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
// Pinyin runs AFTER Alias / Deterministic (those are exact matches)
// and BEFORE Fuzzy (which handles non-pinyin typos).
package pinyin

import (
	"context"
	"encoding/json"
	"fmt"
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

// Processor matches characters to canonical entries by pinyin.
type Processor struct {
	lex           lexicon.Lexicon
	converter     lexicon.PinyinConverter
	pinyinToEntry map[string]lexicon.Entry // pinyin → Entry
	confidenceFor map[string]float64       // pinyin → confidence
}

// New constructs a Pinyin Processor.
//
// Both lex and converter must be non-nil for the Processor to be
// functional. With either nil, the Processor is a no-op.
func New(lex lexicon.Lexicon, converter lexicon.PinyinConverter) *Processor {
	p := &Processor{lex: lex, converter: converter}
	if lex == nil || converter == nil {
		return p
	}

	pinyinToEntry := make(map[string]lexicon.Entry)
	confidenceFor := make(map[string]float64)

	lex.All(func(e lexicon.Entry) bool {
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
// pinyin matching. If the Entry has at least one Variant{Homophone},
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
// Pinyin is medium-certainty: homophones are inherently ambiguous
// (multiple characters share the same pronunciation). It sits
// BELOW Alias / Deterministic and ABOVE Fuzzy / Context in the
// certainty hierarchy.
func (p *Processor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyMedium }

// Process implements lexnorm.Processor.
//
// Walks Original character by character; for each, computes its
// pinyin forms and looks up matching Entries. Apply / Suggest / Skip
// per confidence vs. Config thresholds.
func (p *Processor) Process(_ context.Context, s *lexnorm.State) error {
	if len(p.pinyinToEntry) == 0 {
		return nil
	}

	original := s.Original()
	autoApply := s.Config().AutoApplyThreshold
	suggest := s.Config().SuggestThreshold

	for i := 0; i < len(original); {
		r, size := utf8.DecodeRuneInString(original[i:])
		char := string(r)

		// Skip non-CJK characters (Latin, digits, etc.).
		if !isLikelyCJK(r) {
			i += size
			continue
		}

		forms := p.converter.ToPinyin(char)
		if len(forms) == 0 {
			i += size
			continue
		}

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

		span := lexnorm.Span{Start: i, End: i + size}
		meta := lexnorm.ChangeMeta{
			Source:     Name,
			Confidence: bestConf,
			RuleID:     "homophone",
			EntryID:    string(bestEntry.ID),
			Reason:     fmt.Sprintf("pinyin %s → %s", bestForm, bestEntry.Text),
		}

		if bestConf >= autoApply {
			_ = s.Replace(span, bestEntry.Text, meta)
		} else if bestConf >= suggest {
			_ = s.Suggest(span, bestEntry.Text, meta)
		}
		// else: Skip

		i += size
	}
	return nil
}

// isLikelyCJK returns true if r is in the CJK Unified Ideographs block
// (basic CJK, no extensions). This is a coarse filter to skip Latin /
// digits / punctuation in the per-character scan.
//
// The filter is conservative: characters outside the basic CJK block
// are skipped to avoid spurious pinyin matching on, e.g., emoji or
// Latin punctuation. For non-CJK input, the Pinyin Processor is a
// no-op (use Alias / Deterministic / Fuzzy instead).
func isLikelyCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// Descriptor is the Registry Descriptor for this Processor.
//
// See alias.Descriptor for notes on Lexicon binding. Pinyin additionally
// requires a PinyinConverter; the Descriptor returns a no-op Processor
// without one.
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyMedium,
	New: func(_ json.RawMessage) (lexnorm.Processor, error) {
		return New(nil, nil), nil
	},
	Default: func() any { return nil },
}
