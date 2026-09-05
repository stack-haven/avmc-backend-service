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
	"errors"
	"sync"
	"testing"

	"github.com/stack-haven/lexnorm/lexicon"
)

// ----------------------------------------------------------------------------
// Builder
// ----------------------------------------------------------------------------

func TestBuilder_Basic(t *testing.T) {
	lex, err := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "e1", Text: "hello"}).
		Add(lexicon.Entry{ID: "e2", Text: "world"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if lex.Len() != 2 {
		t.Errorf("Len = %d, want 2", lex.Len())
	}
	if lex.Version() != "" {
		t.Errorf("Version = %q, want empty", lex.Version())
	}
}

func TestBuilder_WithVersion(t *testing.T) {
	lex, err := lexicon.NewBuilderWithVersion("v1.0").
		Add(lexicon.Entry{ID: "e1", Text: "hello"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if lex.Version() != "v1.0" {
		t.Errorf("Version = %q, want v1.0", lex.Version())
	}
}

func TestBuilder_DuplicateID_ReturnsError(t *testing.T) {
	_, err := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "e1", Text: "hello"}).
		Add(lexicon.Entry{ID: "e1", Text: "world"}).
		Build()
	if err == nil {
		t.Fatal("duplicate ID must return error")
	}
	if !stringsContains(err.Error(), "duplicate") {
		t.Errorf("error must mention 'duplicate', got %v", err)
	}
}

func TestBuilder_DuplicateText_ReturnsError(t *testing.T) {
	_, err := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "e1", Text: "hello"}).
		Add(lexicon.Entry{ID: "e2", Text: "hello"}).
		Build()
	if err == nil {
		t.Fatal("duplicate Text must return error")
	}
}

func TestBuilder_InvalidEntry_ReturnsError(t *testing.T) {
	_, err := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "", Text: "hello"}). // empty ID
		Build()
	if err == nil {
		t.Fatal("empty ID must return error")
	}
}

func TestBuilder_UnknownRelationRef_ReturnsError(t *testing.T) {
	_, err := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "e1", Text: "hello"}).
		AddRelation(lexicon.Relation{From: "e1", To: "unknown"}).
		Build()
	if err == nil {
		t.Fatal("unknown relation ref must return error")
	}
}

func TestBuilder_NilBuilder_ReturnsError(t *testing.T) {
	var b *lexicon.Builder
	_, err := b.Build()
	if err == nil {
		t.Fatal("nil Builder.Build must return error")
	}
}

func TestBuilder_NgramIndex(t *testing.T) {
	lex, err := lexicon.NewBuilder().
		WithNgram(2).
		Add(lexicon.Entry{ID: "e1", Text: "hello"}).
		Add(lexicon.Entry{ID: "e2", Text: "world"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	// Match() returns Aho-Corasick results over canonical + variants.
	matches := lex.Matcher().Match("hello world")
	if len(matches) != 2 {
		t.Errorf("Matcher.Match = %d matches, want 2", len(matches))
	}
}

// ----------------------------------------------------------------------------
// memLexicon (via Builder)
// ----------------------------------------------------------------------------

func TestLexicon_Entry(t *testing.T) {
	lex, _ := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "e1", Text: "hello"}).
		Build()

	e, ok := lex.Entry("e1")
	if !ok || e.Text != "hello" {
		t.Errorf("Entry(e1) = (%v, %v), want (hello, true)", e, ok)
	}

	_, ok = lex.Entry("unknown")
	if ok {
		t.Error("Entry(unknown) must return false")
	}
}

func TestLexicon_Lookup(t *testing.T) {
	lex, _ := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "e1", Text: "hello"}).
		Add(lexicon.Entry{ID: "e2", Text: "world"}).
		Build()

	e, ok := lex.Lookup("world")
	if !ok || e.ID != "e2" {
		t.Errorf("Lookup(world) = (%v, %v), want (e2, true)", e, ok)
	}

	_, ok = lex.Lookup("unknown")
	if ok {
		t.Error("Lookup(unknown) must return false")
	}
}

func TestLexicon_All_DeterministicOrder(t *testing.T) {
	// Insert out of order; All() must return by ID-sorted order.
	lex, _ := lexicon.NewBuilder().
		Add(
			lexicon.Entry{ID: "e3", Text: "three"},
			lexicon.Entry{ID: "e1", Text: "one"},
			lexicon.Entry{ID: "e2", Text: "two"},
		).
		Build()

	var ids []string
	lex.All(func(e lexicon.Entry) bool {
		ids = append(ids, string(e.ID))
		return true
	})

	want := []string{"e1", "e2", "e3"}
	if !sliceEqString(ids, want) {
		t.Errorf("All() order = %v, want %v (sorted by ID)", ids, want)
	}
}

func TestLexicon_All_EarlyStop(t *testing.T) {
	lex, _ := lexicon.NewBuilder().
		Add(
			lexicon.Entry{ID: "e1", Text: "one"},
			lexicon.Entry{ID: "e2", Text: "two"},
			lexicon.Entry{ID: "e3", Text: "three"},
		).
		Build()

	var ids []string
	lex.All(func(e lexicon.Entry) bool {
		ids = append(ids, string(e.ID))
		return len(ids) < 2 // stop after 2
	})

	if len(ids) != 2 || ids[0] != "e1" || ids[1] != "e2" {
		t.Errorf("All with early stop = %v, want [e1, e2]", ids)
	}
}

func TestLexicon_Relations(t *testing.T) {
	lex, _ := lexicon.NewBuilder().
		Add(
			lexicon.Entry{ID: "e1", Text: "hello"},
			lexicon.Entry{ID: "e2", Text: "world"},
		).
		AddRelation(
			lexicon.Relation{From: "e1", To: "e2", Kind: lexicon.VariantAlias, Weight: 0.8},
			lexicon.Relation{From: "e2", To: "e1", Kind: lexicon.VariantAlias, Weight: 0.5},
		).
		Build()

	rels := lex.Relations("hello")
	if len(rels) != 1 {
		t.Errorf("Relations(hello) = %d, want 1", len(rels))
	}
	if len(rels) > 0 && rels[0].To != "e2" {
		t.Errorf("relation To = %s, want e2", rels[0].To)
	}

	if r := lex.Relations("nonexistent"); len(r) != 0 {
		t.Errorf("Relations(nonexistent) = %v, want empty", r)
	}
}

func TestLexicon_Relations_DefensiveCopy(t *testing.T) {
	lex, _ := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "e1", Text: "hello"}, lexicon.Entry{ID: "e2", Text: "world"}).
		AddRelation(lexicon.Relation{From: "e1", To: "e2", Kind: lexicon.VariantAlias}).
		Build()

	rels := lex.Relations("hello")
	rels[0].Weight = 999.0 // mutate the returned slice

	rels2 := lex.Relations("hello")
	if rels2[0].Weight == 999.0 {
		t.Error("Relations() must return defensive copy")
	}
}

func TestLexicon_Matcher(t *testing.T) {
	lex, _ := lexicon.NewBuilder().
		Add(
			lexicon.Entry{
				ID:   "e1",
				Text: "canonical",
				Variants: []lexicon.Variant{
					{Text: "alt1", Kind: lexicon.VariantAlias, Confidence: 1.0},
					{Text: "alt2", Kind: lexicon.VariantCorrection, Confidence: 0.9},
				},
			},
		).
		Build()

	m := lex.Matcher()
	if m == nil {
		t.Fatal("Matcher must not be nil")
	}
	if m.PatternCount() != 3 {
		t.Errorf("PatternCount = %d, want 3 (canonical + 2 variants)", m.PatternCount())
	}

	matches := m.Match("alt2 here")
	if len(matches) != 1 {
		t.Errorf("Matcher.Match = %d, want 1", len(matches))
	}
}

func TestLexicon_ConcurrentRead(t *testing.T) {
	// Lexicon is safe for concurrent read access.
	lex, _ := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "e1", Text: "hello"}).
		Build()

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_, _ = lex.Entry("e1")
			_, _ = lex.Lookup("hello")
			lex.All(func(lexicon.Entry) bool { return true })
			_ = lex.Matcher().Match("hello")
			_ = lex.Len()
		}()
	}
	wg.Wait()
}

// ----------------------------------------------------------------------------
// SliceSource + Compose
// ----------------------------------------------------------------------------

func TestSliceSource_Basic(t *testing.T) {
	src := lexicon.NewSliceSource(
		[]lexicon.Entry{{ID: "e1", Text: "hello"}},
		nil,
		"v1",
	)
	if src.Version() != "v1" {
		t.Errorf("Version = %q, want v1", src.Version())
	}
	var entries []lexicon.Entry
	src.Entries(func(e lexicon.Entry) bool {
		entries = append(entries, e)
		return true
	})
	if len(entries) != 1 {
		t.Errorf("Entries yielded %d, want 1", len(entries))
	}
}

func TestCompose_Single(t *testing.T) {
	src := lexicon.NewSliceSource(
		[]lexicon.Entry{{ID: "e1", Text: "hello"}},
		nil,
		"v1",
	)
	lex, err := lexicon.Compose(src)
	if err != nil {
		t.Fatal(err)
	}
	if lex.Len() != 1 {
		t.Errorf("Len = %d, want 1", lex.Len())
	}
	if lex.Version() != "v1" {
		t.Errorf("Version = %q, want v1", lex.Version())
	}
}

func TestCompose_Multiple(t *testing.T) {
	src1 := lexicon.NewSliceSource(
		[]lexicon.Entry{{ID: "e1", Text: "hello"}},
		nil,
		"v1",
	)
	src2 := lexicon.NewSliceSource(
		[]lexicon.Entry{{ID: "e2", Text: "world"}},
		nil,
		"v2",
	)
	lex, err := lexicon.Compose(src1, src2)
	if err != nil {
		t.Fatal(err)
	}
	if lex.Len() != 2 {
		t.Errorf("Len = %d, want 2", lex.Len())
	}
	if lex.Version() != "v1+v2" {
		t.Errorf("Version = %q, want v1+v2", lex.Version())
	}
}

func TestCompose_DuplicateID_ReturnsError(t *testing.T) {
	src1 := lexicon.NewSliceSource(
		[]lexicon.Entry{{ID: "e1", Text: "hello"}},
		nil,
		"v1",
	)
	src2 := lexicon.NewSliceSource(
		[]lexicon.Entry{{ID: "e1", Text: "world"}},
		nil,
		"v2",
	)
	_, err := lexicon.Compose(src1, src2)
	if err == nil {
		t.Fatal("duplicate ID across sources must return error")
	}
}

func TestCompose_DuplicateText_ReturnsError(t *testing.T) {
	src1 := lexicon.NewSliceSource(
		[]lexicon.Entry{{ID: "e1", Text: "hello"}},
		nil,
		"v1",
	)
	src2 := lexicon.NewSliceSource(
		[]lexicon.Entry{{ID: "e2", Text: "hello"}},
		nil,
		"v2",
	)
	_, err := lexicon.Compose(src1, src2)
	if err == nil {
		t.Fatal("duplicate Text across sources must return error")
	}
}

func TestCompose_RelationsMerged(t *testing.T) {
	src1 := lexicon.NewSliceSource(
		[]lexicon.Entry{
			{ID: "e1", Text: "hello"},
			{ID: "e2", Text: "world"},
		},
		[]lexicon.Relation{{From: "e1", To: "e2", Kind: lexicon.VariantAlias}},
		"v1",
	)
	src2 := lexicon.NewSliceSource(nil, nil, "v2")
	lex, err := lexicon.Compose(src1, src2)
	if err != nil {
		t.Fatal(err)
	}
	rels := lex.Relations("hello")
	if len(rels) != 1 {
		t.Errorf("Relations(hello) = %d, want 1", len(rels))
	}
}

func TestCompose_NoSources_ReturnsError(t *testing.T) {
	_, err := lexicon.Compose()
	if err == nil {
		t.Fatal("Compose with no sources must return error")
	}
}

func TestCompose_NilSource_ReturnsError(t *testing.T) {
	src := lexicon.NewSliceSource([]lexicon.Entry{{ID: "e1", Text: "hello"}}, nil, "v1")
	_, err := lexicon.Compose(src, nil)
	if err == nil {
		t.Fatal("nil source must return error")
	}
}

// ----------------------------------------------------------------------------
// Test helpers
// ----------------------------------------------------------------------------

func sliceEqString(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringsContains(s, substr string) bool {
	return len(substr) <= len(s) && (len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Reference errors.Is usage to ensure import stays.
var _ = errors.Is
