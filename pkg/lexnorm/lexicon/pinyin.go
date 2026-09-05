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

import "sort"

// PinyinConverter maps a Chinese text string to its pinyin
// representation(s).
//
// # Why Interface
//
// Chinese-to-pinyin conversion requires a pronunciation dictionary
// (e.g., 汉字 → 拼音 mappings, including 多音字 handling). The core
// package does not bundle such a dictionary to keep the dependency
// surface minimal. Application code provides a concrete implementation
// (often via a third-party library or a project-local dictionary).
//
// # Conversion Semantics
//
// ToPinyin returns one or more pinyin forms for text. A word with
// multiple readings (多音字) returns multiple forms; callers should
// index and query all forms. ASCII characters are passed through
// unchanged (no case folding is performed).
type PinyinConverter interface {
	// ToPinyin returns the pinyin form(s) of text.
	//
	// The returned slice is non-empty for valid input. The order of
	// forms in the slice is implementation-defined.
	ToPinyin(text string) []string
}

// PinyinIndex is a simple inverted index mapping pinyin strings to
// EntryIDs.
//
// # Purpose
//
// For "same sound, different character" homophone matching: convert
// the candidate text to pinyin via PinyinConverter, look up entries
// indexed by the resulting pinyin(s), and use those entries as
// candidates for further matching.
//
// # Usage
//
//	idx := NewPinyinIndex()
//	converter := myConverter  // application-provided
//	for _, e := range entries {
//	    for _, p := range converter.ToPinyin(e.Text) {
//	        idx.Add(p, e.ID)
//	    }
//	}
//	idx.Build()
//
//	matches := idx.Query(converter.ToPinyin("你好"))  // []EntryID
//
// # Concurrency
//
// PinyinIndex is safe for concurrent read access after Build.
type PinyinIndex struct {
	entries map[string][]EntryID
	built   bool
}

// NewPinyinIndex creates an empty PinyinIndex.
func NewPinyinIndex() *PinyinIndex {
	return &PinyinIndex{
		entries: make(map[string][]EntryID),
	}
}

// Add indexes a pinyin form under the given EntryID.
//
// Multiple calls with the same (pinyin, id) accumulate (id is added
// once per unique combination).
func (idx *PinyinIndex) Add(pinyin string, id EntryID) {
	if idx == nil || pinyin == "" || id.IsZero() {
		return
	}
	idx.entries[pinyin] = appendUnique(idx.entries[pinyin], id)
}

// Build finalizes the index. Must be called before Query.
func (idx *PinyinIndex) Build() {
	if idx == nil {
		return
	}
	idx.built = true
}

// Query returns all EntryIDs indexed under the given pinyin forms,
// sorted by EntryID for determinism.
//
// Returns nil if no EntryIDs match.
func (idx *PinyinIndex) Query(pinyins ...string) []EntryID {
	if idx == nil || !idx.built || len(pinyins) == 0 {
		return nil
	}

	seen := make(map[EntryID]bool)
	var out []EntryID
	for _, p := range pinyins {
		for _, id := range idx.entries[p] {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Size returns the number of unique pinyin keys.
func (idx *PinyinIndex) Size() int {
	if idx == nil {
		return 0
	}
	return len(idx.entries)
}

// PassthroughConverter is a no-op PinyinConverter that returns the
// input text unchanged (as a single-element slice).
//
// Useful for: tests, ASCII-only deployments, or as a placeholder when
// no real converter is available. NOT useful for CJK phonetic matching.
type PassthroughConverter struct{}

// ToPinyin returns text unchanged as a single-element slice.
func (PassthroughConverter) ToPinyin(text string) []string {
	if text == "" {
		return nil
	}
	return []string{text}
}

// Compile-time interface assertions.
var (
	_ PinyinConverter = PassthroughConverter{}
)
