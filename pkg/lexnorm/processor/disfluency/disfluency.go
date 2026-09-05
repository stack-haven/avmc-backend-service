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

// Package disfluency implements the Disfluency Processor.
//
// # Purpose
//
// Removes disfluency markers (filler words) commonly produced in
// spontaneous speech and ASR transcripts, such as:
//
//	呃 嗯 啊 哦 诶 (single-character fillers)
//	那个 这个 然后 就是说 其实 (multi-character fillers)
//
// # Behavior
//
// The Processor walks the Original text (NOT current Text) for each
// configured filler token and replaces each occurrence with an empty
// string via State.Replace. Subsequent runs (e.g., Normalize) clean up
// any double-whitespace left behind.
//
// # Order in Standard Pipeline
//
// Disfluency runs AFTER Normalize (so whitespace is consistent) and
// BEFORE Alias / Deterministic (so filler words do not pollute
// matching).
//
// # Certainty
//
// Disfluency is high-certainty: filler words are unambiguous to remove
// in isolation, though application-specific tuning is recommended.
package disfluency

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/stack-haven/lexnorm"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "disfluency"

	// Version is the semantic version of this Processor.
	Version = "v1"
)

// defaultTokens is the default list of filler words removed by the
// Disfluency Processor.
//
// Application code may override via WithTokens.
var defaultTokens = []string{
	"呃",
	"嗯",
	"啊",
	"哦",
	"诶",
	"那个",
	"这个",
	"然后",
	"就是说",
	"其实",
	"反正",
	"你知道",
}

// Processor removes disfluency markers from text.
type Processor struct {
	tokens []string
}

// New returns a Processor with the default filler-word list.
func New() *Processor {
	return &Processor{tokens: append([]string(nil), defaultTokens...)}
}

// WithTokens overrides the filler-word list with the given tokens.
//
// Returns the Processor for chaining. Tokens are matched as exact
// substrings (no fuzzy matching).
func (p *Processor) WithTokens(tokens ...string) *Processor {
	p.tokens = append([]string(nil), tokens...)
	return p
}

// Tokens returns a defensive copy of the current token list.
func (p *Processor) Tokens() []string {
	out := make([]string, len(p.tokens))
	copy(out, p.tokens)
	return out
}

// Name implements lexnorm.Processor.
func (p *Processor) Name() string { return Name }

// Version implements lexnorm.Versioner.
func (p *Processor) Version() string { return Version }

// Certainty implements lexnorm.CertaintyReporter.
func (p *Processor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyHigh }

// Process implements lexnorm.Processor.
//
// Walks the Original text for each token; replaces each occurrence with
// an empty string via State.Replace. Matches are collected, de-duplicated
// of overlaps (longer wins), and applied right-to-left so Original
// offsets remain valid as Text shrinks.
func (p *Processor) Process(_ context.Context, s *lexnorm.State) error {
	original := s.Original()

	type match struct{ start, end int }
	var matches []match

	for _, token := range p.tokens {
		if token == "" {
			continue
		}
		idx := 0
		for {
			i := strings.Index(original[idx:], token)
			if i < 0 {
				break
			}
			start := idx + i
			end := start + len(token)
			matches = append(matches, match{start, end})
			idx = end
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by start ascending, then by end descending (longer wins on tie).
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].end > matches[j].end
	})

	// De-overlap: keep a match only if it doesn't overlap the previously
	// kept match. Because we sorted by start then by length-desc, the
	// first match at a given start is the longest; later overlapping
	// matches at the same or earlier start are skipped.
	dedup := matches[:0]
	var lastEnd int
	for _, m := range matches {
		if len(dedup) > 0 && m.start < lastEnd {
			continue // overlaps previous kept match
		}
		dedup = append(dedup, m)
		lastEnd = m.end
	}
	matches = dedup

	// Apply right-to-left so earlier Original positions are unaffected
	// by the contraction caused by later (rightward) replaces.
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		// Errors from conflicting replaces (shouldn't happen after dedup)
		// are silently ignored.
		_ = s.Replace(
			lexnorm.Span{Start: m.start, End: m.end},
			"",
			lexnorm.ChangeMeta{
				Source:     Name,
				Confidence: 1.0,
				RuleID:     "filler",
				Reason:     "filler word removal",
			},
		)
	}
	return nil
}

// Descriptor is the Registry Descriptor for this Processor.
//
// Use it with lexnorm.NewRegistry().Register(disfluency.Descriptor) to
// enable dynamic / configuration-driven construction. The
// configuration is an object with a "tokens" field:
//
//	{ "tokens": ["呃", "嗯", "那个"] }
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyHigh,
	New: func(cfg json.RawMessage) (lexnorm.Processor, error) {
		p := New()
		if len(cfg) > 0 {
			var dc struct {
				Tokens []string `json:"tokens"`
			}
			if err := json.Unmarshal(cfg, &dc); err != nil {
				return nil, fmt.Errorf("disfluency config: %w", err)
			}
			if len(dc.Tokens) > 0 {
				p.WithTokens(dc.Tokens...)
			}
		}
		return p, nil
	},
	Default: func() any { return nil },
}
