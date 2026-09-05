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
	"errors"
	"fmt"
	"time"
)

// Result is the immutable outcome of a single Normalize call.
//
// # Value Semantics (Architecture Invariant)
//
// Result is a value type. The Engine returns a Result by value; callers
// may freely copy it, share it across goroutines, or send it over a
// channel. Internally it holds slice/map fields but the slice headers are
// copied and the underlying arrays are not mutated after Engine returns.
//
// # Field Categories
//
//   - Identity:     Status, Err, Duration
//   - Output:       Text, Original, Changes, Suggestions, Errors
//   - Observability: Steps, Runtime
//   - Aggregates:   Err (errors.Join of Errors)
//
// # 1.2 Decision D3
//
// All fields from both 1.0 and 1.1 are present. Original / Duration /
// Steps / Err (1.0) coexist with Suggestions / Errors / Runtime (1.1)
// because they serve different purposes:
//
//   - Original / Duration / Steps: per-call metadata (audit, performance).
//   - Suggestions / Errors / Runtime: finer-grained inspection (UI,
//     observability).
type Result struct {
	// Text is the normalized output. Equal to Original when no Processor
	// applied any change.
	Text string

	// Original is the input text passed to Normalize. Preserved so
	// callers can compute diffs and audit chains without keeping their
	// own copy.
	Original string

	// Status describes the overall outcome (success / partial / canceled
	// / failed). See [Status] for semantics.
	Status Status

	// Changes contains all recorded Changes (applied + suggested), in
	// the order they were produced by the Pipeline. Order is meaningful
	// for replay and audit.
	Changes []Change

	// Suggestions is a convenience slice of Changes with Applied=false.
	// It is a derived view (not stored separately): populated by the
	// Engine before returning.
	Suggestions []Change

	// Errors contains per-step errors (Processor errors that did not
	// abort the call). When ErrorPolicy=ContinueOnError (default), these
	// are non-fatal; when ErrorPolicy=FailFast, only the first is present.
	Errors []error

	// Err is errors.Join(Errors...), or nil if Errors is empty. Provided
	// for the common idiom:
	//
	//	res, err := engine.Normalize(ctx, text)
	//	if err != nil { ... }
	//	if errors.Is(err, lexnorm.ErrInvalidConfig) { ... }
	Err error

	// Duration is the wall-clock time spent in Normalize, including all
	// Processor execution. Useful for performance monitoring and SLO.
	Duration time.Duration

	// Steps captures per-Processor timing in Pipeline order. Order is
	// deterministic (matches Pipeline.Processors()).
	Steps []StepTiming

	// Runtime is the immutable snapshot of the runtime context. Captured
	// at the start of Normalize and never modified.
	Runtime RuntimeInfo
}

// IsZero reports whether r is the zero value (no Normalize was run).
func (r Result) IsZero() bool {
	return r.Text == "" &&
		r.Original == "" &&
		r.Status == StatusSuccess &&
		len(r.Changes) == 0 &&
		len(r.Errors) == 0 &&
		r.Duration == 0 &&
		len(r.Steps) == 0 &&
		r.Runtime.IsZero()
}

// HasChanges reports whether any Change (applied or suggested) was
// produced.
func (r Result) HasChanges() bool {
	return len(r.Changes) > 0
}

// HasErrors reports whether any error was recorded.
func (r Result) HasErrors() bool {
	return len(r.Errors) > 0
}

// String returns a compact human-readable summary suitable for logs.
func (r Result) String() string {
	return fmt.Sprintf(
		"Result{status=%s text_len=%d changes=%d suggestions=%d errors=%d duration=%s}",
		r.Status,
		len(r.Text),
		len(r.Changes),
		len(r.Suggestions),
		len(r.Errors),
		r.Duration,
	)
}

// StepTiming records the execution of a single Processor within a
// Pipeline run.
//
// StepTiming is part of Result.Steps. Order matches Pipeline.Processors()
// (deterministic).
type StepTiming struct {
	// Processor is the Name() of the Processor.
	Processor string

	// ProcessorVersion is the Version() of the Processor if it implements
	// Versioner, empty otherwise.
	ProcessorVersion string

	// Action is the high-level outcome: ActionReplace / ActionRemove /
	// ActionSuggest. Mixed (e.g., a Processor that emits both Replace
	// and Suggest) is summarized by the most impactful Action, with the
	// breakdown available via Result.Changes filtered by Processor.
	Action Action

	// ChangeCount is the number of Changes emitted by this Processor.
	ChangeCount int

	// Duration is the wall-clock time spent in this Processor.
	Duration time.Duration

	// Error is the Processor error, or nil on success.
	//
	// The underlying error is wrapped in *ProcessorError to carry the
	// Processor name; use errors.As(err, &lexnorm.ProcessorError{}) to
	// extract it.
	Error error
}

// IsZero reports whether st is the zero value.
func (st StepTiming) IsZero() bool {
	return st.Processor == "" &&
		st.ProcessorVersion == "" &&
		st.ChangeCount == 0 &&
		st.Duration == 0 &&
		st.Error == nil
}

// HasError reports whether this step produced an error.
func (st StepTiming) HasError() bool {
	return st.Error != nil
}

// joinErrors is a convenience for Result.Err construction.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return errors.Join(errs...)
}
