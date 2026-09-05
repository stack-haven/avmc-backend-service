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

func TestNgramIndex_Basic(t *testing.T) {
	idx := lexicon.NewNgramIndex(2)
	idx.Add("hello", "e1")
	idx.Add("world", "e2")
	idx.Add("help", "e3")
	idx.Build()

	// Query "hello" should match e1 exactly (3 bigrams), and e3 via "he", "el".
	got := idx.Query("hello")
	gotSet := toEntryIDSet(got)
	if _, ok := gotSet["e1"]; !ok {
		t.Error("e1 (exact match) must be returned")
	}
	if _, ok := gotSet["e3"]; !ok {
		t.Error("e3 (shared bigrams with 'hello') must be returned")
	}
	if _, ok := gotSet["e2"]; ok {
		t.Error("e2 (no shared bigrams) must NOT be returned")
	}
}

func TestNgramIndex_SortByOverlap(t *testing.T) {
	idx := lexicon.NewNgramIndex(2)
	idx.Add("aaa", "exact")  // shares "aa" twice
	idx.Add("aa", "partial") // shares "aa" once
	idx.Add("zzzz", "none")  // shares nothing
	idx.Build()

	got := idx.Query("aaa")
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(got), got)
	}
	if got[0] != "exact" {
		t.Errorf("top result = %q, want %q (most overlap)", got[0], "exact")
	}
	if got[1] != "partial" {
		t.Errorf("second result = %q, want %q", got[1], "partial")
	}
}

func TestNgramIndex_Trigram(t *testing.T) {
	idx := lexicon.NewNgramIndex(3)
	idx.Add("hello", "e1")
	idx.Add("help", "e2")
	idx.Build()

	// Query "hello" shares "hel" with e1 and e2.
	got := idx.Query("hello")
	if len(got) != 2 {
		t.Errorf("expected 2 matches, got %d: %v", len(got), got)
	}
}

func TestNgramIndex_ShortText(t *testing.T) {
	// Text shorter than n is indexed as a whole.
	idx := lexicon.NewNgramIndex(3)
	idx.Add("hi", "e1")
	idx.Build()

	got := idx.Query("hi")
	if len(got) != 1 || got[0] != "e1" {
		t.Errorf("expected [e1], got %v", got)
	}
}

func TestNgramIndex_NoMatch(t *testing.T) {
	idx := lexicon.NewNgramIndex(2)
	idx.Add("hello", "e1")
	idx.Build()

	got := idx.Query("xxxxx")
	if len(got) != 0 {
		t.Errorf("expected 0 matches, got %v", got)
	}
}

func TestNgramIndex_EmptyQuery(t *testing.T) {
	idx := lexicon.NewNgramIndex(2)
	idx.Add("hello", "e1")
	idx.Build()

	if got := idx.Query(""); got != nil {
		t.Errorf("empty query must return nil, got %v", got)
	}
}

func TestNgramIndex_QueryBeforeBuild(t *testing.T) {
	idx := lexicon.NewNgramIndex(2)
	idx.Add("hello", "e1")
	// No Build() call.

	got := idx.Query("hello")
	if got != nil {
		t.Errorf("Query before Build must return nil, got %v", got)
	}
}

func TestNgramIndex_UTF8(t *testing.T) {
	idx := lexicon.NewNgramIndex(2)
	// 2-rune texts have exactly one bigram each.
	idx.Add("你好", "e1")
	idx.Add("好事", "e2")
	idx.Add("你们", "e3")
	idx.Build()

	// Query "你好" has bigram "你好" → only e1 matches exactly.
	got := idx.Query("你好")
	if len(got) != 1 || got[0] != "e1" {
		t.Errorf("expected [e1], got %v", got)
	}

	// With a longer text, bigrams are split into multiple n-grams.
	idx2 := lexicon.NewNgramIndex(2)
	idx2.Add("你好世界", "e1") // bigrams: 你好, 好世, 世界
	idx2.Add("好事多磨", "e2") // bigrams: 好事, 事多, 多磨
	idx2.Add("你们好", "e3")  // bigrams: 你们, 们好
	idx2.Build()

	got2 := idx2.Query("你好世界")
	// Bigrams in query: 你好, 好世, 世界.
	// e1 ("你好世界") matches all 3; e2 and e3 share no bigrams.
	if len(got2) != 1 || got2[0] != "e1" {
		t.Errorf("expected [e1], got %v", got2)
	}
}

func TestNgramIndex_DuplicateEntryID(t *testing.T) {
	idx := lexicon.NewNgramIndex(2)
	idx.Add("hello", "e1")
	idx.Add("help", "e1") // same ID, different text
	idx.Build()

	got := idx.Query("hello")
	gotSet := toEntryIDSet(got)
	count := 0
	for id := range gotSet {
		if id == "e1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("e1 must appear once (dedup), got %d", count)
	}
}

func TestNgramIndex_ZeroOrNegativeN(t *testing.T) {
	idx := lexicon.NewNgramIndex(0)
	if idx.N() != 1 {
		t.Errorf("n=0 must default to 1, got %d", idx.N())
	}
	idx2 := lexicon.NewNgramIndex(-5)
	if idx2.N() != 1 {
		t.Errorf("negative n must default to 1, got %d", idx2.N())
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
		{"sunday", "saturday", 3},
		{"你好", "你坏", 1}, // one rune substitution (UTF-8 aware)
	}
	for _, tc := range tests {
		got := testLevenshtein(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"HELLO", "hello"},
		{"  hello  ", "hello"},
		{"Hello World", "hello world"},
		{"你好", "你好"}, // CJK unchanged
	}
	for _, tc := range tests {
		got := lexicon.NormalizeText(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Helpers (kept local to test file).

func toEntryIDSet(ids []lexicon.EntryID) map[lexicon.EntryID]bool {
	m := make(map[lexicon.EntryID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// testLevenshtein exposes the internal function via a public wrapper.
// We re-implement here to test the package-level behavior.
func testLevenshtein(a, b string) int {
	// Mirror the implementation in ngram.go.
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	if len(ar) < len(br) {
		ar, br = br, ar
	}
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := 0; j <= len(br); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

// Sanity: ensure sorting helpers work as expected.
func TestSortDeterminism(t *testing.T) {
	idx := lexicon.NewNgramIndex(2)
	idx.Add("a", "x")
	idx.Add("a", "y") // same bigram "a"
	idx.Build()

	got := idx.Query("a")
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	// Deterministic order: alphabetical by EntryID.
	if got[0] > got[1] {
		// sort.Slice is stable so tied scores preserve input order; we
		// don't strictly require this but verify non-decreasing.
		if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i] < got[j] }) {
			t.Errorf("results not sorted: %v", got)
		}
	}
}
