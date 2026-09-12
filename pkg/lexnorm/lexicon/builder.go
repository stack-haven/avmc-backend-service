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
	"strings"
)

// Builder constructs a Lexicon from a set of Entries and Relations.
//
// # Construction Lifecycle
//
//  1. Create a Builder via NewBuilder (or NewBuilderWithVersion).
//  2. Call Add / AddRelation to register Entries and Relations.
//  3. (Optional) Enable n-gram or Pinyin indexing via WithNgram /
//     WithPinyin. These must be set before Build.
//  4. Call Build to produce an immutable Lexicon.
//
// Builder is NOT safe for concurrent use.
//
// # Indexing at Build Time
//
// Build performs all indexing in a single pass:
//
//   - ID map (O(1) Entry lookup by ID)
//   - Text map (O(1) Lookup by canonical Text)
//   - Aho-Corasick automaton over canonical Text + Variants
//   - n-gram inverted index (if enabled)
//   - Pinyin inverted index (if enabled)
//
// After Build, the resulting Lexicon is safe for concurrent read.
type Builder struct {
	cfg memLexiconConfig
}

// NewBuilder creates an empty Builder with no version string.
func NewBuilder() *Builder {
	return &Builder{}
}

// NewBuilderWithVersion creates an empty Builder with the given version.
//
// The version is recorded in the resulting Lexicon's Version() and
// surfaces in Result.RuntimeInfo for audit.
func NewBuilderWithVersion(version string) *Builder {
	b := NewBuilder()
	b.cfg.version = version
	return b
}

// Add appends Entries to the Builder.
//
// Returns the Builder for chaining.
func (b *Builder) Add(entries ...Entry) *Builder {
	b.cfg.entries = append(b.cfg.entries, entries...)
	return b
}

// AddRelation appends Relations to the Builder.
//
// Returns the Builder for chaining. Relations reference EntryIDs;
// both endpoints must be present in the Lexicon at Build time.
func (b *Builder) AddRelation(relations ...Relation) *Builder {
	b.cfg.relations = append(b.cfg.relations, relations...)
	return b
}

// WithNgram enables n-gram indexing with the given n-gram size.
//
// Typical values: 2 (bigram) or 3 (trigram). n < 1 is treated as 1.
// Must be called before Build.
//
// WithNgram(n) where n <= 0 disables n-gram indexing.
func (b *Builder) WithNgram(n int) *Builder {
	b.cfg.ngramSize = n
	return b
}

// WithPinyin enables Pinyin indexing using the provided converter.
//
// The converter is called once per Entry.Text during Build. Must be
// called before Build.
func (b *Builder) WithPinyin(converter PinyinConverter) *Builder {
	b.cfg.usePinyin = true
	b.cfg.pinyinConverter = converter
	return b
}

// Build constructs the immutable Lexicon.
//
// Returns ErrConflict-wrapped errors for:
//   - Invalid Entry (empty ID)
//   - Duplicate Entry ID
//   - Duplicate canonical Text
//   - Relation referencing unknown EntryID
//
// The returned Lexicon is safe for concurrent read access.
func (b *Builder) Build() (Lexicon, error) {
	if b == nil {
		return nil, errNilBuilder
	}
	return buildMemLexicon(b.cfg)
}

// Validate checks the accumulated Entries against the lexicon hygiene
// rules WITHOUT building. Build() stays intentionally lenient (backward
// compatibility); callers that want data-quality enforcement invoke
// Validate before Build and fail fast on the returned error.
//
// Validate rejects:
//
//  1. Entry without canonical Text (empty Text).
//  2. Variant that equals or is a substring of its own canonical Text —
//     matching it would rewrite the canonical into itself with the
//     remainder duplicated (observed corruption: "万康盛鼎" →
//     "万康盛鼎集团" produced "万康盛鼎集团集团").
//  3. Variant text that equals ANOTHER entry's canonical Text — alias
//     matching would destroy that canonical name wherever it occurs.
//  4. The same variant text declared on multiple entries — resolution
//     would be arbitrary (first-by-ID wins) and silently ambiguous.
//
// These checks can be run both on Builder input and on already-decoded
// entry sets (see example 09 for a stress-test integration).
func (b *Builder) Validate() error {
	if b == nil {
		return errNilBuilder
	}

	canonicals := make(map[string]EntryID, len(b.cfg.entries))
	for _, e := range b.cfg.entries {
		if !e.IsValid() {
			return &errInvalidEntry{e: e}
		}
		if e.Text == "" {
			return fmt.Errorf("lexicon: entry %q has empty canonical text: %w", e.ID, ErrConflict)
		}
		canonicals[e.Text] = e.ID
	}

	type variantOrigin struct {
		text    string
		entryID EntryID
	}
	seenVariants := make(map[string]EntryID)

	for _, e := range b.cfg.entries {
		for _, v := range e.Variants {
			if !v.IsValid() {
				continue // empty variants are dropped at Build; not an error
			}
			// Rule 2: variant equals or is inside its own canonical.
			if v.Text == e.Text {
				return fmt.Errorf(
					"lexicon: entry %q: variant %q equals its canonical text: %w",
					e.ID, v.Text, ErrConflict)
			}
			if strings.Contains(e.Text, v.Text) {
				return fmt.Errorf(
					"lexicon: entry %q: variant %q is a substring of canonical %q: %w",
					e.ID, v.Text, e.Text, ErrConflict)
			}
			// Rule 3: variant collides with another entry's canonical.
			if owner, ok := canonicals[v.Text]; ok && e.ID != owner {
				return fmt.Errorf(
					"lexicon: entry %q: variant %q collides with canonical text of entry %q: %w",
					e.ID, v.Text, string(owner), ErrConflict)
			}
			// Rule 4: duplicate variant across entries.
			if prev, ok := seenVariants[v.Text]; ok && prev != e.ID {
				return fmt.Errorf(
					"lexicon: variant %q declared on both entry %q and entry %q: %w",
					v.Text, string(prev), e.ID, ErrConflict)
			}
			seenVariants[v.Text] = e.ID
		}
	}
	return nil
}

// ErrConflict is the sentinel wrapped (via fmt.Errorf %w) by every
// validation failure reported by Builder.Validate and Build. Use
// errors.Is to detect it.
var ErrConflict = errors.New("lexicon: conflict")

// errNilBuilder is returned when Build is called on a nil Builder.
var errNilBuilder = &errBuilderNil{}

type errBuilderNil struct{}

func (e *errBuilderNil) Error() string {
	return "lexicon: nil Builder"
}
