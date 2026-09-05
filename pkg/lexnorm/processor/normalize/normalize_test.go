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

package normalize_test

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/processor/normalize"
)

// Compile-time assertion.
var _ lexnorm.Processor = (*normalize.Processor)(nil)

// newState creates a State with the given text for testing.
func newState(t *testing.T, text string) *lexnorm.State {
	t.Helper()
	s, err := lexnorm.NewState(context.Background(), text, nil, lexnorm.DefaultConfig())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	return s
}

// ----------------------------------------------------------------------------
// Properties
// ----------------------------------------------------------------------------

func TestProcessor_Identity(t *testing.T) {
	p := normalize.New()
	if got := p.Name(); got != "normalize" {
		t.Errorf("Name = %q, want normalize", got)
	}
	if got := p.Version(); got != "v1" {
		t.Errorf("Version = %q, want v1", got)
	}
	if got := p.Certainty(); got != lexnorm.CertaintyHigh {
		t.Errorf("Certainty = %v, want CertaintyHigh", got)
	}
}

func TestProcessor_ImplementsProcessor(t *testing.T) {
	var _ lexnorm.Processor = normalize.New()
	var _ lexnorm.Versioner = normalize.New()
	var _ lexnorm.CertaintyReporter = normalize.New()
}

// ----------------------------------------------------------------------------
// Whitespace handling
// ----------------------------------------------------------------------------

func TestNormalize_CollapseSpaces(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello", "hello"},
		{"hello world", "hello world"},
		{"hello  world", "hello world"},
		{"hello   world", "hello world"},
		{"\thello\tworld", "hello world"},
		{"hello\nworld", "hello world"},
		{"hello\r\nworld", "hello world"},
		{"  hello  world  ", "hello world"},
		{"\t\n hello \n\t world \n\t", "hello world"},
	}
	for _, tc := range tests {
		s := newState(t, tc.in)
		p := normalize.New()
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatalf("Process(%q): %v", tc.in, err)
		}
		if got := s.Text(); got != tc.want {
			t.Errorf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalize_StripControlChars(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello\x00world", "helloworld"},
		{"hello\x07world", "helloworld"},
		{"hello\x1B[31mred\x1B[0m", "hello[31mred[0m"}, // ANSI escape stripped
	}
	for _, tc := range tests {
		s := newState(t, tc.in)
		p := normalize.New()
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != tc.want {
			t.Errorf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ----------------------------------------------------------------------------
// Full-width to half-width
// ----------------------------------------------------------------------------

func TestNormalize_FullWidthToHalf(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ABC", "ABC"},   // ASCII unchanged
		{"ＡＢＣ", "ABC"},   // Full-width letters
		{"１２３", "123"},   // Full-width digits
		{"！＠＃", "!@#"},   // Full-width symbols
		{"　", ""},        // Full-width space → trimmed
		{"你好", "你好"},     // CJK unchanged
		{"Ａ你好Ｂ", "A你好B"}, // Mixed: full-width + CJK
		{"Ａ Ｂ", "A B"},   // Full-width + space
	}
	for _, tc := range tests {
		s := newState(t, tc.in)
		p := normalize.New()
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != tc.want {
			t.Errorf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalize_FullWidthDisabled(t *testing.T) {
	s := newState(t, "ＡＢＣ")
	p := normalize.New().WithFullWidthToHalf(false)
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "ＡＢＣ" {
		t.Errorf("with FullWidthToHalf disabled, text should be unchanged, got %q", got)
	}
}

// ----------------------------------------------------------------------------
// Change recording
// ----------------------------------------------------------------------------

func TestNormalize_RecordsChange(t *testing.T) {
	s := newState(t, "  hello  ")
	p := normalize.New()
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	changes := s.Changes()
	// Per-position replaces: 2 leading spaces + 2 trailing spaces = 4 edits.
	if len(changes) != 4 {
		t.Fatalf("len(Changes) = %d, want 4", len(changes))
	}
	for i, c := range changes {
		if !c.Applied {
			t.Errorf("change %d: must be Applied=true", i)
		}
		if c.Source != "normalize" {
			t.Errorf("change %d: Source = %q, want normalize", i, c.Source)
		}
		if c.Confidence != 1.0 {
			t.Errorf("change %d: Confidence = %v, want 1.0", i, c.Confidence)
		}
	}
	// Final Text after all changes must be cleaned.
	if got := s.Text(); got != "hello" {
		t.Errorf("Text = %q, want %q", got, "hello")
	}
}

func TestNormalize_NoChangeWhenUnchanged(t *testing.T) {
	s := newState(t, "hello")
	p := normalize.New()
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("no Change should be recorded for unchanged input, got %d", len(s.Changes()))
	}
}

func TestNormalize_EmptyInput(t *testing.T) {
	s := newState(t, "")
	p := normalize.New()
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(s.Changes()) != 0 {
		t.Error("empty input must not produce Changes")
	}
}

// ----------------------------------------------------------------------------
// Independence (Invariant I1)
// ----------------------------------------------------------------------------

func TestNormalize_RunsWithoutEngine(t *testing.T) {
	// Normalize must be usable without Engine.
	p := normalize.New()
	s := newState(t, "  hello world  ")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "hello world" {
		t.Errorf("Text = %q, want %q", got, "hello world")
	}
}

// ----------------------------------------------------------------------------
// Determinism (Invariant I9)
// ----------------------------------------------------------------------------

func TestNormalize_Deterministic(t *testing.T) {
	p := normalize.New()
	for i := 0; i < 5; i++ {
		s := newState(t, "  hello\tworld\n  ")
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != "hello world" {
			t.Errorf("run %d: Text = %q, want %q", i, got, "hello world")
		}
	}
}

// ----------------------------------------------------------------------------
// Fuzz
// ----------------------------------------------------------------------------

func FuzzNormalize(f *testing.F) {
	p := normalize.New()
	f.Add("hello world")
	f.Add("  hello  world  ")
	f.Add("ＡＢＣ")
	f.Add("\x00\x07\x1B[31m")
	f.Add("你好世界")

	f.Fuzz(func(t *testing.T, input string) {
		s := newState(t, input)
		// Just ensure no panic.
		_ = p.Process(context.Background(), s)
	})
}

// ----------------------------------------------------------------------------
// Benchmark
// ----------------------------------------------------------------------------

func BenchmarkNormalize(b *testing.B) {
	p := normalize.New()
	inputs := []string{
		"hello world",
		"  hello   world  with   extra   spaces  ",
		"ＡＢＣ　defｇｈｉ", // full-width
		"mixed: ＡＢＣ abc 你好世界",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := newStateB(b, inputs[i%len(inputs)])
		_ = p.Process(context.Background(), s)
	}
}

// newStateB is the benchmark variant of newState (uses *testing.B).
func newStateB(b *testing.B, text string) *lexnorm.State {
	b.Helper()
	s, err := lexnorm.NewState(context.Background(), text, nil, lexnorm.DefaultConfig())
	if err != nil {
		b.Fatalf("NewState: %v", err)
	}
	return s
}
