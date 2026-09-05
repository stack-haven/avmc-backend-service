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

package lexnorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stack-haven/lexnorm/lexicon"
)

// Engine is the runtime Facade for ark-lexnorm.
//
// # 1.2 Decision D2: New Returns Error (fail-fast)
//
// New(opts ...Option) (*Engine, error) performs all validation at
// construction time. Configuration errors return ErrInvalidConfig
// immediately; the Engine is not partially constructed.
//
// # Construction Modes (mutually exclusive)
//
//	// Single-profile:
//	e, _ := lexnorm.New(
//	    lexnorm.WithLexicon(lex),
//	    lexnorm.WithPipeline(p),
//	)
//
//	// Multi-profile:
//	e, _ := lexnorm.New(
//	    lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
//	        "default": {Lexicon: lexA, Pipeline: pipeA},
//	        "asr":     {Lexicon: lexB, Pipeline: pipeB},
//	    }),
//	    lexnorm.WithDefaultProfile("default"),
//	)
//
//	// Dynamic:
//	e, _ := lexnorm.New(lexnorm.WithProfileResolver(myResolver))
//
// # Concurrency
//
// An Engine is safe for concurrent use across goroutines. Multiple
// Normalize calls may execute in parallel; each call uses its own
// State (invariant I4). The underlying Lexicon / Pipeline / Runtime
// components must be safe for concurrent read access.
type Engine struct {
	// Single-profile mode.
	singleLex  lexicon.Lexicon
	singlePipe Pipeline

	// Multi-profile mode.
	profiles       map[ProfileID]ProfileBundle
	defaultProfile ProfileID
	hasProfiles    bool

	// Dynamic mode.
	resolver ProfileResolver

	// HA mode (mutually exclusive with the modes above).
	// When set, Lexicon is captured from store.Current() per call.
	lexiconStore *lexicon.Store

	// Preset mode (counts as one mode; may co-exist with Lexicon source).
	// Holds the Preset reference for resolveRuntime.
	preset *Preset

	// Cross-cutting.
	middleware  []Middleware
	hooks       []Hook
	errorPolicy ErrorPolicy
	baseConfig  Config
}

// New constructs an Engine with the given Options.
//
// Returns (*Engine, nil) on success.
// Returns (nil, err) on any validation failure, where err wraps
// ErrInvalidConfig (use errors.Is to detect).
//
// # Validation
//
// New enforces (D2 fail-fast):
//
//   - Exactly one configuration mode: single-profile, multi-profile, or dynamic.
//   - Required fields per mode (e.g., Lexicon and Pipeline for single).
//   - Valid ErrorPolicy, Config (per Config.Validate).
//   - Valid ProfileBundle for each registered Profile.
func New(opts ...Option) (*Engine, error) {
	cfg := defaultEngineConfig()
	for _, opt := range opts {
		opt.apply(&cfg)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	e := &Engine{
		profiles:       cfg.profiles,
		defaultProfile: cfg.defaultProfile,
		hasProfiles:    cfg.hasProfiles,
		resolver:       cfg.resolver,
		lexiconStore:   cfg.lexiconStore,
		preset:         cfg.preset,
		middleware:     cfg.middleware,
		hooks:          cfg.hooks,
		errorPolicy:    cfg.errorPolicy,
		baseConfig:     cfg.config,
	}
	if cfg.lex != nil || cfg.pipe != nil {
		e.singleLex = cfg.lex
		e.singlePipe = cfg.pipe
	}
	return e, nil
}

// Normalize is the main entry point: processes text through the Engine's
// configured Pipeline and returns the Result.
//
// # Behavior
//
//  1. Resolves the Runtime for this call (CallOption override > Engine
//     default > ProfileResolver).
//  2. Constructs a State from the input text.
//  3. Triggers EventPipelineStart (Hook).
//  4. Executes the Middleware chain wrapping Pipeline execution.
//  5. Populates the Result with Text, Changes, Steps, RuntimeInfo, etc.
//  6. Triggers EventPipelineEnd (Hook).
//
// # Returns
//
//   - (Result, nil) on full success.
//   - (Result, nil) on partial success (Result.Status == StatusPartial).
//     In this case, Result.Text and Result.Changes are populated, and
//     Result.Errors contains the per-Processor errors.
//   - (Result, err) on cancellation (StatusCanceled) or fundamental
//     failure (StatusFailed). The error wraps the appropriate sentinel
//     (ErrRuntime, ErrInvalidConfig, etc.).
//
// # Concurrency
//
// Normalize is safe to call concurrently from multiple goroutines on the
// same Engine.
func (e *Engine) Normalize(ctx context.Context, text string, opts ...CallOption) (Result, error) {
	if e == nil {
		return Result{}, fmt.Errorf("nil Engine: %w", ErrRuntime)
	}

	startTime := time.Now()

	// 1. Parse call options.
	callCfg := defaultCallConfig()
	for _, opt := range opts {
		opt.apply(&callCfg)
	}

	// 2. Resolve Runtime.
	rt, err := e.resolveRuntime(ctx, callCfg)
	if err != nil {
		return Result{}, err
	}

	// 3. Construct State.
	state, err := NewState(ctx, text, rt.Lexicon, rt.Config)
	if err != nil {
		return Result{}, err
	}

	// 4. Trigger EventPipelineStart.
	triggerHooks(e.hooks, Event{
		Type:     EventPipelineStart,
		State:    state,
		Runtime:  rt,
		Duration: time.Since(startTime),
	})

	// 5. Build Middleware chain.
	inner := func(ctx context.Context, s *State) error {
		return e.runProcessors(ctx, s, rt)
	}
	handler := chainMiddleware(inner, e.middleware...)

	// 6. Execute.
	procErr := handler(ctx, state)

	// 7. Build Result.
	result := e.buildResult(state, rt, procErr, startTime)

	// 8. Trigger EventPipelineEnd.
	triggerHooks(e.hooks, Event{
		Type:     EventPipelineEnd,
		State:    state,
		Runtime:  rt,
		Result:   &result,
		Error:    procErr,
		Duration: result.Duration,
	})

	// 9. Return.
	if procErr != nil && result.Status == StatusFailed {
		return result, procErr
	}
	return result, nil
}

// resolveRuntime returns the Runtime for this call.
//
// Resolution priority:
//
//  1. WithRuntime CallOption (full override).
//  2. WithProfileID CallOption in multi-profile mode.
//  3. Engine's default in single-profile mode.
//  4. ProfileResolver for dynamic mode.
//  5. Engine's default in multi-profile mode.
//  6. LexiconStore for HA mode (captures current Snapshot per call).
func (e *Engine) resolveRuntime(ctx context.Context, cc callConfig) (*Runtime, error) {
	// 1. WithRuntime override.
	if cc.hasRuntime {
		if cc.runtime == nil {
			return nil, fmt.Errorf("WithRuntime(nil): %w", ErrRuntime)
		}
		return cc.runtime, nil
	}

	// 2. WithProfileID for multi-profile / resolver.
	if cc.profileID != "" {
		if e.resolver != nil {
			return e.resolver.Resolve(ctx, cc.profileID)
		}
		if e.hasProfiles {
			bundle, ok := e.profiles[cc.profileID]
			if !ok {
				return nil, fmt.Errorf("profile %q not registered: %w", cc.profileID, ErrRuntime)
			}
			return newRuntimeFromBundle(cc.profileID, bundle), nil
		}
		return nil, fmt.Errorf("WithProfileID requires WithProfiles or WithProfileResolver: %w", ErrRuntime)
	}

	// 3. Single-profile mode.
	if e.singleLex != nil && e.singlePipe != nil {
		id := ProfileID("default")
		return newRuntimeFromBundle(id, ProfileBundle{
			Lexicon:  e.singleLex,
			Pipeline: e.singlePipe,
			Config:   e.baseConfig,
		}), nil
	}

	// 4. Dynamic mode.
	if e.resolver != nil {
		// Need a ProfileID; default is "default".
		return e.resolver.Resolve(ctx, ProfileID("default"))
	}

	// 5. Multi-profile mode (default).
	if e.hasProfiles {
		id := e.defaultProfile
		if id == "" {
			// Pick any (deterministic: first key by sorted order).
			for k := range e.profiles {
				if id == "" || k < id {
					id = k
				}
			}
		}
		bundle := e.profiles[id]
		return newRuntimeFromBundle(id, bundle), nil
	}

	// 6. HA mode: capture current Snapshot from Store (architecture invariant I8).
	if e.lexiconStore != nil {
		lex := e.lexiconStore.Current()
		if lex == nil {
			return nil, fmt.Errorf("LexiconStore has no current Lexicon: %w", ErrRuntime)
		}
		return newRuntimeFromBundle(ProfileID("default"), ProfileBundle{
			Lexicon:  lex,
			Pipeline: e.singlePipe,
			Config:   e.baseConfig,
		}), nil
	}

	// 7. Preset mode: Pipeline + Config come from a Preset; Lexicon
	//    is optional (the Preset's Pipeline processors carry their
	//    own Lexicon references).
	if e.preset != nil {
		return newRuntimeFromBundle(ProfileID("default"), ProfileBundle{
			Lexicon:  e.singleLex, // may be nil; Pipeline processors have their own
			Pipeline: e.singlePipe,
			Config:   e.baseConfig,
		}), nil
	}

	return nil, fmt.Errorf("Engine has no profiles configured: %w", ErrRuntime)
}

// runProcessors executes each Processor in the Pipeline, collecting
// per-step timings and errors.
//
// Behavior depends on ErrorPolicy:
//
//   - ContinueOnError (default): all Processors are called.
//   - FailFast: stops on the first Processor error.
//
// Context cancellation is checked before each Processor.
func (e *Engine) runProcessors(ctx context.Context, s *State, rt *Runtime) error {
	processors := rt.Pipeline.Processors()
	steps := make([]StepTiming, 0, len(processors))
	var procErrs []error
	runStart := time.Now()

	for _, proc := range processors {
		// Check context.
		if err := ctx.Err(); err != nil {
			procErrs = append(procErrs, err)
			break
		}

		// Track changes before this Processor for ChangeCount.
		changesBefore := len(s.changes)

		// Trigger EventProcessorStart.
		processorVersion := ""
		if v, ok := proc.(Versioner); ok {
			processorVersion = v.Version()
		}
		triggerHooks(e.hooks, Event{
			Type:             EventProcessorStart,
			State:            s,
			Runtime:          rt,
			Processor:        proc.Name(),
			ProcessorVersion: processorVersion,
			Duration:         time.Since(runStart),
		})

		// Execute with timing.
		stepStart := time.Now()
		err := proc.Process(ctx, s)
		stepDuration := time.Since(stepStart)

		// B1 fix: back-fill Processor / ProcessorVersion on the Changes
		// produced by this Processor. State.Replace/Suggest/Rewrite leave
		// these empty by design (they cannot know which Processor owns
		// the call); the Engine is the authoritative source. We do this
		// BEFORE firing EventProcessorEnd so Hooks also see the populated
		// Change.Processor.
		for i := changesBefore; i < len(s.changes); i++ {
			s.changes[i].Processor = proc.Name()
			s.changes[i].ProcessorVersion = processorVersion
		}

		// Trigger EventProcessorEnd.
		triggerHooks(e.hooks, Event{
			Type:             EventProcessorEnd,
			State:            s,
			Runtime:          rt,
			Processor:        proc.Name(),
			ProcessorVersion: processorVersion,
			Error:            err,
			Duration:         stepDuration,
		})

		// Build StepTiming.
		timing := StepTiming{
			Processor:   proc.Name(),
			Duration:    stepDuration,
			ChangeCount: len(s.changes) - changesBefore,
		}
		if v, ok := proc.(Versioner); ok {
			timing.ProcessorVersion = v.Version()
		}
		if err != nil {
			timing.Error = err
			procErrs = append(procErrs, err)
		}
		steps = append(steps, timing)

		// FailFast policy.
		if err != nil && e.errorPolicy == FailFast {
			break
		}
	}

	// Attach steps to State for the Result builder.
	s.steps = steps
	return errors.Join(procErrs...)
}

// buildResult populates a Result from the executed State and Runtime.
func (e *Engine) buildResult(s *State, rt *Runtime, procErr error, startTime time.Time) Result {
	allChanges := s.Changes()
	suggestions := make([]Change, 0)
	for _, c := range allChanges {
		if !c.Applied {
			suggestions = append(suggestions, c)
		}
	}

	result := Result{
		Text:        s.Text(),
		Original:    s.Original(),
		Status:      StatusSuccess,
		Changes:     allChanges,
		Suggestions: suggestions,
		Steps:       s.steps,
		Duration:    time.Since(startTime),
		Runtime:     rt.info(),
	}

	// Determine Status.
	if procErr != nil {
		result.Errors = unwrapJoinedErrors(procErr)
		result.Err = procErr

		// StatusCanceled if any error is context.Canceled or context.DeadlineExceeded.
		hasContextErr := false
		for _, e := range result.Errors {
			if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
				hasContextErr = true
				break
			}
		}
		if hasContextErr {
			result.Status = StatusCanceled
		} else if len(allChanges) == 0 && e.errorPolicy == FailFast {
			result.Status = StatusFailed
		} else {
			result.Status = StatusPartial
		}
	}

	return result
}

// unwrapJoinedErrors splits an errors.Join result into a slice.
//
// Returns nil if err is nil. Returns [err] if err is not a joined error.
func unwrapJoinedErrors(err error) []error {
	if err == nil {
		return nil
	}
	type unwrapper interface {
		Unwrap() []error
	}
	if u, ok := err.(unwrapper); ok {
		out := make([]error, 0, len(u.Unwrap()))
		for _, e := range u.Unwrap() {
			if e != nil {
				out = append(out, e)
			}
		}
		return out
	}
	return []error{err}
}
