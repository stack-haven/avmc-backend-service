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
	"sync"
)

// memLexicon is the in-memory implementation of Lexicon.
//
// memLexicon is built by Builder and is safe for concurrent read access.
// Mutating the underlying maps after Build is undefined behavior; use
// Builder to construct a new Lexicon instead.
type memLexicon struct {
	// entries is sorted by ID (for deterministic All() iteration).
	entries []Entry

	// Indexes (built once by Builder).
	byID      map[EntryID]Entry
	byText    map[string]Entry      // exact match on canonical Text
	relations map[string][]Relation // canonical Text → related Relations
	patterns  []string              // Aho-Corasick patterns (canonical + variants)
	matchers  *Matcher              // pre-built Aho-Corasick automaton
	ngram     *NgramIndex
	pinyin    *PinyinIndex
	version   string

	// Read-write lock for safety, though writes only happen during Build.
	mu sync.RWMutex
}

// Build constructs a memLexicon from the given configuration.
//
// Returns ErrConflict if the entries contain duplicates (by ID or Text)
// or if relations reference unknown EntryIDs.
func buildMemLexicon(cfg memLexiconConfig) (*memLexicon, error) {
	l := &memLexicon{
		byID:      make(map[EntryID]Entry, len(cfg.entries)),
		byText:    make(map[string]Entry, len(cfg.entries)),
		relations: make(map[string][]Relation),
		version:   cfg.version,
	}

	// 1. Index entries by ID and Text; check for duplicates.
	for _, e := range cfg.entries {
		if !e.IsValid() {
			return nil, &errInvalidEntry{e: e}
		}
		if _, exists := l.byID[e.ID]; exists {
			return nil, &errDuplicateID{id: e.ID}
		}
		if _, exists := l.byText[e.Text]; exists {
			return nil, &errDuplicateText{text: e.Text}
		}
		l.byID[e.ID] = e
		l.byText[e.Text] = e
		l.entries = append(l.entries, e)
	}

	// Sort entries by ID for deterministic All() iteration.
	sort.Slice(l.entries, func(i, j int) bool {
		return l.entries[i].ID < l.entries[j].ID
	})

	// 2. Validate relations and group by canonical Text.
	for _, r := range cfg.relations {
		if _, ok := l.byID[r.From]; !ok {
			return nil, &errUnknownRelationRef{id: r.From}
		}
		if _, ok := l.byID[r.To]; !ok {
			return nil, &errUnknownRelationRef{id: r.To}
		}
		from := l.byID[r.From]
		l.relations[from.Text] = append(l.relations[from.Text], r)
	}

	// 3. Build Aho-Corasick patterns from canonical Text + Variants.
	patterns := buildPatterns(l.entries)
	l.patterns = patterns
	l.matchers = NewMatcher(patterns)

	// 4. Build n-gram index (optional).
	if cfg.ngramSize > 0 {
		l.ngram = NewNgramIndex(cfg.ngramSize)
		for _, e := range l.entries {
			l.ngram.Add(e.Text, e.ID)
		}
		l.ngram.Build()
	}

	// 5. Build Pinyin index (optional, requires converter).
	if cfg.pinyinConverter != nil && cfg.usePinyin {
		l.pinyin = NewPinyinIndex()
		for _, e := range l.entries {
			forms := cfg.pinyinConverter.ToPinyin(e.Text)
			for _, p := range forms {
				l.pinyin.Add(p, e.ID)
			}
		}
		l.pinyin.Build()
	}

	return l, nil
}

// memLexiconConfig is the internal configuration for buildMemLexicon.
type memLexiconConfig struct {
	entries         []Entry
	relations       []Relation
	version         string
	ngramSize       int // 0 = no n-gram index
	usePinyin       bool
	pinyinConverter PinyinConverter
}

// buildPatterns collects all matchable strings (canonical + variants)
// for the Aho-Corasick matcher. Order is deterministic: for each entry,
// canonical first, then variants in their original order.
func buildPatterns(entries []Entry) []string {
	var patterns []string
	for _, e := range entries {
		patterns = append(patterns, e.Text)
		for _, v := range e.Variants {
			if v.IsValid() {
				patterns = append(patterns, v.Text)
			}
		}
	}
	return patterns
}

// ----------------------------------------------------------------------------
// Lexicon interface implementation
// ----------------------------------------------------------------------------

// Entry returns the Entry with the given ID.
func (l *memLexicon) Entry(id EntryID) (Entry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	e, ok := l.byID[id]
	return e, ok
}

// Lookup returns the Entry whose canonical Text exactly matches text.
func (l *memLexicon) Lookup(text string) (Entry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	e, ok := l.byText[text]
	return e, ok
}

// Relations returns all Relations whose From matches text.
func (l *memLexicon) Relations(text string) []Relation {
	l.mu.RLock()
	defer l.mu.RUnlock()
	// Return a defensive copy.
	src := l.relations[text]
	out := make([]Relation, len(src))
	copy(out, src)
	return out
}

// All iterates over every Entry in deterministic (ID-sorted) order.
func (l *memLexicon) All(yield func(Entry) bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, e := range l.entries {
		if !yield(e) {
			return
		}
	}
}

// Matcher returns the pre-built multi-pattern matching engine.
func (l *memLexicon) Matcher() *Matcher {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.matchers
}

// Len returns the number of Entries.
func (l *memLexicon) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Version returns the Lexicon's version string.
func (l *memLexicon) Version() string {
	return l.version
}

// ----------------------------------------------------------------------------
// Error types
// ----------------------------------------------------------------------------

// errInvalidEntry is returned when an entry lacks a valid ID.
type errInvalidEntry struct {
	e Entry
}

func (e *errInvalidEntry) Error() string {
	return "lexicon: invalid entry (empty ID)"
}

// errDuplicateID is returned when two entries share an EntryID.
type errDuplicateID struct {
	id EntryID
}

func (e *errDuplicateID) Error() string {
	return "lexicon: duplicate entry ID: " + string(e.id)
}

// errDuplicateText is returned when two entries share canonical Text.
type errDuplicateText struct {
	text string
}

func (e *errDuplicateText) Error() string {
	return "lexicon: duplicate canonical text: " + e.text
}

// errUnknownRelationRef is returned when a relation references an
// unknown EntryID.
type errUnknownRelationRef struct {
	id EntryID
}

func (e *errUnknownRelationRef) Error() string {
	return "lexicon: relation references unknown entry: " + string(e.id)
}
