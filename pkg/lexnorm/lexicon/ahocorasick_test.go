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

package lexicon_test

import (
	"sort"
	"testing"

	"github.com/stack-haven/lexnorm/lexicon"
)

func TestAhoCorasick_Empty(t *testing.T) {
	ac := lexicon.NewAhoCorasick(nil)
	if got := ac.Match("anything"); got != nil {
		t.Errorf("empty automaton must match nothing, got %v", got)
	}

	ac = lexicon.NewAhoCorasick([]string{})
	if got := ac.Match("anything"); got != nil {
		t.Errorf("empty slice must match nothing, got %v", got)
	}
}

func TestAhoCorasick_SinglePattern(t *testing.T) {
	ac := lexicon.NewAhoCorasick([]string{"hello"})
	matches := ac.Match("say hello world")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Start != 4 || matches[0].End != 9 {
		t.Errorf("span = [%d,%d), want [4,9)", matches[0].Start, matches[0].End)
	}
	if matches[0].Pattern != "hello" {
		t.Errorf("Pattern = %q, want %q", matches[0].Pattern, "hello")
	}
}

func TestAhoCorasick_MultipleOccurrences(t *testing.T) {
	ac := lexicon.NewAhoCorasick([]string{"ab"})
	matches := ac.Match("ababab")
	if len(matches) != 3 {
		t.Errorf("expected 3 matches in 'ababab', got %d", len(matches))
	}
	want := []int{0, 2, 4}
	for i, m := range matches {
		if m.Start != want[i] {
			t.Errorf("match[%d].Start = %d, want %d", i, m.Start, want[i])
		}
	}
}

func TestAhoCorasick_OverlappingPatterns(t *testing.T) {
	// Classic Aho-Corasick example.
	ac := lexicon.NewAhoCorasick([]string{"he", "she", "his", "hers"})
	text := "ushers"
	matches := ac.Match(text)

	// Expected: "he" at [1,3), "she" at [1,4), "hers" at [1,5).
	if len(matches) != 3 {
		t.Errorf("expected 3 matches, got %d: %+v", len(matches), matches)
	}
}

func TestAhoCorasick_NoMatch(t *testing.T) {
	ac := lexicon.NewAhoCorasick([]string{"xyz", "abc"})
	matches := ac.Match("hello world")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestAhoCorasick_DuplicatePatterns(t *testing.T) {
	ac := lexicon.NewAhoCorasick([]string{"a", "a", "b"})
	matches := ac.Match("ab")
	// Two duplicate "a" patterns + one "b" pattern = 3 matches.
	if len(matches) != 3 {
		t.Errorf("expected 3 matches (a×2 + b×1), got %d", len(matches))
	}
	// Count how many "a" vs "b".
	aCount, bCount := 0, 0
	for _, m := range matches {
		switch m.Pattern {
		case "a":
			aCount++
		case "b":
			bCount++
		}
	}
	if aCount != 2 || bCount != 1 {
		t.Errorf("got aCount=%d bCount=%d, want 2/1", aCount, bCount)
	}
}

func TestAhoCorasick_UTF8(t *testing.T) {
	// Chinese: 你好 (3 bytes each)
	ac := lexicon.NewAhoCorasick([]string{"你好"})
	matches := ac.Match("say 你好 world")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	// 你好 is 6 bytes; preceded by "say " (4 bytes).
	if matches[0].Start != 4 || matches[0].End != 10 {
		t.Errorf("UTF-8 span = [%d,%d), want [4,10)", matches[0].Start, matches[0].End)
	}
}

func TestAhoCorasick_MixedLength(t *testing.T) {
	// Patterns of different lengths match at the same position.
	ac := lexicon.NewAhoCorasick([]string{"a", "ab", "abc"})
	matches := ac.Match("xabc")
	// At position 3: "a" at [1,2), "ab" at [1,3), "abc" at [1,4).
	if len(matches) != 3 {
		t.Errorf("expected 3 matches at position 1, got %d: %+v", len(matches), matches)
	}
}

func TestAhoCorasick_PatternIdx(t *testing.T) {
	ac := lexicon.NewAhoCorasick([]string{"first", "second"})
	matches := ac.Match("first and second")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	// Sort by Start to make assertion order-independent.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Start < matches[j].Start
	})
	if matches[0].PatternIdx != 0 {
		t.Errorf("first match PatternIdx = %d, want 0", matches[0].PatternIdx)
	}
	if matches[1].PatternIdx != 1 {
		t.Errorf("second match PatternIdx = %d, want 1", matches[1].PatternIdx)
	}
}

func TestAhoCorasick_PatternCount(t *testing.T) {
	ac := lexicon.NewAhoCorasick([]string{"a", "b", "c"})
	if got := ac.PatternCount(); got != 3 {
		t.Errorf("PatternCount = %d, want 3", got)
	}
}

func TestAhoCorasick_LargeInput(t *testing.T) {
	// Stress test.
	patterns := []string{"a", "ab", "abc", "abcd"}
	ac := lexicon.NewAhoCorasick(patterns)

	text := ""
	for i := 0; i < 1000; i++ {
		text += "abcd"
	}
	matches := ac.Match(text)
	// Each 4-char "abcd" yields matches for all 4 patterns.
	if len(matches) != 4*1000 {
		t.Errorf("expected %d matches, got %d", 4*1000, len(matches))
	}
}
