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
	"fmt"
)

// Processor is the minimal normalization capability unit.
//
// # Frozen API (v1.0+)
//
// The Processor interface is permanently frozen as of v1.0. Two methods,
// no more, no less:
//
//   - Name(): for identification in logs, Change.Processor, and Registry.
//   - Process(): performs the normalization step.
//
// # Mandatory Invariants (see .agents/RULES.md §2)
//
//   - I1: Processor can run independently of Engine.
//   - I3: Processor does not modify Lexicon.
//   - I6: All text modifications must go through State.Replace / Suggest.
//   - I9: Processor must be deterministic given identical inputs and
//     identical RuntimeInfo.
//
// # Implementation Note
//
// The Process method receives a *State that is exclusive to this call.
// Processors MUST NOT share State across goroutines or store references
// to it for later use.
type Processor interface {
	// Name returns a stable identifier for this Processor.
	//
	// Name is used in:
	//   - Result.Change.Processor (audit trail)
	//   - Result.StepTiming.Processor
	//   - Registry lookups
	//   - log messages
	//
	// Two Processors of the same kind MUST return the same Name (e.g.,
	// all instances of Normalize return "normalize"). Different
	// configurations of the same Processor should also return the same
	// Name; differentiate via Version() if needed.
	Name() string

	// Process performs the normalization step.
	//
	// Process MAY:
	//   - Read State.Text(), State.Lexicon(), State.Config()
	//   - Call State.IsLocked(span) to check Protected Spans
	//   - Call State.Replace(span, to, meta) to apply a change
	//   - Call State.Suggest(span, to, meta) to propose without applying
	//   - Call State.Lock(span) to protect a region
	//   - Return nil on success, or a non-nil error on failure
	//
	// Process MUST NOT:
	//   - Mutate the Lexicon (invariant I3)
	//   - Bypass State to modify State.Text directly (invariant I6)
	//   - Block indefinitely (use ctx for cancellation)
	//   - Share State across goroutines
	Process(ctx context.Context, s *State) error
}

// Versioner is an optional interface implemented by Processors that have
// a version.
//
// Versioner is OPTIONAL. Processors that do not implement it are still
// fully valid; the engine simply leaves Change.ProcessorVersion and
// RuntimeInfo.ProcessorVersions[name] empty for that Processor.
//
// # When to Implement
//
//   - Built-in Processors: always implement Versioner (with a semantic
//     version string like "v1.2.3").
//   - User-defined Processors: implement Versioner when the Processor has
//     a meaningful semantic version (e.g., backed by a model or rule
//     set that evolves over time).
//   - Trivial Processors (e.g., stateless wrappers): may omit Versioner.
type Versioner interface {
	// Version returns a semantic version string for this Processor.
	//
	// The returned string is opaque to the engine and is used purely for
	// audit / replay. Common formats:
	//   - Semantic version: "v1.2.3"
	//   - Git SHA: "abc1234"
	//   - Date-based: "v20240904"
	//
	// MUST be a pure function of the Processor's configuration: identical
	// configs MUST return identical versions (determinism invariant I9).
	Version() string
}

// CertaintyReporter is an optional interface implemented by Processors that
// declare a confidence tier.
//
// CertaintyReporter is OPTIONAL. Processors that do not implement it
// have no self-declared Certainty (useful for Processors whose certainty
// is per-call rather than per-kind).
type CertaintyReporter interface {
	// Certainty returns the self-declared confidence tier.
	Certainty() Certainty
}

// ProcessorError wraps an error with the producing Processor's identity.
//
// ProcessorError is the standard error type returned by Processors when
// they want callers to be able to extract the producing Processor's
// name. Use errors.As to retrieve:
//
//	var pe *lexnorm.ProcessorError
//	if errors.As(err, &pe) {
//	    log.Printf("processor %s failed in %s: %v", pe.Name, pe.Op, pe.Err)
//	}
//
// # Construction
//
// Use WrapProcessorError:
//
//	return lexnorm.WrapProcessorError("alias", "match", err)
//
// Or build directly:
//
//	return &lexnorm.ProcessorError{
//	    Name: "alias",
//	    Op:   "match",
//	    Err:  err,
//	}
type ProcessorError struct {
	// Name is the Processor name (from Processor.Name()).
	Name string

	// Op is the operation phase within the Processor (e.g., "match",
	// "validate", "apply"). Free-form string; the engine does not
	// interpret it.
	Op string

	// Err is the wrapped underlying error.
	Err error
}

// Error implements the error interface.
func (e *ProcessorError) Error() string {
	if e == nil {
		return "<nil ProcessorError>"
	}
	if e.Op == "" {
		return fmt.Sprintf("processor %s: %v", e.Name, e.Err)
	}
	return fmt.Sprintf("processor %s in %s: %v", e.Name, e.Op, e.Err)
}

// Unwrap returns the underlying error, enabling errors.Is and errors.As.
func (e *ProcessorError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// WrapProcessorError wraps err with the Processor name and operation.
//
// If err is nil, WrapProcessorError returns nil (no wrapping needed).
// If err is already a *ProcessorError, WrapProcessorError returns it
// unchanged (avoid double-wrapping).
func WrapProcessorError(name, op string, err error) error {
	if err == nil {
		return nil
	}
	if pe, ok := err.(*ProcessorError); ok {
		return pe
	}
	return &ProcessorError{Name: name, Op: op, Err: err}
}
