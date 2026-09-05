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

// errNilBuilder is returned when Build is called on a nil Builder.
var errNilBuilder = &errBuilderNil{}

type errBuilderNil struct{}

func (e *errBuilderNil) Error() string {
	return "lexicon: nil Builder"
}
