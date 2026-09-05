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

// Package lexutil provides test helpers for ark-lexnorm tests.
//
// This package is for internal use only. Do NOT import it from production code.
//
// # Test Helper Strategy
//
// We can't use a real Lexicon (it's a complex data structure with
// Aho-Corasick, etc.) in every test. Instead, tests use the small
// helpers here to construct minimal Lexicon instances that satisfy
// the lexicon.Lexicon interface for the purpose of Engine / Pipeline
// testing.
package lexutil

import (
	"sort"
	"strings"
	"sync"

	"github.com/stack-haven/lexnorm/lexicon"
)

// MemLexicon is a minimal in-memory Lexicon for tests.
//
// It supports only exact Text lookup (Lookup) and EntryID (Entry); All,
// Relations, Matcher return empty results. The Version field is set
// at construction.
//
// MemLexicon is safe for concurrent read access.
type MemLexicon struct {
	mu      sync.RWMutex
	byID    map[lexicon.EntryID]lexicon.Entry
	byText  map[string]lexicon.Entry
	ordered []lexicon.Entry // sorted by ID for deterministic All()
	version string
}

// NewMemLexicon creates a MemLexicon from the given entries and a
// version string.
func NewMemLexicon(entries []lexicon.Entry, version string) *MemLexicon {
	l := &MemLexicon{
		byID:    make(map[lexicon.EntryID]lexicon.Entry, len(entries)),
		byText:  make(map[string]lexicon.Entry, len(entries)),
		version: version,
	}
	for _, e := range entries {
		l.byID[e.ID] = e
		l.byText[e.Text] = e
		l.ordered = append(l.ordered, e)
	}
	// Sort by ID for deterministic iteration order.
	sort.Slice(l.ordered, func(i, j int) bool {
		return l.ordered[i].ID < l.ordered[j].ID
	})
	return l
}

// Entry implements lexicon.Lexicon.
func (l *MemLexicon) Entry(id lexicon.EntryID) (lexicon.Entry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	e, ok := l.byID[id]
	return e, ok
}

// Lookup implements lexicon.Lexicon.
func (l *MemLexicon) Lookup(text string) (lexicon.Entry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	e, ok := l.byText[text]
	return e, ok
}

// Relations implements lexicon.Lexicon.
func (l *MemLexicon) Relations(_ string) []lexicon.Relation {
	return nil
}

// All implements lexicon.Lexicon.
func (l *MemLexicon) All(yield func(lexicon.Entry) bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, e := range l.ordered {
		if !yield(e) {
			return
		}
	}
}

// Matcher implements lexicon.Lexicon.
func (l *MemLexicon) Matcher() *lexicon.Matcher {
	return nil
}

// Len implements lexicon.Lexicon.
func (l *MemLexicon) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.ordered)
}

// Version implements lexicon.Lexicon.
func (l *MemLexicon) Version() string {
	return l.version
}

// SimpleEntry is a convenience constructor for tests.
func SimpleEntry(id, text string, variants ...lexicon.Variant) lexicon.Entry {
	return lexicon.Entry{
		ID:       lexicon.EntryID(id),
		Text:     text,
		Variants: variants,
	}
}

// AliasVariant is a convenience for tests.
func AliasVariant(text string) lexicon.Variant {
	return lexicon.Variant{
		Text:       text,
		Kind:       lexicon.VariantAlias,
		Confidence: 1.0,
		Source:     "test",
	}
}

// Compile-time interface assertion.
var _ lexicon.Lexicon = (*MemLexicon)(nil)

// Avoid unused import warnings for strings in future use.
var _ = strings.TrimSpace
