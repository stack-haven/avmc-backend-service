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

package lexicon

import (
	"unicode/utf8"
)

// AhoCorasick is a multi-pattern string matching automaton.
//
// # Algorithm
//
// Aho-Corasick finds all occurrences of any pattern in a set in O(text +
// matches) time after O(sum of pattern lengths) preprocessing. Patterns
// are organized into a Trie; each node carries a failure link to the
// longest proper suffix that is also a Trie node; outputs are propagated
// from failure nodes so that each match is found at the appropriate
// position during a single linear scan.
//
// # Determinism
//
// When multiple patterns match at the same position, all are reported.
// Match order in the output slice is by Start position (ascending), then
// by PatternIdx (ascending) for ties.
//
// # Byte Offsets
//
// Match.Start and Match.End are byte offsets in the input text. For
// UTF-8 text with multi-byte runes, End - Start equals the byte length
// of the matched pattern (which equals len(pattern)).
type AhoCorasick struct {
	root     *acNode
	patterns []string // for output mapping
}

// acNode is a single Trie node with failure-link and propagated outputs.
type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	outputs  []int // pattern indices ending here (after propagation from fail)
}

// Match represents one occurrence of a pattern in the input text.
type Match struct {
	// Start is the inclusive byte offset where the match begins.
	Start int

	// End is the exclusive byte offset where the match ends.
	End int

	// Pattern is the matched pattern (the original string).
	Pattern string

	// PatternIdx is the index of the pattern in the input slice.
	PatternIdx int
}

// NewAhoCorasick builds an Aho-Corasick automaton from the given patterns.
//
// An empty patterns list produces a valid automaton that matches nothing.
// Duplicate patterns are allowed; each is matched and reported separately.
func NewAhoCorasick(patterns []string) *AhoCorasick {
	ac := &AhoCorasick{
		root:     &acNode{children: make(map[rune]*acNode)},
		patterns: append([]string(nil), patterns...),
	}

	// 1. Insert all patterns into the Trie.
	for i, p := range patterns {
		node := ac.root
		for _, r := range p {
			child, ok := node.children[r]
			if !ok {
				child = &acNode{children: make(map[rune]*acNode)}
				node.children[r] = child
			}
			node = child
		}
		node.outputs = append(node.outputs, i)
	}

	// 2. BFS to compute failure links and propagate outputs.
	queue := make([]*acNode, 0)
	for _, child := range ac.root.children {
		child.fail = ac.root
		queue = append(queue, child)
	}

	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]

		for r, child := range v.children {
			// Compute failure link for child.
			f := v.fail
			for f != ac.root {
				if next, ok := f.children[r]; ok {
					child.fail = next
					break
				}
				f = f.fail
			}
			if child.fail == nil {
				if next, ok := ac.root.children[r]; ok {
					child.fail = next
				} else {
					child.fail = ac.root
				}
			}

			// Propagate outputs from failure node.
			child.outputs = append(child.outputs, child.fail.outputs...)

			queue = append(queue, child)
		}
	}

	return ac
}

// Match finds all pattern occurrences in text, sorted by Start position.
//
// Time complexity: O(len(text) + total match length) after construction.
//
// For an empty pattern set, returns nil.
func (ac *AhoCorasick) Match(text string) []Match {
	if ac == nil || len(ac.patterns) == 0 {
		return nil
	}

	var matches []Match
	node := ac.root
	bytePos := 0

	for bytePos < len(text) {
		r, size := utf8.DecodeRuneInString(text[bytePos:])

		// Follow transitions, falling back via failure links.
		for node != ac.root {
			if _, ok := node.children[r]; ok {
				break
			}
			node = node.fail
		}
		if next, ok := node.children[r]; ok {
			node = next
		} else {
			node = ac.root
		}

		// Emit all outputs (patterns ending at this position).
		for _, idx := range node.outputs {
			pattern := ac.patterns[idx]
			endByte := bytePos + size
			startByte := endByte - len(pattern)
			matches = append(matches, Match{
				Start:      startByte,
				End:        endByte,
				Pattern:    pattern,
				PatternIdx: idx,
			})
		}

		bytePos += size
	}

	return matches
}

// PatternCount returns the number of patterns in the automaton.
func (ac *AhoCorasick) PatternCount() int {
	if ac == nil {
		return 0
	}
	return len(ac.patterns)
}
