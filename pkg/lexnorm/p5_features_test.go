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
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/ctxproc"
	"github.com/stack-haven/lexnorm/processor/normalize"
)

// --- Pipeline composition helpers (spec Step 5) ---

func names(ps []lexnorm.Processor) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Name())
	}
	return out
}

func TestPipelineHelpers(t *testing.T) {
	base := lexnorm.NewPipeline(normalize.New(), ctxproc.New())

	// Replace preserves position.
	replaced, err := lexnorm.ReplaceProcessor(base, "context", boomProcessor{})
	if err != nil {
		t.Fatal(err)
	}
	got := names(replaced.Processors())
	if got[0] != "normalize" || got[1] != "panic" {
		t.Errorf("replace: %v", got)
	}
	// Original untouched.
	if n := names(base.Processors()); n[1] != "context" {
		t.Errorf("base mutated: %v", n)
	}

	// Remove preserves order.
	removed, err := lexnorm.RemoveProcessor(base, "normalize")
	if err != nil {
		t.Fatal(err)
	}
	if n := names(removed.Processors()); len(n) != 1 || n[0] != "context" {
		t.Errorf("remove: %v", n)
	}

	// Append.
	appended := lexnorm.AppendProcessor(base, boomProcessor{})
	if n := names(appended.Processors()); len(n) != 3 || n[2] != "panic" {
		t.Errorf("append: %v", n)
	}

	// Unknown name → error wrapping ErrInvalidConfig.
	if _, err := lexnorm.RemoveProcessor(base, "nope"); !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("unknown name: err = %v", err)
	}
}

// --- Contextual v1 ---

func TestContextual_UniqueCandidate_Suggests(t *testing.T) {
	lex, _ := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e1", Text: "龚建军",
			Variants: []lexicon.Variant{{Text: "建军工", Kind: lexicon.VariantContextual, Confidence: 0.8}}},
	).Build()
	p := ctxproc.NewWithLexicon(lex)
	st, _ := lexnorm.NewState(context.Background(), "让建军工来评审", lex, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if st.Text() != "让建军工来评审" {
		t.Errorf("contextual v1 must never apply: %q", st.Text())
	}
	found := false
	for _, c := range st.Changes() {
		if c.To == "龚建军" && !c.Applied {
			found = true
		}
	}
	if !found {
		t.Errorf("expected suggestion 龚建军, changes = %+v", st.Changes())
	}
}

func TestContextual_Ambiguous_Skips(t *testing.T) {
	// Same surface form mapped to two people: without a scorer, skip.
	lex, _ := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e1", Text: "龚千友",
			Variants: []lexicon.Variant{{Text: "龚工", Kind: lexicon.VariantContextual, Confidence: 0.8}}},
		lexicon.Entry{ID: "e2", Text: "龚建军",
			Variants: []lexicon.Variant{{Text: "龚工", Kind: lexicon.VariantContextual, Confidence: 0.8}}},
	).Build()
	p := ctxproc.NewWithLexicon(lex)
	st, _ := lexnorm.NewState(context.Background(), "龚工来了", lex, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if len(st.Changes()) != 0 {
		t.Errorf("ambiguous candidate must skip, changes = %+v", st.Changes())
	}

	// With a scorer pruning to one candidate → suggest.
	p2 := ctxproc.NewWithLexicon(lex).WithScorer(func(_ context.Context, _ string, _, _ int, cands []ctxproc.Candidate) []ctxproc.Candidate {
		out := cands[:0]
		for _, c := range cands {
			if c.EntryID == "e2" {
				out = append(out, c)
			}
		}
		return out
	})
	st2, _ := lexnorm.NewState(context.Background(), "龚工来了", lex, lexnorm.DefaultConfig())
	_ = p2.Process(context.Background(), st2)
	if len(st2.Changes()) != 1 || st2.Changes()[0].To != "龚建军" {
		t.Errorf("scorer should resolve to 龚建军, changes = %+v", st2.Changes())
	}
}

// --- Builder.Validate (lexicon hygiene) ---

func TestBuilderValidate(t *testing.T) {
	cases := []struct {
		name    string
		entries []lexicon.Entry
	}{
		{"empty canonical", []lexicon.Entry{{ID: "e1", Text: ""}}},
		{"variant equals canonical", []lexicon.Entry{{
			ID: "e1", Text: "田华",
			Variants: []lexicon.Variant{{Text: "田华", Kind: lexicon.VariantAlias}},
		}}},
		{"variant substring of own canonical", []lexicon.Entry{{
			ID: "e1", Text: "万康盛鼎集团",
			Variants: []lexicon.Variant{{Text: "万康盛鼎", Kind: lexicon.VariantAlias}},
		}}},
		{"variant collides with other canonical", []lexicon.Entry{
			{ID: "e1", Text: "李四",
				Variants: []lexicon.Variant{{Text: "李四光", Kind: lexicon.VariantAlias}}},
			{ID: "e2", Text: "李四光"},
		}},
		{"duplicate variant across entries", []lexicon.Entry{
			{ID: "e1", Text: "田华",
				Variants: []lexicon.Variant{{Text: "田工", Kind: lexicon.VariantAlias}}},
			{ID: "e2", Text: "田工伟",
				Variants: []lexicon.Variant{{Text: "田工", Kind: lexicon.VariantAlias}}},
		}},
	}
	for _, tc := range cases {
		b := lexicon.NewBuilder()
		b.Add(tc.entries...)
		if err := b.Validate(); !errors.Is(err, lexicon.ErrConflict) {
			t.Errorf("%s: Validate err = %v, want ErrConflict", tc.name, err)
		}
	}

	// Clean lexicon passes.
	b := lexicon.NewBuilder()
	b.Add(lexicon.Entry{ID: "e1", Text: "田华",
		Variants: []lexicon.Variant{{Text: "田工", Kind: lexicon.VariantAlias, Confidence: 0.9}}})
	if err := b.Validate(); err != nil {
		t.Errorf("clean lexicon: Validate err = %v", err)
	}

	// Build stays lenient for backward compatibility: the substring
	// case still builds (callers opt into strictness via Validate).
	b2 := lexicon.NewBuilder()
	b2.Add(lexicon.Entry{ID: "e1", Text: "万康盛鼎集团",
		Variants: []lexicon.Variant{{Text: "万康盛鼎", Kind: lexicon.VariantAlias}}})
	if _, err := b2.Build(); err != nil {
		t.Errorf("Build must remain lenient: %v", err)
	}
}
