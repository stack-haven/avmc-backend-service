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

package ctxproc_test

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/processor/ctxproc"
)

var _ lexnorm.Processor = (*ctxproc.Processor)(nil)

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
	p := ctxproc.New()
	if p.Name() != "context" {
		t.Errorf("Name = %q, want context", p.Name())
	}
	if p.Version() != "v1" {
		t.Errorf("Version = %q, want v1", p.Version())
	}
	if p.Certainty() != lexnorm.CertaintyLow {
		t.Errorf("Certainty = %v, want CertaintyLow", p.Certainty())
	}
}

// ----------------------------------------------------------------------------
// Behavior: no-op
// ----------------------------------------------------------------------------

func TestContext_NoOp(t *testing.T) {
	p := ctxproc.New()
	tests := []string{
		"",
		"hello world",
		"你好世界",
		"mixed 你好 world",
		"long text that has many words and should be left unchanged by the no-op context processor",
	}
	for _, text := range tests {
		s := newState(t, text)
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatalf("Process(%q): %v", text, err)
		}
		if got := s.Text(); got != text {
			t.Errorf("Context.Process must not modify text\n  input:    %q\n  got:      %q\n  expected: %q",
				text, got, text)
		}
		if len(s.Changes()) != 0 {
			t.Errorf("Context.Process must not record Changes for %q, got %d",
				text, len(s.Changes()))
		}
	}
}

func TestContext_PreservesPriorChanges(t *testing.T) {
	// Context is no-op: it must not invalidate or modify changes from
	// earlier Processors (e.g., Normalize / Alias / Fuzzy).
	s := newState(t, "周莉群")
	cfg := s.Config()
	cfg.AutoApplyThreshold = 0.6
	cfg.SuggestThreshold = 0.4
	s, _ = lexnorm.NewState(context.Background(), "周莉群", nil, cfg)

	// Simulate a prior Apply by directly recording a Change.
	if err := s.Replace(lexnorm.Span{Start: 0, End: 9}, "周丽群", lexnorm.ChangeMeta{
		Source:     "test",
		Confidence: 0.95,
		Reason:     "pre-existing change",
	}); err != nil {
		t.Fatal(err)
	}

	p := ctxproc.New()
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	// Text and Changes must be preserved.
	if got := s.Text(); got != "周丽群" {
		t.Errorf("Text must be preserved by no-op Context, got %q", got)
	}
	changes := s.Changes()
	if len(changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(changes))
	}
	if !changes[0].Applied {
		t.Error("pre-existing Applied change must remain Applied")
	}
}

// ----------------------------------------------------------------------------
// Independence & determinism
// ----------------------------------------------------------------------------

func TestContext_IndependentOfEngine(t *testing.T) {
	p := ctxproc.New()
	s := newState(t, "test")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := s.Text(); got != "test" {
		t.Errorf("Text = %q, want %q", got, "test")
	}
}

func TestContext_Deterministic(t *testing.T) {
	p := ctxproc.New()
	for i := 0; i < 5; i++ {
		s := newState(t, "test input 你好 world")
		if err := p.Process(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := s.Text(); got != "test input 你好 world" {
			t.Errorf("run %d: Text = %q, want unchanged", i, got)
		}
	}
}

// ----------------------------------------------------------------------------
// Fuzz
// ----------------------------------------------------------------------------

func FuzzContext(f *testing.F) {
	f.Add("")
	f.Add("hello")
	f.Add("你好世界")
	f.Add("mixed 你好 world")
	f.Fuzz(func(t *testing.T, input string) {
		p := ctxproc.New()
		s := newState(t, input)
		_ = p.Process(context.Background(), s)
		if got := s.Text(); got != input {
			t.Errorf("Context must not modify text\n  input: %q\n  got:   %q", input, got)
		}
	})
}

// ----------------------------------------------------------------------------
// Benchmark
// ----------------------------------------------------------------------------

func BenchmarkContext(b *testing.B) {
	p := ctxproc.New()
	inputs := []string{
		"",
		"hello world",
		"你好世界这是一个测试字符串",
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
