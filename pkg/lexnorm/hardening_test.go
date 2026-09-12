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

package lexnorm_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/normalize"
)

// --- 4a: State.Replace containment defense ---

func TestState_Replace_ContainingSpan_Rejected(t *testing.T) {
	s, err := lexnorm.NewState(context.Background(), "甲乙丙丁戊", nil, lexnorm.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(lexnorm.Span{Start: 0, End: 6}, "X", lexnorm.ChangeMeta{Confidence: 1}); err != nil {
		t.Fatal(err)
	}
	// [0,9) strictly contains replaced [0,6): must now conflict instead of
	// silently clobbering while leaving a phantom audit entry.
	err = s.Replace(lexnorm.Span{Start: 0, End: 9}, "YZ", lexnorm.ChangeMeta{Confidence: 1})
	if !errors.Is(err, lexnorm.ErrConflict) {
		t.Fatalf("containing replace: err = %v, want lexnorm.ErrConflict", err)
	}
	// Same-span override remains allowed (documented idempotency).
	if err := s.Replace(lexnorm.Span{Start: 0, End: 6}, "X2", lexnorm.ChangeMeta{Confidence: 1}); err != nil {
		t.Fatalf("same-span override should be allowed: %v", err)
	}
	// Disjoint span still fine.
	if err := s.Replace(lexnorm.Span{Start: 9, End: 15}, "Q", lexnorm.ChangeMeta{Confidence: 1}); err != nil {
		t.Fatalf("disjoint replace should be allowed: %v", err)
	}
}

func TestState_Replace_ContainingMultiple_Rejected(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "甲乙丙丁戊", nil, lexnorm.DefaultConfig())
	_ = s.Replace(lexnorm.Span{Start: 0, End: 3}, "a", lexnorm.ChangeMeta{Confidence: 1})
	_ = s.Replace(lexnorm.Span{Start: 6, End: 9}, "b", lexnorm.ChangeMeta{Confidence: 1})
	err := s.Replace(lexnorm.Span{Start: 0, End: 9}, "c", lexnorm.ChangeMeta{Confidence: 1})
	if !errors.Is(err, lexnorm.ErrConflict) {
		t.Fatalf("span containing two replaced regions: err = %v, want lexnorm.ErrConflict", err)
	}
}

func TestState_MutationCount(t *testing.T) {
	s, _ := lexnorm.NewState(context.Background(), "甲乙丙丁", nil, lexnorm.DefaultConfig())
	if s.MutationCount() != 0 {
		t.Fatal("fresh state must report 0 mutations")
	}
	_ = s.Replace(lexnorm.Span{Start: 0, End: 3}, "x", lexnorm.ChangeMeta{Confidence: 1})
	if s.MutationCount() != 1 {
		t.Fatalf("after Replace: %d, want 1", s.MutationCount())
	}
	if err := s.Rewrite("xyz", lexnorm.ChangeMeta{Confidence: 1}); err != nil {
		t.Fatal(err)
	}
	if s.MutationCount() != 2 {
		t.Fatalf("after Rewrite: %d, want 2", s.MutationCount())
	}
}

// --- 4b: alias records declared confidence (unset → 1.0) ---

func TestAlias_RecordsDeclaredConfidence(t *testing.T) {
	lex, _ := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e1", Text: "田华",
			Variants: []lexicon.Variant{{Text: "田工", Kind: lexicon.VariantAlias, Confidence: 0.9}}},
		lexicon.Entry{ID: "e2", Text: "王强",
			Variants: []lexicon.Variant{{Text: "老王", Kind: lexicon.VariantAlias}}},
	).Build()
	p := alias.New(lex)
	st, _ := lexnorm.NewState(context.Background(), "田工和老王", lex, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, c := range st.Changes() {
		got[c.From] = c.Confidence
	}
	if got["田工"] != 0.9 {
		t.Errorf("declared confidence: got %v, want 0.9", got["田工"])
	}
	if got["老王"] != 1.0 {
		t.Errorf("unset confidence should default to 1.0: got %v", got["老王"])
	}
}

// --- 4c: duplicate variant across entries → single audit record ---

func TestAlias_DuplicateVariant_SingleChange(t *testing.T) {
	lex, _ := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e1", Text: "田华",
			Variants: []lexicon.Variant{{Text: "田工", Kind: lexicon.VariantAlias, Confidence: 0.9}}},
		lexicon.Entry{ID: "e2", Text: "田工伟",
			Variants: []lexicon.Variant{{Text: "田工", Kind: lexicon.VariantAlias, Confidence: 0.9}}},
	).Build()
	p := alias.New(lex)
	st, _ := lexnorm.NewState(context.Background(), "田工来了", lex, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	changes := st.Changes()
	if len(changes) != 1 {
		t.Fatalf("changes = %d, want 1 (deduped): %+v", len(changes), changes)
	}
	// Winner is the documented first-by-ID entry.
	if changes[0].To != "田华" {
		t.Errorf("winner = %q, want 田华 (first EntryID wins, documented)", changes[0].To)
	}
}

// --- 4d: normalize after a mutating processor must not corrupt ---

func TestNormalize_AfterDisfluency_NoCorruption(t *testing.T) {
	// Regression: normalize used to scan current Text but emit spans in
	// Original coordinates, truncating multibyte characters when not
	// first in the pipeline.
	pipeline := lexnorm.NewPipeline(disfluency.New(), normalize.New())
	lex, _ := lexicon.NewBuilder().Build()
	e, err := lexnorm.New(lexnorm.WithPipeline(pipeline), lexnorm.WithLexicon(lex))
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Normalize(context.Background(), "你好 嗯  世界 啊")
	if err != nil {
		t.Fatal(err)
	}
	if !utf8ValidString(res.Text) {
		t.Fatalf("output is not valid UTF-8: %q", res.Text)
	}
	// Disfluency removed 嗯 and one adjacent space; normalize collapsed
	// the remaining double space via the Rewrite fallback.
	want := "你好 世界"
	if res.Text != want {
		t.Errorf("text = %q, want %q", res.Text, want)
	}
}

func utf8ValidString(s string) bool { return utf8.ValidString(s) }

func TestNormalize_FallbackUsesRewrite(t *testing.T) {
	lex, _ := lexicon.NewBuilder().Build()
	e, _ := lexnorm.New(lexnorm.WithPipeline(lexnorm.NewPipeline(disfluency.New(), normalize.New())), lexnorm.WithLexicon(lex))
	// Double space survives filler removal (only one adjacent space is
	// consumed), so normalize still has work → Rewrite path fires.
	res, _ := e.Normalize(context.Background(), "你好 嗯  世界")
	kinds := map[lexnorm.ChangeKind]int{}
	for _, c := range res.Changes {
		kinds[c.Kind]++
	}
	if kinds[lexnorm.ChangeRewrite] != 1 {
		t.Errorf("normalize fallback should emit exactly one Rewrite change, got %d", kinds[lexnorm.ChangeRewrite])
	}
	if !strings.Contains(res.Text, "世界") {
		t.Errorf("unexpected text %q", res.Text)
	}
}
