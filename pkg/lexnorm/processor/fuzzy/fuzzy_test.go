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

package fuzzy_test

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/internal/lexutil"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/fuzzy"
)

var _ lexnorm.Processor = (*fuzzy.Processor)(nil)

func newState(t *testing.T, text string) *lexnorm.State {
	t.Helper()
	s, err := lexnorm.NewState(context.Background(), text, nil, lexnorm.DefaultConfig())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	return s
}

func approximateVariant(text string, conf float64) lexicon.Variant {
	return lexicon.Variant{
		Text:       text,
		Kind:       lexicon.VariantApproximate,
		Confidence: conf,
		Source:     "test",
	}
}

func sampleLex() lexicon.Lexicon {
	return lexutil.NewMemLexicon([]lexicon.Entry{
		{
			ID:   "e-1",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				approximateVariant("周莉群", 0.95),
				approximateVariant("周里群", 0.85),
			},
		},
		{
			ID:   "e-2",
			Text: "田华",
			Variants: []lexicon.Variant{
				approximateVariant("田花", 0.7),
			},
		},
	}, "v1")
}

// ----------------------------------------------------------------------------
// Properties
// ----------------------------------------------------------------------------

func TestProcessor_Identity(t *testing.T) {
	p := fuzzy.New(sampleLex())
	if p.Name() != "fuzzy" {
		t.Errorf("Name = %q, want fuzzy", p.Name())
	}
	if p.Version() != "v1" {
		t.Errorf("Version = %q, want v1", p.Version())
	}
	if p.Certainty() != lexnorm.CertaintyMedium {
		t.Errorf("Certainty = %v, want CertaintyMedium", p.Certainty())
	}
}

func TestProcessor_NilLex_NoOp(t *testing.T) {
	p := fuzzy.New(nil)
	s := newState(t, "周莉群")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周莉群" {
		t.Errorf("nil Lexicon must be no-op, got %q", got)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("nil Lexicon must produce no Changes, got %d", len(s.Changes()))
	}
}

// ----------------------------------------------------------------------------
// Behavior: Apply / Suggest / Skip
// ----------------------------------------------------------------------------

func TestFuzzy_Apply(t *testing.T) {
	// 0.95 >= default 0.95 → Apply.
	s := newState(t, "周莉群")
	p := fuzzy.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周丽群" {
		t.Errorf("Text = %q, want %q (Apply)", got, "周丽群")
	}
	changes := s.Changes()
	if len(changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(changes))
	}
	if !changes[0].Applied {
		t.Error("confidence 0.95 >= autoApply 0.95 must be Applied")
	}
	if changes[0].EntryID != "e-1" {
		t.Errorf("EntryID = %q, want e-1", changes[0].EntryID)
	}
}

func TestFuzzy_Suggest(t *testing.T) {
	// 0.85: not Apply (0.95), but >= Suggest (0.65) → Suggest.
	s := newState(t, "周里群")
	p := fuzzy.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周里群" {
		t.Errorf("Text = %q, want unchanged (Suggest at 0.85)", got)
	}
	changes := s.Changes()
	if len(changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(changes))
	}
	if changes[0].Applied {
		t.Error("confidence 0.85 < autoApply 0.95 must NOT be Applied")
	}
}

func TestFuzzy_Skip(t *testing.T) {
	// 0.7: not Apply (0.95), >= Suggest (0.65) → Suggest, not Skip.
	// To get Skip, lower Suggest below 0.7.
	cfg := lexnorm.DefaultConfig()
	cfg.SuggestThreshold = 0.9 // above 0.7
	s, _ := lexnorm.NewState(context.Background(), "田花", nil, cfg)
	p := fuzzy.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("confidence 0.7 < suggest 0.9 must skip, got %d Changes", len(s.Changes()))
	}
}

func TestFuzzy_MultipleMatches(t *testing.T) {
	s := newState(t, "周莉群和周里群和田花")
	p := fuzzy.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	// 周莉群→周丽群 (0.95 Apply)
	// 周里群→周丽群 (0.85 Suggest, text unchanged)
	// 田花→田华 (0.7 Suggest, text unchanged)
	if got := s.Text(); got != "周丽群和周里群和田花" {
		t.Errorf("Text = %q, want %q (only first Applied)", got, "周丽群和周里群和田花")
	}
	if got := len(s.Changes()); got != 3 {
		t.Errorf("len(Changes) = %d, want 3 (one per match)", got)
	}
}

func TestFuzzy_PreservesCanonical(t *testing.T) {
	// Canonical "周丽群" must not be modified by fuzzy (its variants are typos).
	s := newState(t, "周丽群")
	p := fuzzy.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周丽群" {
		t.Errorf("canonical must be preserved, got %q", got)
	}
}

// ----------------------------------------------------------------------------
// Independence & determinism
// ----------------------------------------------------------------------------

func TestFuzzy_IndependentOfEngine(t *testing.T) {
	p := fuzzy.New(sampleLex())
	s := newState(t, "周莉群")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周丽群" {
		t.Errorf("Text = %q, want %q", got, "周丽群")
	}
}

func TestFuzzy_Deterministic(t *testing.T) {
	p := fuzzy.New(sampleLex())
	for i := 0; i < 5; i++ {
		s := newState(t, "周莉群和周里群")
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != "周丽群和周里群" {
			t.Errorf("run %d: Text = %q, want %q", i, got, "周丽群和周里群")
		}
		if got := len(s.Changes()); got != 2 {
			t.Errorf("run %d: len(Changes) = %d, want 2", i, got)
		}
	}
}

// ----------------------------------------------------------------------------
// Fuzz
// ----------------------------------------------------------------------------

func FuzzFuzzy(f *testing.F) {
	lex := sampleLex()
	f.Add("周莉群")
	f.Add("周莉群和周里群")
	f.Add("周丽群")
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		p := fuzzy.New(lex)
		s := newState(t, input)
		_ = p.Process(context.Background(), s)
	})
}

// ----------------------------------------------------------------------------
// Benchmark
// ----------------------------------------------------------------------------

func BenchmarkFuzzy(b *testing.B) {
	p := fuzzy.New(sampleLex())
	inputs := []string{
		"周莉群",
		"周莉群和周里群和田花",
		"周丽群",
		"周莉群周莉群周莉群周莉群周莉群",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := newStateB(b, inputs[i%len(inputs)])
		_ = p.Process(context.Background(), s)
	}
}

func newStateB(b *testing.B, text string) *lexnorm.State {
	b.Helper()
	s, err := lexnorm.NewState(context.Background(), text, nil, lexnorm.DefaultConfig())
	if err != nil {
		b.Fatalf("NewState: %v", err)
	}
	return s
}
