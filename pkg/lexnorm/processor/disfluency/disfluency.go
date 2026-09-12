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

// Package disfluency implements the Noise Processor.
//
// # Purpose
//
// Removes speech noise: filler words, modal particles, stutter
// repeats, and repeated phrases.
//
// # Noise Safety Policy
//
// Not all speech noise is unambiguous. The Processor groups filler
// tokens in two tiers:
//
//   - Unambiguous single-character particles (呃 嗯 啊 哦 诶) are
//     removed unconditionally: they carry no lexical meaning.
//   - Multi-character tokens (那个 这个 然后 就是说 其实 反正 你知道)
//     can carry meaning ("那个文件给我" — "that file"). By default
//     they are removed ONLY when standalone, i.e. delimited by text
//     boundaries (start/end of text, whitespace, punctuation) on both
//     sides. WithAggressiveFillers restores the legacy behavior of
//     unconditional removal.
//
// Repeated-word and repeated-phrase detection NEVER modify text in
// the default configuration: runs of identical characters / phrases
// produce Suggestions so a human (or downstream policy) decides.
// Folding "哈哈哈哈" (laughter) automatically would be wrong; folding
// "啊啊啊啊" (stutter) often right — that judgment is context.
//
// # Behavior
//
// The Processor walks the Original text for each configured token and
// replaces each removal via State.Replace. When a removal leaves a
// double space behind, ONE adjacent space is consumed with the token
// so the output does not degrade.
//
// # Order in Standard Pipeline
//
// Noise runs AFTER Normalize (whitespace consistent) and BEFORE
// Canonicalization / Deterministic (so filler words do not pollute
// matching).
//
// # Certainty
//
// Noise removal of unambiguous particles is high-certainty. Ambiguous
// token removal is guarded; repeated-word folding is suggestion-only.
package disfluency

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/stack-haven/lexnorm"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "disfluency"

	// Version is the semantic version of this Processor.
	Version = "v1"
)

// unambiguousFillers carry no lexical meaning and are removed
// unconditionally in the default configuration.
var unambiguousFillers = []string{"呃", "嗯", "啊", "哦", "诶"}

// ambiguousFillers may carry meaning in context; by default they are
// removed only when standalone (boundary-guarded).
var ambiguousFillers = []string{"那个", "这个", "然后", "就是说", "其实", "反正", "你知道"}

// legacyFillers is the v1 default token list (unconditional removal),
// restored via WithAggressiveFillers.
var legacyFillers = append(append([]string(nil), unambiguousFillers...), ambiguousFillers...)

// match is one candidate filler occurrence in Original coordinates.
type match struct{ start, end int }

// Processor removes speech noise from text.
type Processor struct {
	tokens []string

	// guardMultiChar removes multi-char tokens only when standalone.
	guardMultiChar bool

	// repeatedWordMinRun: runs of ≥ N identical characters produce a
	// Suggest collapsing the run. 0 disables. Default 3.
	repeatedWordMinRun int

	// repeatedPhraseMaxLen / repeatedPhraseMinRepeats: immediate
	// repetitions of a 2..maxLen-rune phrase repeated ≥ minRepeats
	// times produce a Suggest collapsing the run. 0 disables.
	// Defaults 4 and 3.
	repeatedPhraseMaxLen    int
	repeatedPhraseMinRepeat int
}

// New returns a Processor with the default noise policy:
//
//   - unambiguous particles removed unconditionally
//   - ambiguous multi-char tokens removed only when standalone
//   - repeated-word / repeated-phrase detection produces Suggestions
//     only (never modifies text)
func New() *Processor {
	return &Processor{
		tokens:                  append(append([]string(nil), unambiguousFillers...), ambiguousFillers...),
		guardMultiChar:          true,
		repeatedWordMinRun:      3,
		repeatedPhraseMaxLen:    4,
		repeatedPhraseMinRepeat: 3,
	}
}

// WithTokens overrides the filler-word list with the given tokens.
//
// Tokens passed explicitly are matched as exact substrings and removed
// unconditionally (the caller takes responsibility for their
// semantics; this matches the historical WithTokens contract).
func (p *Processor) WithTokens(tokens ...string) *Processor {
	p.tokens = append([]string(nil), tokens...)
	p.guardMultiChar = false
	return p
}

// WithAggressiveFillers restores the legacy v1 behavior: the full
// default token list (including 那个 / 这个 / ...) removed
// unconditionally wherever they occur.
func (p *Processor) WithAggressiveFillers() *Processor {
	p.tokens = append([]string(nil), legacyFillers...)
	p.guardMultiChar = false
	return p
}

// WithRepeatedWordFolding configures repeated-character detection.
// Runs of ≥ minRun identical characters yield a Suggest (never an
// Apply). minRun < 2 disables the detection.
func (p *Processor) WithRepeatedWordFolding(minRun int) *Processor {
	if minRun < 2 {
		minRun = 0
	}
	p.repeatedWordMinRun = minRun
	return p
}

// WithRepeatedPhraseFolding configures repeated-phrase detection.
// Immediate repetitions (≥ minRepeats) of a 2..maxPhraseLen rune
// phrase yield a Suggest (never an Apply). maxPhraseLen < 2 or
// minRepeats < 2 disables the detection.
func (p *Processor) WithRepeatedPhraseFolding(maxPhraseLen, minRepeats int) *Processor {
	if maxPhraseLen < 2 || minRepeats < 2 {
		maxPhraseLen, minRepeats = 0, 0
	}
	p.repeatedPhraseMaxLen = maxPhraseLen
	p.repeatedPhraseMinRepeat = minRepeats
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

// isWordRune reports whether r could be part of a meaningful word
// (letters, digits, CJK ideographs). Punctuation and whitespace are
// boundaries.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// standaloneAt reports whether the occurrence original[start:end] is
// delimited by text boundaries on both sides.
func standaloneAt(original string, start, end int) bool {
	if start > 0 {
		r, _ := lastRuneBefore(original, start)
		if isWordRune(r) {
			return false
		}
	}
	if end < len(original) {
		r, _ := utf8DecodeRune(original[end:])
		if isWordRune(r) {
			return false
		}
	}
	return true
}

func utf8DecodeRune(s string) (rune, int) {
	for i, r := range s {
		_ = i
		return r, len(string(r))
	}
	return 0, 0
}

func lastRuneBefore(s string, pos int) (rune, int) {
	// Walk back over at most 4 bytes to find a valid rune boundary.
	for i := 1; i <= 4 && i <= pos; i++ {
		b := s[pos-i]
		if b&0xC0 != 0x80 { // not a continuation byte → rune start
			r, size := utf8DecodeRune(s[pos-i:])
			if size == i {
				return r, i
			}
		}
	}
	return 0, 0
}

// removalSpan expands [start,end) to consume ONE adjacent space so the
// removal does not leave a double space behind.
func removalSpan(original string, start, end int) (int, int) {
	if end < len(original) && original[end] == ' ' {
		return start, end + 1
	}
	if start > 0 && original[start-1] == ' ' {
		return start - 1, end
	}
	return start, end
}

// Process implements lexnorm.Processor.
//
// Walks the Original text for each token; replaces each occurrence
// with an empty string via State.Replace. Matches are de-duplicated of
// overlaps (longer wins) and applied right-to-left so Original offsets
// remain valid. Repeated-word / repeated-phrase runs are recorded as
// Suggestions.
func (p *Processor) Process(_ context.Context, s *lexnorm.State) error {
	original := s.Original()

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
			// Noise safety policy: guarded multi-char tokens are removed
			// only when standalone.
			if p.guardMultiChar && isMultiRune(token) && !standaloneAt(original, start, end) {
				idx = end
				continue
			}
			matches = append(matches, match{start, end})
			idx = end
		}
	}

	if len(matches) > 0 {
		// Sort by start ascending, then by end descending (longer wins
		// on tie).
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].start != matches[j].start {
				return matches[i].start < matches[j].start
			}
			return matches[i].end > matches[j].end
		})

		// De-overlap: keep a match only if it doesn't overlap the
		// previously kept match.
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

		// Apply right-to-left so earlier Original positions are
		// unaffected by the contraction caused by later replaces.
		for i := len(matches) - 1; i >= 0; i-- {
			m := matches[i]
			rs, re := removalSpan(original, m.start, m.end)
			// Errors from conflicting replaces (overlaps with locked
			// regions or earlier processors' edits) are silently ignored.
			_ = s.Replace(
				lexnorm.Span{Start: rs, End: re},
				"",
				lexnorm.ChangeMeta{
					Source:     Name,
					Confidence: 1.0,
					RuleID:     "filler",
					Reason:     "filler word removal",
				},
			)
		}
	}

	p.suggestRepeatedRuns(s, original, matches)
	return nil
}

// suggestRepeatedRuns detects runs of identical characters and
// immediately repeated phrases and records collapse Suggestions.
// Suggestions never modify State.Text.
func (p *Processor) suggestRepeatedRuns(s *lexnorm.State, original string, removals []match) {
	overlapsRemoval := func(start, end int) bool {
		for _, m := range removals {
			if start < m.end && end > m.start {
				return true
			}
		}
		return false
	}
	runes := []rune(original)
	// byteAt maps rune index → byte offset, built lazily: most inputs
	// contain no repeated runs, and both scans below can express their
	// results in rune indices until a suggestion actually fires.
	var byteAt []int

	suggest := func(rStart, rEnd int, collapsed string) {
		if rStart < 0 || rEnd > len(runes) || rEnd <= rStart {
			return
		}
		if byteAt == nil {
			byteAt = make([]int, len(runes)+1)
			off := 0
			for i, r := range runes {
				byteAt[i] = off
				off += len(string(r))
			}
			byteAt[len(runes)] = off
		}
		bStart, bEnd := byteAt[rStart], byteAt[rEnd]
		if overlapsRemoval(bStart, bEnd) {
			return
		}
		_ = s.Suggest(
			lexnorm.Span{Start: bStart, End: bEnd},
			collapsed,
			lexnorm.ChangeMeta{
				Source:     Name,
				Confidence: 0.9,
				RuleID:     "repeated-run",
				Reason:     "repeated run detected; collapse suggestion (not applied)",
			},
		)
	}

	// 1. Identical single-character runs.
	if n := p.repeatedWordMinRun; n >= 2 {
		i := 0
		for i < len(runes) {
			j := i
			for j+1 < len(runes) && runes[j+1] == runes[i] {
				j++
			}
			run := j - i + 1
			if run >= n {
				suggest(i, j+1, string(runes[i]))
			}
			i = j + 1
		}
	}

	// 2. Immediately repeated phrases (2..maxLen runes, ≥ minRepeats).
	if p.repeatedPhraseMaxLen >= 2 && p.repeatedPhraseMinRepeat >= 2 {
		for i := 0; i < len(runes); i++ {
			for L := p.repeatedPhraseMaxLen; L >= 2; L-- {
				if i+L > len(runes) {
					continue
				}
				phrase := string(runes[i : i+L])
				// A phrase that is one repeated character is already
				// covered by rule 1.
				if isSingleRuneRepeat(phrase) {
					continue
				}
				k := 1
				for i+(k+1)*L <= len(runes) && string(runes[i+(k)*L:i+(k+1)*L]) == phrase {
					k++
				}
				if k >= p.repeatedPhraseMinRepeat {
					suggest(i, i+k*L, phrase)
					i += k*L - 1 // resume after the run
					break
				}
			}
		}
	}
}

func isSingleRuneRepeat(s string) bool {
	rs := []rune(s)
	for _, r := range rs {
		if r != rs[0] {
			return false
		}
	}
	return true
}

func isMultiRune(token string) bool { return len([]rune(token)) > 1 }

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

	Version:           Version,
	Category:          lexnorm.CategoryNoise,
	MutatesText:       true,
	SupportsSuggest:   true,
	SupportsProtected: true,
	Deterministic:     true,
	Determinism:       lexnorm.DeterministicTrue,
	DefaultOrder:      2,
	Description:       "Remove speech noise: filler words (guarded for ambiguous tokens); repeated-run collapse as Suggestions.",
}

// Descriptor implements lexnorm.DescribedProcessor: it exposes the
// capability metadata of this Processor (category, mutation mode,
// determinism) for Registry queries, audit tooling, and docs.
func (p *Processor) Descriptor() lexnorm.Descriptor { return Descriptor }
