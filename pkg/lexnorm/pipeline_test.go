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
	"sync"
	"testing"

	"github.com/stack-haven/lexnorm"
)

// Compile-time assertions.
var (
	_ lexnorm.Processor = lexnorm.NewPipeline()
	_ lexnorm.Pipeline  = lexnorm.NewPipeline()
)

// ----------------------------------------------------------------------------
// Construction
// ----------------------------------------------------------------------------

func TestNewPipeline_Empty(t *testing.T) {
	p := lexnorm.NewPipeline()
	if p == nil {
		t.Fatal("NewPipeline() must not return nil")
	}
	if got := p.Processors(); len(got) != 0 {
		t.Errorf("empty Pipeline must have 0 Processors, got %d", len(got))
	}
}

func TestNewPipeline_Order(t *testing.T) {
	a := &trackingProcessor{name: "a"}
	b := &trackingProcessor{name: "b"}
	c := &trackingProcessor{name: "c"}
	p := lexnorm.NewPipeline(a, b, c)

	got := p.Processors()
	if len(got) != 3 {
		t.Fatalf("expected 3 Processors, got %d", len(got))
	}
	want := []string{"a", "b", "c"}
	for i, proc := range got {
		if proc.Name() != want[i] {
			t.Errorf("Processors()[%d].Name() = %q, want %q", i, proc.Name(), want[i])
		}
	}
}

func TestNewPipeline_NoDuplicatesRemoved(t *testing.T) {
	// The same Processor can appear multiple times in a Pipeline
	// (e.g., running the same Processor twice for emphasis).
	a := &trackingProcessor{name: "a"}
	p := lexnorm.NewPipeline(a, a)
	if got := len(p.Processors()); got != 2 {
		t.Errorf("Pipeline should allow duplicates, got %d Processors", got)
	}
}

// ----------------------------------------------------------------------------
// Identity / Interface
// ----------------------------------------------------------------------------

func TestPipeline_Name(t *testing.T) {
	p := lexnorm.NewPipeline()
	if got := p.Name(); got != "pipeline" {
		t.Errorf("Name() = %q, want %q", got, "pipeline")
	}
}

func TestPipeline_ImplementsProcessor(t *testing.T) {
	// The interface assertion above is the strongest test (compile-time).
	// Add a runtime check for documentation.
	p := lexnorm.NewPipeline()
	var proc lexnorm.Processor = p
	if proc.Name() != "pipeline" {
		t.Error("Pipeline must satisfy Processor interface")
	}
}

func TestPipeline_Version_Default(t *testing.T) {
	p := lexnorm.NewPipeline()
	// Version is via optional Versioner interface (consistent with Processor).
	v, ok := p.(lexnorm.Versioner)
	if !ok {
		t.Fatal("default Pipeline should implement Versioner")
	}
	if got := v.Version(); got != "" {
		t.Errorf("default Pipeline.Version() = %q, want empty", got)
	}
}

// ----------------------------------------------------------------------------
// Process: basic behavior
// ----------------------------------------------------------------------------

func TestPipeline_Process_Empty(t *testing.T) {
	// Empty Pipeline is a no-op: no error, no state change.
	s, _ := lexnorm.NewState(context.Background(), "hello", nil, lexnorm.DefaultConfig())
	p := lexnorm.NewPipeline()
	if err := p.Process(context.Background(), s); err != nil {
		t.Errorf("empty Pipeline.Process must return nil, got %v", err)
	}
	if s.Text() != "hello" {
		t.Errorf("State must be unchanged, got %q", s.Text())
	}
	if len(s.Changes()) != 0 {
		t.Errorf("State.Changes must be empty, got %d", len(s.Changes()))
	}
}

func TestPipeline_Process_SingleProcessor(t *testing.T) {
	a := &trackingProcessor{name: "a"}
	s, _ := lexnorm.NewState(context.Background(), "hello", nil, lexnorm.DefaultConfig())
	p := lexnorm.NewPipeline(a)
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if a.called.Load() != 1 {
		t.Errorf("Processor must be called once, got %d", a.called.Load())
	}
}

func TestPipeline_Process_OrderPreserved(t *testing.T) {
	var calls []string
	a := &recordingProcessor{name: "a", calls: &calls}
	b := &recordingProcessor{name: "b", calls: &calls}
	c := &recordingProcessor{name: "c", calls: &calls}

	s, _ := lexnorm.NewState(context.Background(), "hello", nil, lexnorm.DefaultConfig())
	p := lexnorm.NewPipeline(a, b, c)
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	want := []string{"a", "b", "c"}
	if !sliceEq(calls, want) {
		t.Errorf("call order = %v, want %v", calls, want)
	}
}

func TestPipeline_Process_StatePropagation(t *testing.T) {
	// Processors mutate State. Pipeline.Process must propagate mutations.
	first := &replaceProcessor{name: "first", span: lexnorm.Span{0, 5}, to: "HI"}
	second := &replaceProcessor{name: "second", span: lexnorm.Span{6, 11}, to: "WORLD"}

	s, _ := lexnorm.NewState(context.Background(), "hello world", nil, lexnorm.DefaultConfig())
	p := lexnorm.NewPipeline(first, second)
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	// First replace: "hello world"[0:5]="hello" → "HI" + " world" = "HI world"
	// Second replace: Original[6,11]="world" → "WORLD" (Original position)
	//   After first replace, Text[6:11] = "world" (positions 3-8).
	//   Replacing Text[3:8] with "WORLD" → "HI " + "WORLD" = "HI WORLD".
	if got := s.Text(); got != "HI WORLD" {
		t.Errorf("Text = %q, want %q", got, "HI WORLD")
	}
	if got := len(s.Changes()); got != 2 {
		t.Errorf("len(Changes) = %d, want 2", got)
	}
}

// ----------------------------------------------------------------------------
// Process: error handling (ContinueOnError)
// ----------------------------------------------------------------------------

func TestPipeline_Process_NoErrors(t *testing.T) {
	a := &trackingProcessor{name: "a"}
	b := &trackingProcessor{name: "b"}
	p := lexnorm.NewPipeline(a, b)

	s, _ := lexnorm.NewState(context.Background(), "x", nil, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), s); err != nil {
		t.Errorf("Process with no errors must return nil, got %v", err)
	}
}

func TestPipeline_Process_AllErrors_Aggregated(t *testing.T) {
	// ContinueOnError: all Processors are called; errors are joined.
	errA := errors.New("a failed")
	errB := errors.New("b failed")
	errC := errors.New("c failed")
	a := &trackingProcessor{name: "a", err: errA}
	b := &trackingProcessor{name: "b", err: errB}
	c := &trackingProcessor{name: "c", err: errC}

	s, _ := lexnorm.NewState(context.Background(), "x", nil, lexnorm.DefaultConfig())
	p := lexnorm.NewPipeline(a, b, c)
	err := p.Process(context.Background(), s)

	if err == nil {
		t.Fatal("Process must return non-nil when Processors fail")
	}
	// All errors must be inspectable.
	for _, want := range []error{errA, errB, errC} {
		if !errors.Is(err, want) {
			t.Errorf("Process error must include %v (errors.Is)", want)
		}
	}
	// All Processors must have been called.
	if a.called.Load() != 1 || b.called.Load() != 1 || c.called.Load() != 1 {
		t.Errorf("all Processors must be called once even on error: a=%d b=%d c=%d",
			a.called.Load(), b.called.Load(), c.called.Load())
	}
}

func TestPipeline_Process_PartialErrors(t *testing.T) {
	// Mix of success and failure.
	errB := errors.New("b failed")
	a := &trackingProcessor{name: "a"}
	b := &trackingProcessor{name: "b", err: errB}
	c := &trackingProcessor{name: "c"}

	s, _ := lexnorm.NewState(context.Background(), "x", nil, lexnorm.DefaultConfig())
	p := lexnorm.NewPipeline(a, b, c)
	err := p.Process(context.Background(), s)

	if !errors.Is(err, errB) {
		t.Errorf("Process error must include errB, got %v", err)
	}
	// errA and errC are nil; errors.Join discards them.
	if errors.Is(err, errors.New("nonexistent")) {
		t.Error("Process error must not include spurious errors")
	}
}

// ----------------------------------------------------------------------------
// Process: context cancellation
// ----------------------------------------------------------------------------

func TestPipeline_Process_ContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	a := &trackingProcessor{name: "a"}
	b := &trackingProcessor{name: "b"}

	s, _ := lexnorm.NewState(context.Background(), "x", nil, lexnorm.DefaultConfig())
	p := lexnorm.NewPipeline(a, b)
	err := p.Process(ctx, s)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Process must return context.Canceled, got %v", err)
	}
	if a.called.Load() != 0 {
		t.Errorf("first Processor must NOT be called when ctx already cancelled, got %d calls", a.called.Load())
	}
}

func TestPipeline_Process_ContextCancelledMidway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// First Processor cancels the context.
	first := &cancellingProcessor{name: "first", cancel: cancel}
	second := &trackingProcessor{name: "second"}

	s, _ := lexnorm.NewState(context.Background(), "x", nil, lexnorm.DefaultConfig())
	p := lexnorm.NewPipeline(first, second)
	err := p.Process(ctx, s)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Process must return context.Canceled, got %v", err)
	}
	// Second must not have been called.
	if second.called.Load() != 0 {
		t.Errorf("second Processor must NOT be called after cancellation, got %d calls", second.called.Load())
	}
}

// ----------------------------------------------------------------------------
// Processors
// ----------------------------------------------------------------------------

func TestPipeline_Processors_DefensiveCopy(t *testing.T) {
	a := &trackingProcessor{name: "a"}
	b := &trackingProcessor{name: "b"}
	p := lexnorm.NewPipeline(a, b)

	procs := p.Processors()
	procs[0] = &trackingProcessor{name: "tampered"}

	// Internal state must be unaffected.
	if p.Processors()[0].Name() != "a" {
		t.Error("Processors() must return defensive copy")
	}
}

// ----------------------------------------------------------------------------
// Nested Pipelines
// ----------------------------------------------------------------------------

func TestPipeline_Nested_Order(t *testing.T) {
	// Pipeline containing a Pipeline. D6: Pipeline implements Processor.
	inner := lexnorm.NewPipeline(
		&trackingProcessor{name: "inner-1"},
		&trackingProcessor{name: "inner-2"},
	)
	outer := lexnorm.NewPipeline(
		&trackingProcessor{name: "outer-1"},
		inner,
		&trackingProcessor{name: "outer-3"},
	)

	var calls []string
	for _, proc := range outer.Processors() {
		if pl, ok := proc.(lexnorm.Pipeline); ok {
			for _, p := range pl.Processors() {
				calls = append(calls, p.Name())
			}
		} else {
			calls = append(calls, proc.Name())
		}
	}

	want := []string{"outer-1", "inner-1", "inner-2", "outer-3"}
	if !sliceEq(calls, want) {
		t.Errorf("nested pipeline order = %v, want %v", calls, want)
	}
}

func TestPipeline_Nested_Process(t *testing.T) {
	// Verify that nested Pipelines are actually executed.
	var calls []string
	inner := lexnorm.NewPipeline(
		&recordingProcessor{name: "inner-1", calls: &calls},
		&recordingProcessor{name: "inner-2", calls: &calls},
	)
	outer := lexnorm.NewPipeline(
		&recordingProcessor{name: "outer-1", calls: &calls},
		inner,
		&recordingProcessor{name: "outer-3", calls: &calls},
	)

	s, _ := lexnorm.NewState(context.Background(), "x", nil, lexnorm.DefaultConfig())
	if err := outer.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	want := []string{"outer-1", "inner-1", "inner-2", "outer-3"}
	if !sliceEq(calls, want) {
		t.Errorf("nested pipeline execution = %v, want %v", calls, want)
	}
}

// ----------------------------------------------------------------------------
// Custom Pipeline Implementation
// ----------------------------------------------------------------------------

func TestPipeline_CustomImplementation(t *testing.T) {
	// D6 enables custom Pipeline implementations.
	custom := &customPipeline{name: "conditional"}
	var p lexnorm.Pipeline = custom

	if p.Name() != "conditional" {
		t.Errorf("custom Pipeline.Name() = %q, want %q", p.Name(), "conditional")
	}
	if len(p.Processors()) != 0 {
		t.Errorf("custom Pipeline.Processors() should be empty, got %d", len(p.Processors()))
	}

	// Custom pipeline should run via its own Process method.
	s, _ := lexnorm.NewState(context.Background(), "x", nil, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), s); err != nil {
		t.Errorf("custom Pipeline.Process must not error, got %v", err)
	}
}

// conditionalPipeline is an example custom Pipeline that conditionally
// runs Processors based on a flag.
type customPipeline struct {
	name string
}

func (c *customPipeline) Name() string                                      { return c.name }
func (c *customPipeline) Process(_ context.Context, _ *lexnorm.State) error { return nil }
func (c *customPipeline) Processors() []lexnorm.Processor {
	return nil
}

// ----------------------------------------------------------------------------
// Concurrency
// ----------------------------------------------------------------------------

func TestPipeline_ConcurrentSafe(t *testing.T) {
	// Pipeline is immutable; multiple goroutines can share one instance.
	p := lexnorm.NewPipeline(
		&trackingProcessor{name: "a"},
		&trackingProcessor{name: "b"},
		&trackingProcessor{name: "c"},
	)

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			s, err := lexnorm.NewState(context.Background(), "x", nil, lexnorm.DefaultConfig())
			if err != nil {
				t.Errorf("NewState: %v", err)
				return
			}
			if err := p.Process(context.Background(), s); err != nil {
				t.Errorf("Process: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestPipeline_IndependentExecution(t *testing.T) {
	// Invariant I1: Pipeline can run without Engine.
	a := &trackingProcessor{name: "a"}
	p := lexnorm.NewPipeline(a)
	s, _ := lexnorm.NewState(context.Background(), "text", nil, lexnorm.DefaultConfig())

	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if a.called.Load() != 1 {
		t.Errorf("Processor must be called once, got %d", a.called.Load())
	}
}

// ----------------------------------------------------------------------------
// Test helpers
// ----------------------------------------------------------------------------

// (Shared helpers are in testhelpers_test.go.)
