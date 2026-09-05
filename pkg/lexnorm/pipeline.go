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
)

// Pipeline composes Processors into an ordered execution chain.
//
// # 1.2 Interface (D6 Decision)
//
// Pipeline was a concrete struct in earlier drafts. 1.2 promotes it to an
// interface so users can implement custom pipelines (e.g., conditional
// branching, parallel execution, retry logic).
//
// # Pipeline Is a Processor
//
// Pipeline satisfies Processor (architecture invariant I4). A Pipeline can
// therefore be nested inside another Pipeline:
//
//	inner := lexnorm.NewPipeline(procA, procB)
//	outer := lexnorm.NewPipeline(procC, inner, procD)
//
// # ErrorPolicy: ContinueOnError (built-in default)
//
// The built-in Pipeline implementation continues on Processor errors and
// joins them via errors.Join. To get fail-fast behavior:
//
//   - Call Processors() directly and stop on the first error, or
//   - Implement a custom Pipeline that does so.
//
// Engine.M5 will handle ErrorPolicy at the Engine level.
//
// # Immutability and Concurrency
//
// Pipelines are immutable after construction. A single Pipeline instance
// can be safely shared across goroutines; each Normalize call uses its
// own State.
type Pipeline interface {
	Processor

	// Processors returns the ordered list of Processors in this Pipeline.
	//
	// The returned slice is defensive: modifications do not affect the
	// Pipeline. Processors are invoked in slice order during Process.
	Processors() []Processor
}

// NewPipeline creates a Pipeline from the given Processors.
//
// An empty processor list produces a valid no-op Pipeline. The returned
// Pipeline implements Processor (so it can be nested in another Pipeline).
//
// The default ErrorPolicy is ContinueOnError (errors are joined and
// returned; all Processors are still called).
func NewPipeline(processors ...Processor) Pipeline {
	return &pipeline{processors: processors}
}

// pipeline is the default Pipeline implementation.
type pipeline struct {
	processors []Processor
}

// Name returns "pipeline", identifying this as a composite Processor.
//
// The Name is stable and used in:
//   - Result.StepTiming.Processor (when the Pipeline itself is wrapped
//     in a Step)
//   - log messages
//
// For nested pipelines, the inner Pipeline appears in StepTiming as
// "pipeline", not by its individual Processors' names.
func (p *pipeline) Name() string { return "pipeline" }

// Process executes each Processor in order, accumulating errors.
//
// # Behavior
//
//   - All Processors are invoked (ContinueOnError semantics).
//   - A Processor error does not stop the loop; it is appended to the
//     error list.
//   - The final return value is errors.Join(allErrors) or nil if all
//     Processors succeeded.
//   - ctx.Err() is checked before each Processor. If the context is
//     done, remaining Processors are skipped and ctx.Err() is appended.
//
// # State
//
// Processors mutate State via State.Replace / Suggest / Lock.
// Pipeline.Process itself does not modify State.
//
// # Concurrency
//
// Pipeline.Process is safe to call concurrently from multiple goroutines,
// provided each call uses its own *State.
func (p *pipeline) Process(ctx context.Context, s *State) error {
	var errs []error
	for _, proc := range p.processors {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if err := proc.Process(ctx, s); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Processors returns a defensive copy of the processor list.
//
// Modifications to the returned slice do not affect the Pipeline.
func (p *pipeline) Processors() []Processor {
	out := make([]Processor, len(p.processors))
	copy(out, p.processors)
	return out
}

// Version returns "" for the default Pipeline.
//
// Custom Pipeline implementations may implement Versioner (via a Version()
// method) to declare a semantic version, which Engine uses to populate
// Result.RuntimeInfo.PipelineVersion.
//
// The default empty string is intentional: a Pipeline is a thin
// composition, and its "effective version" is best derived from its
// constituent Processors by Engine.M5.
func (p *pipeline) Version() string { return "" }
