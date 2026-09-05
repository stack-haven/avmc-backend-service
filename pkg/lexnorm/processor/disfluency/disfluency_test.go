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

package disfluency_test

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/processor/disfluency"
)

var _ lexnorm.Processor = (*disfluency.Processor)(nil)

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
	p := disfluency.New()
	if p.Name() != "disfluency" {
		t.Errorf("Name = %q, want disfluency", p.Name())
	}
	if p.Version() != "v1" {
		t.Errorf("Version = %q, want v1", p.Version())
	}
	if p.Certainty() != lexnorm.CertaintyHigh {
		t.Errorf("Certainty = %v, want CertaintyHigh", p.Certainty())
	}
}

func TestProcessor_DefaultTokens(t *testing.T) {
	p := disfluency.New()
	tokens := p.Tokens()
	if len(tokens) == 0 {
		t.Fatal("default tokens must not be empty")
	}
	for _, expected := range []string{"呃", "嗯", "那个", "然后"} {
		found := false
		for _, tok := range tokens {
			if tok == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("default tokens must contain %q", expected)
		}
	}
}

func TestProcessor_WithTokens(t *testing.T) {
	p := disfluency.New().WithTokens("foo", "bar")
	tokens := p.Tokens()
	if len(tokens) != 2 {
		t.Errorf("len(tokens) = %d, want 2", len(tokens))
	}
}

// ----------------------------------------------------------------------------
// Behavior
// ----------------------------------------------------------------------------

func TestDisfluency_RemoveSingleToken(t *testing.T) {
	tests := []struct{ in, want string }{
		{"呃你好", "你好"},
		{"嗯，你好", "，你好"},
		{"你好，呃", "你好，"},
		{"然后我们去吃饭", "我们去吃饭"},
		{"那个这个然后", ""},
	}
	for _, tc := range tests {
		s := newState(t, tc.in)
		p := disfluency.New()
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != tc.want {
			t.Errorf("disfluency(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDisfluency_MultipleOccurrences(t *testing.T) {
	// "呃那个，然后呃" → 4 fillers (呃×2 + 那个 + 然后) + 1 punctuation (，).
	// Disfluency removes only the filler words, leaving the punctuation.
	s := newState(t, "呃那个，然后呃")
	p := disfluency.New()
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "，" {
		t.Errorf("Text = %q, want %q", got, "，")
	}
	if got := len(s.Changes()); got != 4 {
		t.Errorf("len(Changes) = %d, want 4 (呃×2 + 那个 + 然后)", got)
	}
}

func TestDisfluency_PreservesNonToken(t *testing.T) {
	// Tokens not in the default list are preserved.
	s := newState(t, "你好，世界")
	p := disfluency.New()
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "你好，世界" {
		t.Errorf("Text = %q, want unchanged", got)
	}
}

func TestDisfluency_EmptyTokens(t *testing.T) {
	s := newState(t, "你好，呃世界")
	p := disfluency.New().WithTokens()
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "你好，呃世界" {
		t.Errorf("Text = %q, want unchanged", got)
	}
}

func TestDisfluency_CustomTokens(t *testing.T) {
	s := newState(t, "ABCdefABC")
	p := disfluency.New().WithTokens("ABC")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "def" {
		t.Errorf("Text = %q, want %q", got, "def")
	}
}

// ----------------------------------------------------------------------------
// Independence & determinism
// ----------------------------------------------------------------------------

func TestDisfluency_IndependentOfEngine(t *testing.T) {
	p := disfluency.New()
	s := newState(t, "呃你好")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "你好" {
		t.Errorf("Text = %q, want %q", got, "你好")
	}
}

func TestDisfluency_Deterministic(t *testing.T) {
	p := disfluency.New()
	for i := 0; i < 5; i++ {
		s := newState(t, "呃那个，然后我们呃去吃饭")
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		// Comma is preserved (Disfluency only removes filler words).
		if got := s.Text(); got != "，我们去吃饭" {
			t.Errorf("run %d: Text = %q, want %q", i, got, "，我们去吃饭")
		}
	}
}

// ----------------------------------------------------------------------------
// Fuzz
// ----------------------------------------------------------------------------

func FuzzDisfluency(f *testing.F) {
	p := disfluency.New()
	f.Add("呃你好")
	f.Add("那个，这个，然后")
	f.Add("你好，世界")
	f.Add("")

	f.Fuzz(func(t *testing.T, input string) {
		s := newState(t, input)
		_ = p.Process(context.Background(), s)
	})
}

// ----------------------------------------------------------------------------
// Benchmark
// ----------------------------------------------------------------------------

func BenchmarkDisfluency(b *testing.B) {
	p := disfluency.New()
	inputs := []string{
		"呃你好",
		"那个，这个，然后我们呃去吃饭",
		"你好，世界",
		"呃那个然后其实反正就是说你知道吗我们去吃饭",
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
