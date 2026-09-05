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

package pinyin_test

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/internal/lexutil"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/pinyin"
)

var _ lexnorm.Processor = (*pinyin.Processor)(nil)

func newState(t *testing.T, text string) *lexnorm.State {
	t.Helper()
	s, err := lexnorm.NewState(context.Background(), text, nil, lexnorm.DefaultConfig())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	return s
}

// mapConverter maps specific characters to pinyin forms.
type mapConverter struct {
	m map[string][]string
}

func (c *mapConverter) ToPinyin(text string) []string {
	if c == nil {
		return nil
	}
	return c.m[text]
}

func sampleLex() lexicon.Lexicon {
	return lexutil.NewMemLexicon([]lexicon.Entry{
		{
			ID:   "ting",
			Text: "厅",
			Variants: []lexicon.Variant{
				{Text: "听", Kind: lexicon.VariantHomophone, Confidence: 0.8},
			},
		},
		{
			ID:   "gong",
			Text: "工",
			Variants: []lexicon.Variant{
				{Text: "公", Kind: lexicon.VariantHomophone, Confidence: 0.7},
				{Text: "功", Kind: lexicon.VariantHomophone, Confidence: 0.7},
			},
		},
	}, "v1")
}

func sampleConverter() *mapConverter {
	return &mapConverter{
		m: map[string][]string{
			"听": {"ting"},
			"公": {"gong"},
			"功": {"gong"},
			"厅": {"ting"},
			"工": {"gong"},
		},
	}
}

// ----------------------------------------------------------------------------
// Properties
// ----------------------------------------------------------------------------

func TestProcessor_Identity(t *testing.T) {
	p := pinyin.New(sampleLex(), sampleConverter())
	if p.Name() != "pinyin" {
		t.Errorf("Name = %q, want pinyin", p.Name())
	}
	if p.Version() != "v1" {
		t.Errorf("Version = %q, want v1", p.Version())
	}
	if p.Certainty() != lexnorm.CertaintyMedium {
		t.Errorf("Certainty = %v, want CertaintyMedium", p.Certainty())
	}
}

func TestProcessor_NilLex_NoOp(t *testing.T) {
	p := pinyin.New(nil, sampleConverter())
	s := newState(t, "听")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "听" {
		t.Errorf("nil Lexicon must be no-op, got Text = %q", got)
	}
}

func TestProcessor_NilConverter_NoOp(t *testing.T) {
	p := pinyin.New(sampleLex(), nil)
	s := newState(t, "听")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "听" {
		t.Errorf("nil Converter must be no-op, got Text = %q", got)
	}
}

// ----------------------------------------------------------------------------
// Behavior: Apply / Suggest / Skip
// ----------------------------------------------------------------------------

func TestPinyin_Apply_OnHomophone(t *testing.T) {
	// "听" pinyin "ting" matches "厅" (confidence 0.8).
	// Default AutoApplyThreshold = 0.95. With 0.8 < 0.95, this is Suggest.
	s := newState(t, "听")
	p := pinyin.New(sampleLex(), sampleConverter())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	// Text unchanged (Suggestion, not Apply).
	if got := s.Text(); got != "听" {
		t.Errorf("Text = %q, want unchanged (Suggest at 0.8 < 0.95)", got)
	}
	changes := s.Changes()
	if len(changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(changes))
	}
	if changes[0].Applied {
		t.Error("confidence 0.8 < autoApply 0.95 must NOT be Applied")
	}
	if changes[0].From != "听" || changes[0].To != "厅" {
		t.Errorf("From/To = %q/%q, want 听/厅", changes[0].From, changes[0].To)
	}
	if changes[0].Source != "pinyin" {
		t.Errorf("Source = %q, want pinyin", changes[0].Source)
	}
}

func TestPinyin_Apply_WithLowThreshold(t *testing.T) {
	// Set AutoApply to 0.7 and Suggest to 0.3 (Suggest < AutoApply required).
	cfg := lexnorm.DefaultConfig()
	cfg.AutoApplyThreshold = 0.7
	cfg.SuggestThreshold = 0.3

	s, err := lexnorm.NewState(context.Background(), "听", nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	p := pinyin.New(sampleLex(), sampleConverter())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "厅" {
		t.Errorf("Text = %q, want %q (Apply at conf 0.8 > threshold 0.7)", got, "厅")
	}
}

func TestPinyin_Suggest_BelowAutoAboveSuggest(t *testing.T) {
	cfg := lexnorm.DefaultConfig()
	cfg.AutoApplyThreshold = 0.9
	cfg.SuggestThreshold = 0.5

	s, _ := lexnorm.NewState(context.Background(), "听", nil, cfg)
	p := pinyin.New(sampleLex(), sampleConverter())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "听" {
		t.Errorf("Text = %q, want unchanged (Suggest)", got)
	}
	changes := s.Changes()
	if len(changes) != 1 || changes[0].Applied {
		t.Errorf("Suggest must record Applied=false Change, got %+v", changes)
	}
}

func TestPinyin_Skip_BelowSuggest(t *testing.T) {
	// "公" pinyin "gong" matches "工" with confidence 0.7.
	// With suggest threshold 0.9, 0.7 is skipped.
	cfg := lexnorm.DefaultConfig()
	cfg.AutoApplyThreshold = 0.95
	cfg.SuggestThreshold = 0.9

	s, _ := lexnorm.NewState(context.Background(), "公", nil, cfg)
	p := pinyin.New(sampleLex(), sampleConverter())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("confidence 0.7 < suggest 0.9 must skip, got %d Changes", len(s.Changes()))
	}
}

func TestPinyin_PreservesCanonical(t *testing.T) {
	// "厅" is canonical; its pinyin "ting" maps to itself.
	s := newState(t, "厅")
	p := pinyin.New(sampleLex(), sampleConverter())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "厅" {
		t.Errorf("canonical Text should be preserved (no actual change), got %q", got)
	}
}

func TestPinyin_SkipsNonCJK(t *testing.T) {
	// Latin characters are skipped (not CJK).
	s := newState(t, "Hello World 123")
	p := pinyin.New(sampleLex(), sampleConverter())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "Hello World 123" {
		t.Errorf("Latin input must be unchanged, got %q", got)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("non-CJK input must produce no Changes, got %d", len(s.Changes()))
	}
}

// ----------------------------------------------------------------------------
// Independence & determinism
// ----------------------------------------------------------------------------

func TestPinyin_IndependentOfEngine(t *testing.T) {
	p := pinyin.New(sampleLex(), sampleConverter())
	s := newState(t, "听")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	// At default thresholds, 0.8 < 0.95, so Suggest.
	if got := s.Text(); got != "听" {
		t.Errorf("Text = %q, want unchanged", got)
	}
	if len(s.Changes()) != 1 {
		t.Errorf("len(Changes) = %d, want 1", len(s.Changes()))
	}
}

func TestPinyin_Deterministic(t *testing.T) {
	p := pinyin.New(sampleLex(), sampleConverter())
	for i := 0; i < 5; i++ {
		s := newState(t, "听公厅功")
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := len(s.Changes()); got != 4 {
			t.Errorf("run %d: len(Changes) = %d, want 4 (one per character)", i, got)
		}
	}
}

// ----------------------------------------------------------------------------
// Fuzz
// ----------------------------------------------------------------------------

func FuzzPinyin(f *testing.F) {
	lex := sampleLex()
	conv := sampleConverter()
	f.Add("听公厅功")
	f.Add("你好世界")
	f.Add("Hello")
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		p := pinyin.New(lex, conv)
		s := newState(t, input)
		_ = p.Process(context.Background(), s)
	})
}

// ----------------------------------------------------------------------------
// Benchmark
// ----------------------------------------------------------------------------

func BenchmarkPinyin(b *testing.B) {
	p := pinyin.New(sampleLex(), sampleConverter())
	inputs := []string{
		"听",
		"听公厅功",
		"听公厅功听公厅功听公厅功",
		"你好世界这是一个测试",
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
