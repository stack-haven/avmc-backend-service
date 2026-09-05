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

package deterministic_test

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/deterministic"
)

var _ lexnorm.Processor = (*deterministic.Processor)(nil)

func newState(t *testing.T, text string) *lexnorm.State {
	t.Helper()
	s, err := lexnorm.NewState(context.Background(), text, nil, lexnorm.DefaultConfig())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	return s
}

func correctionVariant(text string, confidence float64) lexicon.Variant {
	return lexicon.Variant{
		Text:       text,
		Kind:       lexicon.VariantCorrection,
		Confidence: confidence,
		Source:     "test",
	}
}

// sampleLex returns a Lexicon with correction variants for two entries.
func sampleLex() lexicon.Lexicon {
	// Build a minimal Lexicon via SliceSource + Compose (no ngram/pinyin).
	src := lexicon.NewSliceSource(
		[]lexicon.Entry{
			{
				ID:   "e-1",
				Text: "周丽群",
				Variants: []lexicon.Variant{
					correctionVariant("周丽裙", 0.95),
					correctionVariant("周丽郡", 0.85),
				},
			},
			{
				ID:   "e-2",
				Text: "田华",
				Variants: []lexicon.Variant{
					correctionVariant("田花", 1.0),
				},
			},
		},
		nil,
		"v1",
	)
	lex, err := lexicon.Compose(src)
	if err != nil {
		panic(err)
	}
	return lex
}

// ----------------------------------------------------------------------------
// Properties
// ----------------------------------------------------------------------------

func TestProcessor_Identity(t *testing.T) {
	p := deterministic.New(sampleLex())
	if p.Name() != "deterministic" {
		t.Errorf("Name = %q, want deterministic", p.Name())
	}
	if p.Version() != "v1" {
		t.Errorf("Version = %q, want v1", p.Version())
	}
	if p.Certainty() != lexnorm.CertaintyHigh {
		t.Errorf("Certainty = %v, want CertaintyHigh", p.Certainty())
	}
}

func TestProcessor_NilLex_NoOp(t *testing.T) {
	p := deterministic.New(nil)
	s := newState(t, "周丽裙")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周丽裙" {
		t.Errorf("nil Lexicon must be no-op, got %q", got)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("nil Lexicon must produce no Changes, got %d", len(s.Changes()))
	}
}

// ----------------------------------------------------------------------------
// Behavior
// ----------------------------------------------------------------------------

func TestDeterministic_BasicReplace(t *testing.T) {
	tests := []struct{ in, want string }{
		{"周丽裙", "周丽群"},
		{"你好周丽裙", "你好周丽群"},
		{"周丽裙和周丽郡", "周丽群和周丽群"},
		{"田花明天来", "田华明天来"},
	}
	for _, tc := range tests {
		s := newState(t, tc.in)
		p := deterministic.New(sampleLex())
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != tc.want {
			t.Errorf("deterministic(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDeterministic_PreservesCanonical(t *testing.T) {
	s := newState(t, "周丽群和田华")
	p := deterministic.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周丽群和田华" {
		t.Errorf("canonical forms must be preserved, got %q", got)
	}
}

func TestDeterministic_ChangeCarriesConfidence(t *testing.T) {
	s := newState(t, "周丽裙") // Confidence 0.95
	p := deterministic.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	changes := s.Changes()
	if len(changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(changes))
	}
	if changes[0].Confidence != 0.95 {
		t.Errorf("Confidence = %v, want 0.95", changes[0].Confidence)
	}
	if changes[0].EntryID != "e-1" {
		t.Errorf("EntryID = %q, want e-1", changes[0].EntryID)
	}
}

func TestDeterministic_DoesNotTouchAliasVariants(t *testing.T) {
	// Sample Lexicon only has Correction variants; Alias ones are ignored.
	s := newState(t, "周丽群") // canonical, no alias
	p := deterministic.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("canonical-only input must produce no Changes, got %d", len(s.Changes()))
	}
}

// ----------------------------------------------------------------------------
// Independence & determinism
// ----------------------------------------------------------------------------

func TestDeterministic_IndependentOfEngine(t *testing.T) {
	p := deterministic.New(sampleLex())
	s := newState(t, "周丽裙")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周丽群" {
		t.Errorf("Text = %q, want %q", got, "周丽群")
	}
}

func TestDeterministic_Deterministic(t *testing.T) {
	p := deterministic.New(sampleLex())
	for i := 0; i < 5; i++ {
		s := newState(t, "周丽裙、周丽郡、田花")
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != "周丽群、周丽群、田华" {
			t.Errorf("run %d: Text = %q, want %q", i, got, "周丽群、周丽群、田华")
		}
	}
}

// ----------------------------------------------------------------------------
// Fuzz
// ----------------------------------------------------------------------------

func FuzzDeterministic(f *testing.F) {
	lex := sampleLex()
	f.Add("周丽裙")
	f.Add("周丽裙和周丽郡")
	f.Add("周丽群") // canonical
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		p := deterministic.New(lex)
		s := newState(t, input)
		_ = p.Process(context.Background(), s)
	})
}

// ----------------------------------------------------------------------------
// Benchmark
// ----------------------------------------------------------------------------

func BenchmarkDeterministic(b *testing.B) {
	p := deterministic.New(sampleLex())
	inputs := []string{
		"周丽裙",
		"周丽裙、周丽郡、田花",
		"周丽群和田华",
		"周丽裙明天来周丽郡吃饭",
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
