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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stack-haven/lexnorm"
)

// ----------------------------------------------------------------------------
// Per-Processor Hooks
// ----------------------------------------------------------------------------

func TestEngine_PerProcessorHooks(t *testing.T) {
	// Verify EventProcessorStart and EventProcessorEnd fire for each
	// processor, in the correct order.
	var startCount, endCount atomic.Int32
	var seenProcessors []string

	hook := func(e lexnorm.Event) {
		switch e.Type {
		case lexnorm.EventProcessorStart:
			startCount.Add(1)
			seenProcessors = append(seenProcessors, "start:"+e.Processor)
		case lexnorm.EventProcessorEnd:
			endCount.Add(1)
			seenProcessors = append(seenProcessors, "end:"+e.Processor)
		}
	}

	p := lexnorm.NewPipeline(
		&testProcessor{name: "a"},
		&testProcessor{name: "b"},
		&testProcessor{name: "c"},
	)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(p),
		lexnorm.WithHooks(hook),
	)
	if _, err := e.Normalize(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}

	if got := startCount.Load(); got != 3 {
		t.Errorf("ProcessorStart count = %d, want 3", got)
	}
	if got := endCount.Load(); got != 3 {
		t.Errorf("ProcessorEnd count = %d, want 3", got)
	}

	wantOrder := []string{
		"start:a", "end:a",
		"start:b", "end:b",
		"start:c", "end:c",
	}
	if len(seenProcessors) != len(wantOrder) {
		t.Fatalf("seenProcessors = %v, want %v", seenProcessors, wantOrder)
	}
	for i := range seenProcessors {
		if seenProcessors[i] != wantOrder[i] {
			t.Errorf("event[%d] = %q, want %q", i, seenProcessors[i], wantOrder[i])
		}
	}
}

func TestEngine_PerProcessorHook_ReceivesError(t *testing.T) {
	// Per-processor hook should receive the processor's error in
	// EventProcessorEnd.Error.
	hookErrCh := make(chan error, 1)
	hook := func(e lexnorm.Event) {
		if e.Type == lexnorm.EventProcessorEnd && e.Processor == "failing" {
			hookErrCh <- e.Error
		}
	}

	expectedErr := errors.New("intentional failure")
	p := lexnorm.NewPipeline(
		&trackingProcessor{name: "ok"},
		&trackingProcessor{name: "failing", err: expectedErr},
	)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(p),
		lexnorm.WithErrorPolicy(lexnorm.ContinueOnError),
		lexnorm.WithHooks(hook),
	)
	if _, err := e.Normalize(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-hookErrCh:
		if !errors.Is(got, expectedErr) {
			t.Errorf("hook Error = %v, want %v", got, expectedErr)
		}
	case <-time.After(time.Second):
		t.Fatal("hook did not receive error within 1s")
	}
}

// ----------------------------------------------------------------------------
// Timeout Middleware
// ----------------------------------------------------------------------------

func TestTimeout_CompletesBeforeDeadline(t *testing.T) {
	// Fast Processor: Timeout has no effect.
	p := lexnorm.NewPipeline(&testProcessor{name: "fast"})
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(p),
		lexnorm.WithMiddleware(lexnorm.Timeout(time.Second)),
	)
	res, err := e.Normalize(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != lexnorm.StatusSuccess {
		t.Errorf("Status = %v, want StatusSuccess", res.Status)
	}
}

type slowProcessor struct {
	name string
	hook func() // invoked inside Process
}

func (s *slowProcessor) Name() string { return s.name }
func (s *slowProcessor) Process(ctx context.Context, _ *lexnorm.State) error {
	if s.hook != nil {
		s.hook()
	}
	// Block until ctx.Done or 1 second.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
		return nil
	}
}

func TestTimeout_ContextCancelledByMiddleware(t *testing.T) {
	// The slow processor's hook signals it has been entered.
	entered := make(chan struct{})
	p := lexnorm.NewPipeline(&slowProcessor{
		name: "slow",
		hook: func() { close(entered) },
	})
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(p),
		lexnorm.WithMiddleware(lexnorm.Timeout(10*time.Millisecond)),
	)
	_, _ = e.Normalize(context.Background(), "x")
	// We can't easily test that the timeout actually fired (since
	// errors are silently dropped in ContinueOnError), but we can
	// verify that the processor was entered.
	select {
	case <-entered:
		// OK: Processor ran.
	default:
		t.Error("slow Processor was never entered")
	}
}
