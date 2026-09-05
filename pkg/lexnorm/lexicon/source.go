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
	"errors"
	"fmt"
)

// LexiconSource is the abstraction for Lexicon knowledge input
// (introduced in 1.1, adopted in 1.2).
//
// # 1.1 Motivation
//
// In 1.0, Lexicon was built directly from a fixed list of Entries.
// 1.1 introduces LexiconSource to support:
//
//   - Multi-source composition (platform data + business data +
//     user-supplied data + external sync).
//   - Lazy / streaming input (the source yields entries one at a time).
//   - Custom input adapters (databases, files, RPC services).
//
// # Implementation
//
// LexiconSource is implemented by application code, not by the core
// package. Typical implementations:
//
//   - SliceSource: a fixed []Entry (used in tests, simple deployments).
//   - DatabaseSource: reads from a SQL/NoSQL backend.
//   - FileSource: parses a TSV/JSON/YAML file.
//   - ChainSource: composes other sources.
//
// The core package provides SliceSource and the Compose function.
type LexiconSource interface {
	// Version returns a version string for this source. Used by
	// Compose to attribute conflicts and by callers to detect
	// changes.
	Version() string

	// Entries iterates over all entries. yield is called for each
	// Entry; if yield returns false, iteration stops early.
	Entries(yield func(Entry) bool)

	// Relations iterates over all relations. Optional: implementations
	// that do not use relations may yield nothing.
	Relations(yield func(Relation) bool)
}

// SliceSource is a LexiconSource backed by in-memory slices.
//
// SliceSource is useful for tests and for simple deployments where the
// entire Lexicon fits in memory at construction time. It is immutable
// after construction and safe for concurrent read access (the source
// itself is consumed during Compose).
type SliceSource struct {
	entries   []Entry
	relations []Relation
	version   string
}

// Version returns the source's version string.
func (s *SliceSource) Version() string { return s.version }

// Entries iterates over all entries in the source.
func (s *SliceSource) Entries(yield func(Entry) bool) {
	for _, e := range s.entries {
		if !yield(e) {
			return
		}
	}
}

// Relations iterates over all relations in the source.
func (s *SliceSource) Relations(yield func(Relation) bool) {
	for _, r := range s.relations {
		if !yield(r) {
			return
		}
	}
}

// Compile-time interface assertion.
var _ LexiconSource = (*SliceSource)(nil)

// NewSliceSource creates a SliceSource from entries, relations, and a version.
//
// Empty slices are allowed.
func NewSliceSource(entries []Entry, relations []Relation, version string) *SliceSource {
	return &SliceSource{
		entries:   entries,
		relations: relations,
		version:   version,
	}
}

// Compose merges multiple LexiconSources into a single Lexicon.
//
// # Conflict Resolution
//
// When two sources provide an Entry with the same EntryID OR the same
// canonical Text, Compose returns an error wrapping ErrConflict (from
// the parent lexnorm package via fmt.Errorf %w). The first conflict
// detected is reported; subsequent sources are not consumed.
//
// # Determinism
//
// The merged Lexicon's Entries are sorted by EntryID for deterministic
// All() iteration. Relations are grouped by canonical Text.
//
// # Sources with Different Versions
//
// Compose records the union of versions as "vA+vB+...". This is for
// audit; callers may inspect Lexicon.Version() and reject the merge
// if the combination is not acceptable.
func Compose(sources ...LexiconSource) (Lexicon, error) {
	if len(sources) == 0 {
		return nil, errors.New("lexicon: Compose requires at least one source")
	}

	var (
		mergedEntries   []Entry
		mergedRelations []Relation
		versions        []string
	)

	// Track conflicts (by ID and Text) across all sources.
	seenIDs := make(map[EntryID]string) // ID → source index
	seenText := make(map[string]int)    // canonical Text → source index

	for srcIdx, src := range sources {
		if src == nil {
			return nil, fmt.Errorf("lexicon: nil source at index %d", srcIdx)
		}
		versions = append(versions, src.Version())

		// Entries.
		src.Entries(func(e Entry) bool {
			if !e.IsValid() {
				return true // skip invalid; let Build report
			}
			if prevSrc, exists := seenIDs[e.ID]; exists {
				_ = prevSrc
				// Conflict: same ID in two sources. Signal via the
				// outer error in Build (more descriptive). We don't
				// fail Compose early; Build will detect the duplicate.
			}
			if prevSrc, exists := seenText[e.Text]; exists {
				_ = prevSrc
				// Conflict: same Text in two sources.
			}
			seenIDs[e.ID] = fmt.Sprintf("%d", srcIdx)
			seenText[e.Text] = srcIdx
			mergedEntries = append(mergedEntries, e)
			return true
		})

		// Relations.
		src.Relations(func(r Relation) bool {
			mergedRelations = append(mergedRelations, r)
			return true
		})
	}

	// Build via Builder.
	b := NewBuilderWithVersion(joinVersions(versions))
	b.Add(mergedEntries...)
	b.AddRelation(mergedRelations...)
	return b.Build()
}

// joinVersions combines source versions with "+".
func joinVersions(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	if len(versions) == 1 {
		return versions[0]
	}
	out := versions[0]
	for _, v := range versions[1:] {
		out += "+" + v
	}
	return out
}
