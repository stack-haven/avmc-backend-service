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

package alias_test

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/internal/lexutil"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
)

var _ lexnorm.Processor = (*alias.Processor)(nil)

func newState(t *testing.T, text string) *lexnorm.State {
	t.Helper()
	s, err := lexnorm.NewState(context.Background(), text, nil, lexnorm.DefaultConfig())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	return s
}

// sampleLex returns a Lexicon with two canonical names and their aliases.
func sampleLex() lexicon.Lexicon {
	return lexutil.NewMemLexicon([]lexicon.Entry{
		lexutil.SimpleEntry("e-1", "周丽群", lexutil.AliasVariant("周莉群")),
		lexutil.SimpleEntry("e-2", "田华", lexutil.AliasVariant("小田")),
	}, "v1")
}

// ----------------------------------------------------------------------------
// Properties
// ----------------------------------------------------------------------------

func TestProcessor_Identity(t *testing.T) {
	p := alias.New(sampleLex())
	if p.Name() != "alias" {
		t.Errorf("Name = %q, want alias", p.Name())
	}
	if p.Version() != "v1" {
		t.Errorf("Version = %q, want v1", p.Version())
	}
	if p.Certainty() != lexnorm.CertaintyHigh {
		t.Errorf("Certainty = %v, want CertaintyHigh", p.Certainty())
	}
}

func TestProcessor_NilLex_NoOp(t *testing.T) {
	p := alias.New(nil)
	s := newState(t, "周莉群")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周莉群" {
		t.Errorf("nil Lexicon must be no-op, got Text = %q", got)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("nil Lexicon must produce no Changes, got %d", len(s.Changes()))
	}
}

func TestProcessor_NoAliases_NoOp(t *testing.T) {
	lex := lexutil.NewMemLexicon([]lexicon.Entry{
		lexutil.SimpleEntry("e-1", "canonical"), // no Variants
	}, "v1")
	p := alias.New(lex)
	s := newState(t, "canonical")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("Lexicon without aliases must produce no Changes, got %d", len(s.Changes()))
	}
}

// ----------------------------------------------------------------------------
// Behavior
// ----------------------------------------------------------------------------

func TestAlias_BasicReplace(t *testing.T) {
	tests := []struct{ in, want string }{
		{"周莉群", "周丽群"},
		{"你好周莉群", "你好周丽群"},
		{"周莉群和田华", "周丽群和田华"},
		{"小田明天来", "田华明天来"},
	}
	for _, tc := range tests {
		s := newState(t, tc.in)
		p := alias.New(sampleLex())
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != tc.want {
			t.Errorf("alias(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAlias_PreservesCanonical(t *testing.T) {
	// Canonical forms should not be touched (no Variants that match).
	s := newState(t, "周丽群和田华")
	p := alias.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周丽群和田华" {
		t.Errorf("canonical forms must be preserved, got %q", got)
	}
	if len(s.Changes()) != 0 {
		t.Errorf("canonical-only input must produce no Changes, got %d", len(s.Changes()))
	}
}

func TestAlias_ChangeRecordsEntryID(t *testing.T) {
	s := newState(t, "周莉群")
	p := alias.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	changes := s.Changes()
	if len(changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(changes))
	}
	c := changes[0]
	if c.EntryID != "e-1" {
		t.Errorf("EntryID = %q, want e-1", c.EntryID)
	}
	if c.Source != "alias" {
		t.Errorf("Source = %q, want alias", c.Source)
	}
	if c.From != "周莉群" || c.To != "周丽群" {
		t.Errorf("From/To mismatch: %q/%q", c.From, c.To)
	}
}

func TestAlias_MultipleMatches(t *testing.T) {
	s := newState(t, "周莉群、小田、周莉群")
	p := alias.New(sampleLex())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周丽群、田华、周丽群" {
		t.Errorf("Text = %q, want %q", got, "周丽群、田华、周丽群")
	}
	if got := len(s.Changes()); got != 3 {
		t.Errorf("len(Changes) = %d, want 3", got)
	}
}

// ----------------------------------------------------------------------------
// Independence & determinism
// ----------------------------------------------------------------------------

func TestAlias_IndependentOfEngine(t *testing.T) {
	p := alias.New(sampleLex())
	s := newState(t, "周莉群")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "周丽群" {
		t.Errorf("Text = %q, want %q", got, "周丽群")
	}
}

func TestAlias_Deterministic(t *testing.T) {
	p := alias.New(sampleLex())
	for i := 0; i < 5; i++ {
		s := newState(t, "周莉群和小田")
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != "周丽群和田华" {
			t.Errorf("run %d: Text = %q, want %q", i, got, "周丽群和田华")
		}
	}
}

// ----------------------------------------------------------------------------
// Fuzz
// ----------------------------------------------------------------------------

func FuzzAlias(f *testing.F) {
	lex := sampleLex()
	f.Add("周莉群")
	f.Add("周莉群和小田")
	f.Add("周丽群")
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		p := alias.New(lex)
		s := newState(t, input)
		_ = p.Process(context.Background(), s)
	})
}

// ----------------------------------------------------------------------------
// Benchmark
// ----------------------------------------------------------------------------

func BenchmarkAlias(b *testing.B) {
	p := alias.New(sampleLex())
	inputs := []string{
		"周莉群",
		"周莉群、小田、周莉群",
		"周丽群和田华", // canonical-only
		"周莉群和小田一起去吃饭",
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
