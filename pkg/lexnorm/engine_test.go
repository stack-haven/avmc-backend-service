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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/internal/lexutil"
	"github.com/stack-haven/lexnorm/lexicon"
)

// ----------------------------------------------------------------------------
// Test helpers
// ----------------------------------------------------------------------------

// noopPipeline returns a Pipeline that does nothing (used to keep tests
// focused on Engine behavior rather than Pipeline semantics).
func noopPipeline() lexnorm.Pipeline {
	return lexnorm.NewPipeline(&noopProcessor{})
}

type noopProcessor struct{}

func (n *noopProcessor) Name() string                                      { return "noop" }
func (n *noopProcessor) Process(_ context.Context, _ *lexnorm.State) error { return nil }

// simpleLexicon returns a trivial Lexicon with one entry, useful when
// tests don't care about Lexicon contents but need a valid value.
func simpleLexicon() lexicon.Lexicon {
	return lexutil.NewMemLexicon(
		[]lexicon.Entry{lexutil.SimpleEntry("e-1", "canonical")},
		"v1",
	)
}

// ----------------------------------------------------------------------------
// New: validation (D2 fail-fast)
// ----------------------------------------------------------------------------

func TestNew_NoOptions_ReturnsError(t *testing.T) {
	// D2: missing all required configuration is a construction error.
	_, err := lexnorm.New()
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("New() with no options must return ErrInvalidConfig, got %v", err)
	}
}

func TestNew_OnlyLexicon_ReturnsError(t *testing.T) {
	_, err := lexnorm.New(lexnorm.WithLexicon(simpleLexicon()))
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("single-profile mode requires both Lexicon and Pipeline, got %v", err)
	}
}

func TestNew_OnlyPipeline_ReturnsError(t *testing.T) {
	_, err := lexnorm.New(lexnorm.WithPipeline(noopPipeline()))
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("single-profile mode requires both Lexicon and Pipeline, got %v", err)
	}
}

func TestNew_EmptyProfiles_ReturnsError(t *testing.T) {
	_, err := lexnorm.New(lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{}))
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("WithProfiles with empty map must return ErrInvalidConfig, got %v", err)
	}
}

func TestNew_DefaultProfileNotInMap_ReturnsError(t *testing.T) {
	_, err := lexnorm.New(
		lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
			"default": {Lexicon: simpleLexicon(), Pipeline: noopPipeline()},
		}),
		lexnorm.WithDefaultProfile("nonexistent"),
	)
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("WithDefaultProfile not in map must return ErrInvalidConfig, got %v", err)
	}
}

func TestNew_SingleProfile_OK(t *testing.T) {
	e, err := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(noopPipeline()),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if e == nil {
		t.Fatal("New must return non-nil Engine on success")
	}
}

func TestNew_MultiProfile_OK(t *testing.T) {
	e, err := lexnorm.New(
		lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
			"default": {Lexicon: simpleLexicon(), Pipeline: noopPipeline()},
			"asr":     {Lexicon: simpleLexicon(), Pipeline: noopPipeline()},
		}),
		lexnorm.WithDefaultProfile("default"),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if e == nil {
		t.Fatal("New must return non-nil Engine")
	}
}

func TestNew_Resolver_OK(t *testing.T) {
	resolver, err := lexnorm.NewStaticResolver(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
		"default": {Lexicon: simpleLexicon(), Pipeline: noopPipeline()},
	})
	if err != nil {
		t.Fatal(err)
	}
	e, err := lexnorm.New(lexnorm.WithProfileResolver(resolver))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if e == nil {
		t.Fatal("New must return non-nil Engine")
	}
}

func TestNew_MixedModes_ReturnsError(t *testing.T) {
	resolver, _ := lexnorm.NewStaticResolver(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
		"default": {Lexicon: simpleLexicon(), Pipeline: noopPipeline()},
	})
	_, err := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(noopPipeline()),
		lexnorm.WithProfileResolver(resolver),
	)
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("mixing single-profile and resolver must return ErrInvalidConfig, got %v", err)
	}
}

func TestNew_InvalidBundle_ReturnsError(t *testing.T) {
	_, err := lexnorm.New(
		lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
			"bad": {Lexicon: simpleLexicon(), Pipeline: nil}, // nil pipeline
		}),
	)
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("invalid ProfileBundle must return ErrInvalidConfig, got %v", err)
	}
}

func TestNew_InvalidConfig_ReturnsError(t *testing.T) {
	cfg := lexnorm.DefaultConfig()
	cfg.AutoApplyThreshold = -0.1
	_, err := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(noopPipeline()),
		lexnorm.WithConfig(cfg),
	)
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("invalid WithConfig must return ErrInvalidConfig, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// Normalize: single-profile mode
// ----------------------------------------------------------------------------

func TestNormalize_SingleProfile_Basic(t *testing.T) {
	e, err := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(noopPipeline()),
	)
	if err != nil {
		t.Fatal(err)
	}

	res, err := e.Normalize(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if res.Status != lexnorm.StatusSuccess {
		t.Errorf("Status = %v, want StatusSuccess", res.Status)
	}
	if res.Text != "hello world" {
		t.Errorf("Text = %q, want %q (noop)", res.Text, "hello world")
	}
	if res.Original != "hello world" {
		t.Errorf("Original = %q, want %q", res.Original, "hello world")
	}
}

func TestNormalize_SingleProfile_ResultFields(t *testing.T) {
	// D3: Result must have all required fields populated.
	pipe := lexnorm.NewPipeline(
		&replaceProcessor{name: "p", span: lexnorm.Span{0, 5}, to: "HI"},
	)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(pipe),
	)
	res, err := e.Normalize(context.Background(), "hello world")
	if err != nil {
		t.Fatal(err)
	}

	if res.Text != "HI world" {
		t.Errorf("Text = %q, want %q", res.Text, "HI world")
	}
	if res.Original != "hello world" {
		t.Error("Original missing")
	}
	if res.Status != lexnorm.StatusSuccess {
		t.Error("Status should be Success")
	}
	if len(res.Changes) != 1 {
		t.Errorf("len(Changes) = %d, want 1", len(res.Changes))
	}
	if len(res.Steps) != 1 {
		t.Errorf("len(Steps) = %d, want 1", len(res.Steps))
	}
	if res.Runtime.ProfileID == "" {
		t.Error("RuntimeInfo.ProfileID must be populated")
	}
	if res.Runtime.LexiconVersion == "" {
		t.Error("RuntimeInfo.LexiconVersion must be populated")
	}
	if res.Duration == 0 {
		t.Error("Duration must be > 0")
	}
}

func TestNormalize_SingleProfile_ChangesPropagated(t *testing.T) {
	pipe := lexnorm.NewPipeline(
		&replaceProcessor{name: "p1", span: lexnorm.Span{0, 5}, to: "HI"},
		&replaceProcessor{name: "p2", span: lexnorm.Span{6, 11}, to: "WORLD"},
	)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(pipe),
	)
	res, _ := e.Normalize(context.Background(), "hello world")
	if res.Text != "HI WORLD" {
		t.Errorf("Text = %q, want %q", res.Text, "HI WORLD")
	}
	if len(res.Changes) != 2 {
		t.Errorf("len(Changes) = %d, want 2", len(res.Changes))
	}
	if len(res.Steps) != 2 {
		t.Errorf("len(Steps) = %d, want 2", len(res.Steps))
	}
}

// ----------------------------------------------------------------------------
// Normalize: multi-profile mode
// ----------------------------------------------------------------------------

func TestNormalize_MultiProfile_Routing(t *testing.T) {
	// Profile A and Profile B have different Pipelines (different replacements).
	pipeA := lexnorm.NewPipeline(
		&replaceProcessor{name: "A", span: lexnorm.Span{0, 5}, to: "AAA"},
	)
	pipeB := lexnorm.NewPipeline(
		&replaceProcessor{name: "B", span: lexnorm.Span{0, 5}, to: "BBB"},
	)

	e, err := lexnorm.New(
		lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
			"A": {Lexicon: simpleLexicon(), Pipeline: pipeA},
			"B": {Lexicon: simpleLexicon(), Pipeline: pipeB},
		}),
		lexnorm.WithDefaultProfile("A"),
	)
	if err != nil {
		t.Fatal(err)
	}

	resA, _ := e.Normalize(context.Background(), "hello", lexnorm.WithProfileID("A"))
	if resA.Text != "AAA" {
		t.Errorf("Profile A: Text = %q, want %q", resA.Text, "AAA")
	}
	if resA.Runtime.ProfileID != "A" {
		t.Errorf("Profile A: ProfileID = %q, want %q", resA.Runtime.ProfileID, "A")
	}

	resB, _ := e.Normalize(context.Background(), "hello", lexnorm.WithProfileID("B"))
	if resB.Text != "BBB" {
		t.Errorf("Profile B: Text = %q, want %q", resB.Text, "BBB")
	}
	if resB.Runtime.ProfileID != "B" {
		t.Errorf("Profile B: ProfileID = %q, want %q", resB.Runtime.ProfileID, "B")
	}

	// Runtime isolation: A's Runtime must not affect B's call.
	if resA.Runtime.ProfileID == resB.Runtime.ProfileID {
		t.Error("Profile isolation broken")
	}
}

func TestNormalize_MultiProfile_Default(t *testing.T) {
	pipe := lexnorm.NewPipeline(
		&replaceProcessor{name: "p", span: lexnorm.Span{0, 5}, to: "DEFAULT"},
	)
	e, _ := lexnorm.New(
		lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
			"default": {Lexicon: simpleLexicon(), Pipeline: pipe},
		}),
		lexnorm.WithDefaultProfile("default"),
	)
	res, _ := e.Normalize(context.Background(), "hello") // no CallOption
	if res.Text != "DEFAULT" {
		t.Errorf("default Profile: Text = %q, want %q", res.Text, "DEFAULT")
	}
}

func TestNormalize_MultiProfile_UnknownID_ReturnsError(t *testing.T) {
	e, _ := lexnorm.New(
		lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
			"default": {Lexicon: simpleLexicon(), Pipeline: noopPipeline()},
		}),
		lexnorm.WithDefaultProfile("default"),
	)
	_, err := e.Normalize(context.Background(), "text", lexnorm.WithProfileID("missing"))
	if !errors.Is(err, lexnorm.ErrRuntime) {
		t.Errorf("unknown ProfileID must return ErrRuntime, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// Normalize: ProfileResolver mode
// ----------------------------------------------------------------------------

func TestNormalize_Resolver_Dynamic(t *testing.T) {
	// Resolver returns different Runtime based on ProfileID.
	resolver := &dynamicResolver{
		runtimes: map[lexnorm.ProfileID]*lexnorm.Runtime{},
	}

	pipeA := lexnorm.NewPipeline(&replaceProcessor{name: "A", span: lexnorm.Span{0, 3}, to: "X"})
	pipeB := lexnorm.NewPipeline(&replaceProcessor{name: "B", span: lexnorm.Span{0, 3}, to: "Y"})

	bundleA := lexnorm.ProfileBundle{Lexicon: simpleLexicon(), Pipeline: pipeA}
	bundleB := lexnorm.ProfileBundle{Lexicon: simpleLexicon(), Pipeline: pipeB}

	rtA := mustBuildRuntime(t, "A", bundleA)
	rtB := mustBuildRuntime(t, "B", bundleB)
	resolver.runtimes["A"] = rtA
	resolver.runtimes["B"] = rtB

	e, err := lexnorm.New(lexnorm.WithProfileResolver(resolver))
	if err != nil {
		t.Fatal(err)
	}

	resA, _ := e.Normalize(context.Background(), "abc", lexnorm.WithProfileID("A"))
	if resA.Text != "X" {
		t.Errorf("Profile A: Text = %q, want %q", resA.Text, "X")
	}
	resB, _ := e.Normalize(context.Background(), "abc", lexnorm.WithProfileID("B"))
	if resB.Text != "Y" {
		t.Errorf("Profile B: Text = %q, want %q", resB.Text, "Y")
	}
}

func TestNormalize_Resolver_UnknownID_ReturnsError(t *testing.T) {
	resolver, _ := lexnorm.NewStaticResolver(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
		"default": {Lexicon: simpleLexicon(), Pipeline: noopPipeline()},
	})
	e, _ := lexnorm.New(lexnorm.WithProfileResolver(resolver))
	_, err := e.Normalize(context.Background(), "text", lexnorm.WithProfileID("missing"))
	if !errors.Is(err, lexnorm.ErrRuntime) {
		t.Errorf("unknown ProfileID via resolver must return ErrRuntime, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// Normalize: CallOptions
// ----------------------------------------------------------------------------

func TestNormalize_WithRuntime_Override(t *testing.T) {
	// Engine has Pipeline A; call uses Pipeline B via WithRuntime.
	pipeA := lexnorm.NewPipeline(&replaceProcessor{name: "A", span: lexnorm.Span{0, 5}, to: "FROM_A"})
	pipeB := lexnorm.NewPipeline(&replaceProcessor{name: "B", span: lexnorm.Span{0, 5}, to: "FROM_B"})

	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(pipeA),
	)

	bundleB := lexnorm.ProfileBundle{Lexicon: simpleLexicon(), Pipeline: pipeB}
	rt := mustBuildRuntime(t, "custom", bundleB)
	res, err := e.Normalize(context.Background(), "hello", lexnorm.WithRuntime(rt))
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "FROM_B" {
		t.Errorf("WithRuntime override: Text = %q, want %q", res.Text, "FROM_B")
	}
	if res.Runtime.ProfileID != "custom" {
		t.Errorf("WithRuntime: ProfileID = %q, want %q", res.Runtime.ProfileID, "custom")
	}
}

func TestNormalize_WithRuntime_Nil_ReturnsError(t *testing.T) {
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(noopPipeline()),
	)
	_, err := e.Normalize(context.Background(), "text", lexnorm.WithRuntime(nil))
	if !errors.Is(err, lexnorm.ErrRuntime) {
		t.Errorf("WithRuntime(nil) must return ErrRuntime, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// Normalize: error handling
// ----------------------------------------------------------------------------

func TestNormalize_ProcessorError_ContinueOnError(t *testing.T) {
	pipe := lexnorm.NewPipeline(
		&trackingProcessor{name: "a"},
		&trackingProcessor{name: "b", err: errors.New("b failed")},
		&trackingProcessor{name: "c"},
	)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(pipe),
	)
	res, err := e.Normalize(context.Background(), "x")
	if err != nil {
		t.Errorf("ContinueOnError must NOT return non-nil error, got %v", err)
	}
	if res.Status != lexnorm.StatusPartial {
		t.Errorf("Status = %v, want StatusPartial", res.Status)
	}
	if len(res.Errors) != 1 {
		t.Errorf("len(Errors) = %d, want 1", len(res.Errors))
	}
	if !errors.Is(res.Err, errors.New("")) { /* any error check */
		// Test the contained error specifically.
		if !stringsContains(res.Err.Error(), "b failed") {
			t.Errorf("res.Err must contain 'b failed', got %v", res.Err)
		}
	}
}

func TestNormalize_ProcessorError_FailFast(t *testing.T) {
	a := &trackingProcessor{name: "a"}
	b := &trackingProcessor{name: "b", err: errors.New("b failed")}
	c := &trackingProcessor{name: "c"}
	pipe := lexnorm.NewPipeline(a, b, c)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(pipe),
		lexnorm.WithErrorPolicy(lexnorm.FailFast),
	)
	res, _ := e.Normalize(context.Background(), "x")
	if res.Status != lexnorm.StatusFailed {
		t.Errorf("Status = %v, want StatusFailed", res.Status)
	}
	if a.called.Load() != 1 {
		t.Errorf("a must be called once, got %d", a.called.Load())
	}
	if b.called.Load() != 1 {
		t.Errorf("b must be called once, got %d", b.called.Load())
	}
	if c.called.Load() != 0 {
		t.Errorf("c must NOT be called after FailFast, got %d", c.called.Load())
	}
}

// ----------------------------------------------------------------------------
// Normalize: context cancellation
// ----------------------------------------------------------------------------

func TestNormalize_ContextCanceled_StatusCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(noopPipeline()),
	)
	res, _ := e.Normalize(ctx, "x")
	if res.Status != lexnorm.StatusCanceled {
		t.Errorf("Status = %v, want StatusCanceled", res.Status)
	}
}

func TestNormalize_ContextCanceledMidway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	first := &cancellingProcessor{name: "first", cancel: cancel}
	second := &trackingProcessor{name: "second"}
	pipe := lexnorm.NewPipeline(first, second)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(pipe),
	)
	res, _ := e.Normalize(ctx, "x")
	if res.Status != lexnorm.StatusCanceled {
		t.Errorf("Status = %v, want StatusCanceled", res.Status)
	}
	if second.called.Load() != 0 {
		t.Errorf("second must not be called after cancellation, got %d", second.called.Load())
	}
}

// ----------------------------------------------------------------------------
// Concurrency
// ----------------------------------------------------------------------------

func TestNormalize_Concurrent100Goroutines(t *testing.T) {
	// Per M5 requirement: 100+ goroutines concurrent Normalize.
	pipe := lexnorm.NewPipeline(
		&replaceProcessor{name: "p", span: lexnorm.Span{0, 5}, to: "HI"},
	)
	e, err := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(pipe),
	)
	if err != nil {
		t.Fatal(err)
	}

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			res, err := e.Normalize(context.Background(), "hello world")
			if err != nil {
				t.Errorf("Normalize: %v", err)
				return
			}
			if res.Text != "HI world" {
				t.Errorf("Text = %q, want %q", res.Text, "HI world")
			}
		}()
	}
	wg.Wait()
}

func TestNormalize_Concurrent_MultiProfile(t *testing.T) {
	// Concurrent calls across multiple Profiles.
	pipeA := lexnorm.NewPipeline(&replaceProcessor{name: "A", span: lexnorm.Span{0, 5}, to: "AAA"})
	pipeB := lexnorm.NewPipeline(&replaceProcessor{name: "B", span: lexnorm.Span{0, 5}, to: "BBB"})

	e, _ := lexnorm.New(
		lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
			"A": {Lexicon: simpleLexicon(), Pipeline: pipeA},
			"B": {Lexicon: simpleLexicon(), Pipeline: pipeB},
		}),
		lexnorm.WithDefaultProfile("A"),
	)

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N * 2)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			res, _ := e.Normalize(context.Background(), "hello", lexnorm.WithProfileID("A"))
			if res.Text != "AAA" {
				t.Errorf("A: Text = %q, want %q", res.Text, "AAA")
			}
		}()
		go func() {
			defer wg.Done()
			res, _ := e.Normalize(context.Background(), "hello", lexnorm.WithProfileID("B"))
			if res.Text != "BBB" {
				t.Errorf("B: Text = %q, want %q", res.Text, "BBB")
			}
		}()
	}
	wg.Wait()
}

// ----------------------------------------------------------------------------
// Hooks and Middleware
// ----------------------------------------------------------------------------

func TestNormalize_Hooks_Fire(t *testing.T) {
	var startCalled, endCalled atomic.Int32
	hook := func(e lexnorm.Event) {
		switch e.Type {
		case lexnorm.EventPipelineStart:
			startCalled.Add(1)
		case lexnorm.EventPipelineEnd:
			endCalled.Add(1)
		}
	}

	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(noopPipeline()),
		lexnorm.WithHooks(hook),
	)
	_, err := e.Normalize(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if startCalled.Load() != 1 {
		t.Errorf("PipelineStart hook must fire once, got %d", startCalled.Load())
	}
	if endCalled.Load() != 1 {
		t.Errorf("PipelineEnd hook must fire once, got %d", endCalled.Load())
	}
}

func TestNormalize_Hooks_SeeResult(t *testing.T) {
	// PipelineEnd hook should see the populated Result.
	pipe := lexnorm.NewPipeline(
		&replaceProcessor{name: "p", span: lexnorm.Span{0, 3}, to: "XYZ"},
	)
	var capturedResult *lexnorm.Result
	hook := func(e lexnorm.Event) {
		if e.Type == lexnorm.EventPipelineEnd {
			r := e.Result
			capturedResult = r
		}
	}

	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(pipe),
		lexnorm.WithHooks(hook),
	)
	_, err := e.Normalize(context.Background(), "abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if capturedResult == nil {
		t.Fatal("hook did not receive Result")
	}
	if capturedResult.Text != "XYZdef" {
		t.Errorf("hook Result.Text = %q, want %q", capturedResult.Text, "XYZdef")
	}
	if capturedResult.Original != "abcdef" {
		t.Errorf("hook Result.Original = %q, want %q", capturedResult.Original, "abcdef")
	}
}

func TestNormalize_Middleware_Timing(t *testing.T) {
	// Middleware that records timing.
	var middlewareCalled atomic.Int32
	timingMW := func(next lexnorm.Handler) lexnorm.Handler {
		return func(ctx context.Context, s *lexnorm.State) error {
			middlewareCalled.Add(1)
			start := time.Now()
			err := next(ctx, s)
			if time.Since(start) < 0 {
				t.Error("time went backwards")
			}
			return err
		}
	}

	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(noopPipeline()),
		lexnorm.WithMiddleware(timingMW),
	)
	_, err := e.Normalize(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if middlewareCalled.Load() != 1 {
		t.Errorf("middleware must be called once, got %d", middlewareCalled.Load())
	}
}

func TestNormalize_Middleware_Order(t *testing.T) {
	// First registered middleware is outermost.
	var order []string
	mw1 := func(next lexnorm.Handler) lexnorm.Handler {
		return func(ctx context.Context, s *lexnorm.State) error {
			order = append(order, "mw1-before")
			err := next(ctx, s)
			order = append(order, "mw1-after")
			return err
		}
	}
	mw2 := func(next lexnorm.Handler) lexnorm.Handler {
		return func(ctx context.Context, s *lexnorm.State) error {
			order = append(order, "mw2-before")
			err := next(ctx, s)
			order = append(order, "mw2-after")
			return err
		}
	}

	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(&orderRecordingPipeline{order: &order, name: "pipe"}),
		lexnorm.WithMiddleware(mw1, mw2),
	)
	_, _ = e.Normalize(context.Background(), "x")
	want := []string{"mw1-before", "mw2-before", "pipe", "mw2-after", "mw1-after"}
	if !sliceEqStrings(order, want) {
		t.Errorf("middleware order = %v, want %v", order, want)
	}
}

func TestRecoverMiddleware_PanicRecovery(t *testing.T) {
	pipe := lexnorm.NewPipeline(&panickingProcessor{})
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(pipe),
		lexnorm.WithErrorPolicy(lexnorm.FailFast),
		lexnorm.WithMiddleware(lexnorm.Recover()),
	)
	res, _ := e.Normalize(context.Background(), "x")
	// With FailFast and zero changes, a recovered panic produces StatusFailed.
	if res.Status != lexnorm.StatusFailed {
		t.Errorf("Status = %v, want StatusFailed", res.Status)
	}
	if !stringsContains(res.Err.Error(), "panic") {
		t.Errorf("Err must mention 'panic', got %v", res.Err)
	}
}

// ----------------------------------------------------------------------------
// HA: WithLexiconStore (Scenario D from acceptance)
// ----------------------------------------------------------------------------

func TestEngine_WithLexiconStore_Basic(t *testing.T) {
	v1 := mustBuildLexiconForEngine(t, "v1")
	store := lexicon.NewStore(v1)

	e, err := lexnorm.New(
		lexnorm.WithLexiconStore(store),
		lexnorm.WithPipeline(noopPipeline()),
	)
	if err != nil {
		t.Fatal(err)
	}

	res, _ := e.Normalize(context.Background(), "hello")
	if res.Runtime.LexiconVersion != "v1" {
		t.Errorf("Runtime.LexiconVersion = %q, want v1", res.Runtime.LexiconVersion)
	}
}

func TestEngine_WithLexiconStore_HotSwap_RequestConsistency(t *testing.T) {
	// Scenario D: Lexicon 热更新场景。
	v1 := mustBuildLexiconForEngine(t, "v1")
	store := lexicon.NewStore(v1)

	e, err := lexnorm.New(
		lexnorm.WithLexiconStore(store),
		lexnorm.WithPipeline(noopPipeline()),
	)
	if err != nil {
		t.Fatal(err)
	}

	// First call uses V1.
	res1, _ := e.Normalize(context.Background(), "hello")
	if res1.Runtime.LexiconVersion != "v1" {
		t.Errorf("first call version = %q, want v1", res1.Runtime.LexiconVersion)
	}

	// Hot swap to V2.
	v2 := mustBuildLexiconForEngine(t, "v2")
	if err := store.Swap(v2); err != nil {
		t.Fatal(err)
	}

	// Subsequent calls use V2.
	res2, _ := e.Normalize(context.Background(), "hello")
	if res2.Runtime.LexiconVersion != "v2" {
		t.Errorf("post-swap call version = %q, want v2", res2.Runtime.LexiconVersion)
	}
}

func TestEngine_WithLexiconStore_TryUpdate_LKG(t *testing.T) {
	// Successful TryUpdate advances to V2.
	// Failed TryUpdate keeps V1 (LKG).
	v1 := mustBuildLexiconForEngine(t, "v1")
	store := lexicon.NewStore(v1)

	e, _ := lexnorm.New(
		lexnorm.WithLexiconStore(store),
		lexnorm.WithPipeline(noopPipeline()),
	)

	v2 := mustBuildLexiconForEngine(t, "v2")
	if err := store.TryUpdate(func() (lexicon.Lexicon, error) { return v2, nil }); err != nil {
		t.Fatal(err)
	}

	res1, _ := e.Normalize(context.Background(), "x")
	if res1.Runtime.LexiconVersion != "v2" {
		t.Errorf("after successful TryUpdate: version = %q, want v2", res1.Runtime.LexiconVersion)
	}

	// Failed TryUpdate keeps V2 (LKG).
	failErr := errors.New("build failed")
	if err := store.TryUpdate(func() (lexicon.Lexicon, error) { return nil, failErr }); err == nil {
		t.Error("failed TryUpdate must return error")
	}
	res2, _ := e.Normalize(context.Background(), "x")
	if res2.Runtime.LexiconVersion != "v2" {
		t.Errorf("after failed TryUpdate: version = %q, want v2 (LKG)", res2.Runtime.LexiconVersion)
	}
}

func TestEngine_WithLexiconStore_RequiresPipeline(t *testing.T) {
	store := lexicon.NewStore(mustBuildLexiconForEngine(t, "v1"))
	_, err := lexnorm.New(lexnorm.WithLexiconStore(store))
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("WithLexiconStore without WithPipeline must return ErrInvalidConfig, got %v", err)
	}
}

func TestEngine_WithLexiconStore_MutuallyExclusive(t *testing.T) {
	store := lexicon.NewStore(mustBuildLexiconForEngine(t, "v1"))
	_, err := lexnorm.New(
		lexnorm.WithLexiconStore(store),
		lexnorm.WithPipeline(noopPipeline()),
		lexnorm.WithLexicon(simpleLexicon()), // conflict
	)
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("mixing Store and Lexicon must return ErrInvalidConfig, got %v", err)
	}
}

func TestEngine_WithLexiconStore_Uninitialized_ReturnsError(t *testing.T) {
	// Store with no initial Lexicon and no successful Swap.
	store := lexicon.NewStore(nil)
	e, _ := lexnorm.New(
		lexnorm.WithLexiconStore(store),
		lexnorm.WithPipeline(noopPipeline()),
	)
	_, err := e.Normalize(context.Background(), "x")
	if !errors.Is(err, lexnorm.ErrRuntime) {
		t.Errorf("uninitialized Store must yield ErrRuntime, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// Test helpers
// ----------------------------------------------------------------------------

// mustBuildLexiconForEngine creates a Lexicon for Engine integration tests.
func mustBuildLexiconForEngine(t *testing.T, version string) lexicon.Lexicon {
	t.Helper()
	lex, err := lexicon.NewBuilderWithVersion(version).
		Add(lexicon.Entry{ID: "e1", Text: "hello-" + version}).
		Build()
	if err != nil {
		t.Fatalf("Build Lexicon: %v", err)
	}
	return lex
}

// dynamicResolver is a ProfileResolver backed by a map (for tests).
type dynamicResolver struct {
	runtimes map[lexnorm.ProfileID]*lexnorm.Runtime
}

func (d *dynamicResolver) Resolve(_ context.Context, id lexnorm.ProfileID) (*lexnorm.Runtime, error) {
	rt, ok := d.runtimes[id]
	if !ok {
		return nil, errors.New("not found: " + string(id))
	}
	return rt, nil
}

// mustBuildRuntime builds a Runtime from a ProfileBundle and a ProfileID.
func mustBuildRuntime(t *testing.T, id string, b lexnorm.ProfileBundle) *lexnorm.Runtime {
	t.Helper()
	rt, err := lexnorm.NewRuntime(lexnorm.ProfileID(id), b)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	return rt
}

// sliceEqStrings is an alias for sliceEq (used for middleware ordering tests).
var sliceEqStrings = sliceEq
