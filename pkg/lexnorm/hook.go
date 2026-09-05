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

import "time"

// EventType identifies the kind of Hook event.
//
// Future EventTypes may be appended but existing constants must keep
// their numeric values.
type EventType uint8

const (
	// EventPipelineStart fires before any Processor in the Pipeline
	// runs (after Runtime is resolved and State is constructed).
	EventPipelineStart EventType = iota

	// EventProcessorStart fires immediately before each Processor.Process
	// call. Useful for fine-grained tracing.
	EventProcessorStart

	// EventProcessorEnd fires immediately after each Processor.Process
	// call (regardless of error). Useful for per-Processor timing and
	// error tracking.
	EventProcessorEnd

	// EventPipelineEnd fires after all Processors have completed
	// (or failed), and the Result is fully populated.
	EventPipelineEnd
)

// String returns a stable lowercase identifier for the EventType.
func (t EventType) String() string {
	switch t {
	case EventPipelineStart:
		return "pipeline-start"
	case EventProcessorStart:
		return "processor-start"
	case EventProcessorEnd:
		return "processor-end"
	case EventPipelineEnd:
		return "pipeline-end"
	}
	return "unknown"
}

// Event is the payload passed to Hook functions.
//
// Events are read-only: Hook functions MUST NOT modify the State, Runtime,
// or Result. Hook functions that return errors (if implemented via the
// future error-returning Hook signature) are ignored by the Engine.
type Event struct {
	// Type identifies the kind of event.
	Type EventType

	// State is the per-request State. Read-only for Hook.
	State *State

	// Runtime is the immutable Runtime for this call. Read-only for Hook.
	Runtime *Runtime

	// Result is the final Result. Populated for EventPipelineEnd.
	// Nil for EventPipelineStart, EventProcessorStart, EventProcessorEnd.
	Result *Result

	// Processor is the Processor.Name() for per-Processor events.
	// Empty for EventPipelineStart and EventPipelineEnd.
	Processor string

	// ProcessorVersion is the Processor.Version() for per-Processor
	// events. Empty if the Processor does not implement Versioner.
	ProcessorVersion string

	// Error is the Processor-level error (if any). For
	// EventProcessorEnd and EventPipelineEnd.
	Error error

	// Duration is the wall-clock time spent. For EventPipelineStart,
	// this is the time elapsed so far. For EventProcessorEnd, this
	// is the time spent in that Processor.
	Duration time.Duration
}

// Hook is the user-supplied observability callback.
//
// Hooks are called synchronously in registration order. They MUST NOT
// block for extended periods (use a goroutine internally if needed).
// They MUST NOT modify the Event's State, Runtime, or Result.
//
// # Errors and Panics
//
// Hook panics are caught by the surrounding defer (M10 feature); for
// now, Hook implementations should not panic.
type Hook func(e Event)

// triggerHooks invokes all registered hooks for an event.
//
// Hooks are called synchronously in registration order. A hook that
// blocks delays the entire Normalize call.
func triggerHooks(hooks []Hook, e Event) {
	for _, h := range hooks {
		h(e)
	}
}
