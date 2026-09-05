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
	"sort"
	"strings"
	"unicode/utf8"
)

// NgramIndex is a simple n-gram inverted index for fuzzy candidate
// retrieval.
//
// # Purpose
//
// Given a candidate query string, NgramIndex returns the IDs of all
// Entries that share at least one n-gram with the query. This is used
// to prune the candidate set before expensive distance calculations
// (e.g., Levenshtein) in Fuzzy Processor (M9).
//
// # N-gram Definition
//
// For n-gram size N (typically 2 or 3), the n-grams of a string are all
// consecutive N-rune substrings. For example, with N=2:
//
//	"hello" → "he", "el", "ll", "lo"
//
// For ASCII text, n-grams are byte substrings. For Unicode text, they
// are rune substrings (UTF-8-aware).
//
// # Concurrency
//
// NgramIndex is safe for concurrent read access after Build is called.
// Build itself is NOT safe for concurrent use.
type NgramIndex struct {
	n       int
	entries map[string][]EntryID // n-gram → EntryIDs
	built   bool
}

// NewNgramIndex creates an empty NgramIndex with the given n-gram size.
//
// Typical values: 2 (bigram) or 3 (trigram). n < 1 is treated as 1.
// Larger n values yield more precise retrieval but consume more memory.
func NewNgramIndex(n int) *NgramIndex {
	if n < 1 {
		n = 1
	}
	return &NgramIndex{
		n:       n,
		entries: make(map[string][]EntryID),
	}
}

// Add indexes text under the given EntryID.
//
// Multiple Add calls with the same EntryID accumulate (i.e., the ID
// is added once per call).
func (idx *NgramIndex) Add(text string, id EntryID) {
	if idx == nil || text == "" || id.IsZero() {
		return
	}
	idx.addRunes(text, id)
}

func (idx *NgramIndex) addRunes(text string, id EntryID) {
	runes := []rune(text)
	if len(runes) < idx.n {
		// For texts shorter than n, index the whole text as one n-gram.
		idx.entries[text] = appendUnique(idx.entries[text], id)
		return
	}
	for i := 0; i+idx.n <= len(runes); i++ {
		gram := string(runes[i : i+idx.n])
		idx.entries[gram] = appendUnique(idx.entries[gram], id)
	}
}

// appendUnique adds id to the slice if not already present.
func appendUnique(s []EntryID, id EntryID) []EntryID {
	for _, existing := range s {
		if existing == id {
			return s
		}
	}
	return append(s, id)
}

// Query returns all EntryIDs that share at least one n-gram with text,
// sorted by descending overlap count (best candidates first).
//
// Returns an empty slice if no EntryIDs match.
func (idx *NgramIndex) Query(text string) []EntryID {
	if idx == nil || !idx.built || text == "" {
		return nil
	}

	// Count n-gram overlaps per EntryID.
	overlaps := make(map[EntryID]int)
	runes := []rune(text)
	if len(runes) < idx.n {
		if ids, ok := idx.entries[text]; ok {
			for _, id := range ids {
				overlaps[id]++
			}
		}
	} else {
		seen := make(map[string]bool)
		for i := 0; i+idx.n <= len(runes); i++ {
			gram := string(runes[i : i+idx.n])
			if seen[gram] {
				continue
			}
			seen[gram] = true
			if ids, ok := idx.entries[gram]; ok {
				for _, id := range ids {
					overlaps[id]++
				}
			}
		}
	}

	// Sort by descending overlap.
	type entry struct {
		id    EntryID
		score int
	}
	out := make([]entry, 0, len(overlaps))
	for id, score := range overlaps {
		out = append(out, entry{id, score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].id < out[j].id
	})

	result := make([]EntryID, len(out))
	for i, e := range out {
		result[i] = e.id
	}
	return result
}

// Build finalizes the index. Must be called before Query.
//
// After Build, the index is read-only and safe for concurrent access.
func (idx *NgramIndex) Build() {
	if idx == nil {
		return
	}
	idx.built = true
}

// Size returns the number of unique n-grams in the index.
func (idx *NgramIndex) Size() int {
	if idx == nil {
		return 0
	}
	return len(idx.entries)
}

// N returns the n-gram size.
func (idx *NgramIndex) N() int {
	if idx == nil {
		return 0
	}
	return idx.n
}

// LevenshteinDistance computes the edit distance between two strings
// using the classic dynamic-programming algorithm.
//
// Time: O(len(a) * len(b)). Space: O(min(len(a), len(b))) via row reuse.
//
// Distance is the minimum number of single-character edits (insertions,
// deletions, substitutions) to transform a into b.
func LevenshteinDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}

	// Use the shorter string for the column dimension.
	if len(ar) < len(br) {
		ar, br = br, ar
	}

	// prev is the previous row; curr is the current row.
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

// NormalizeText applies a basic Unicode normalization suitable for
// case-insensitive lookup. It lowercases ASCII letters and trims
// surrounding whitespace. Non-ASCII letters (e.g., CJK ideographs)
// are passed through unchanged.
//
// For more sophisticated normalization (NFC/NFD, full Unicode case
// folding), callers should pre-normalize before passing text.
func NormalizeText(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// RuneCount returns the number of runes in s. Equivalent to
// utf8.RuneCountInString(s), exposed here for convenience.
func RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}
