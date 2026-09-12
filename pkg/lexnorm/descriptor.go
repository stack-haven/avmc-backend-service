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

import "encoding/json"

// Descriptor describes a Processor for dynamic registration via Registry.
//
// # Purpose
//
// Descriptor allows Processors to be constructed from configuration
// (e.g., JSON) at runtime, without compile-time imports of the
// Processor package. This enables:
//
//   - YAML / JSON configuration of Pipelines
//   - Plugin systems
//   - Dynamic registration (Register at startup, Get at runtime)
//
// # Independence
//
// Descriptor is a pure data type. It does not depend on Engine,
// Registry, or any specific Processor implementation.
//
// # Usage
//
// Each Processor package (e.g., processor/normalize) exposes a
// Descriptor value:
//
//	package normalize
//
//	var Descriptor = lexnorm.Descriptor{
//	    Name:      "normalize",
//	    Certainty: lexnorm.CertaintyHigh,
//	    New:       func(cfg json.RawMessage) (lexnorm.Processor, error) {
//	        return New(), nil
//	    },
//	    Default: func() any { return nil },
//	}
//
// Application code registers the Descriptor with a Registry:
//
//	reg := lexnorm.NewRegistry()
//	reg.Register(normalize.Descriptor)
//	reg.Register(alias.Descriptor)
//	...
//
// # Config
//
// The cfg parameter to New is a JSON-encoded configuration block. The
// Processor package is responsible for unmarshalling it. If cfg is
// nil or empty, the Processor should use its built-in defaults
// (typically via the Default function).
type Descriptor struct {
	// Name is the unique identifier used for Registry lookup.
	// Required. Must be unique within a Registry.
	Name string

	// Certainty is the self-declared confidence tier (CertaintyHigh /
	// CertaintyMedium / CertaintyLow). Used by application code that
	// orders Processors or filters them by tier.
	Certainty Certainty

	// New constructs a Processor from a JSON config block.
	//
	// Implementations should:
	//   - Return an error if cfg is malformed
	//   - Apply default values if cfg is empty / nil
	//   - Be safe to call concurrently (Registry is concurrent)
	New func(cfg json.RawMessage) (Processor, error)

	// Default returns the default configuration for this Processor.
	// Used by introspection tools and by Registry.Build when no
	// config is provided. May return nil if the Processor has no
	// configurable options.
	Default func() any

	// --- Capability metadata (optional; zero value = undeclared) ---
	//
	// The fields below describe WHAT a Processor does, not how to
	// construct it. They are consumed by Registry queries, audit
	// tooling, documentation generation, and default-Pipeline
	// assembly. They never override user-declared Pipeline order and
	// never gate runtime behavior.

	// Version is the semantic version of the Processor implementation.
	// Declaring it here is equivalent to implementing Versioner;
	// processors should keep the two in sync (Versioner wins when both
	// are present and differ, since the interface is authoritative).
	Version string

	// Category is the formal capability classification (Category*
	// constants). Zero value "" means undeclared.
	Category Category

	// MutatesText reports whether the Processor may modify State.Text.
	MutatesText bool

	// SupportsSuggest reports whether the Processor can produce
	// Suggestions (State.Suggest).
	SupportsSuggest bool

	// SupportsProtected reports whether the Processor's modifications
	// honor Protected Spans (State.Lock) — i.e., its Replace calls go
	// through State and are rejected on locked regions.
	SupportsProtected bool

	// Deterministic declares same-input/same-snapshot determinism.
	// For finer classification (probabilistic / generative), see
	// Determinism.
	Deterministic bool

	// Determinism is the finer determinism class (DeterministicTrue /
	// DeterministicProbabilistic / DeterministicGenerative). When set,
	// it refines the Deterministic bool.
	Determinism Determinism

	// DefaultOrder is the suggested position in the DEFAULT Pipeline
	// (1-based; 0 = undeclared). It is used for default Pipeline
	// construction and documentation display only. It MUST NOT be used
	// to reorder a user-declared Pipeline.
	DefaultOrder int

	// Description is a short human-readable summary for docs and
	// tooling. Optional.
	Description string
}

// Mutation returns the MutationMode implied by the declared capability
// flags:
//
//	MutatesText && SupportsSuggest → MutationMixed
//	MutatesText                    → MutationApply
//	SupportsSuggest                → MutationSuggest
//	otherwise                      → MutationNone
//
// This is a convenience view over the flags; the flags remain the
// source of truth.
func (d Descriptor) Mutation() MutationMode {
	switch {
	case d.MutatesText && d.SupportsSuggest:
		return MutationMixed
	case d.MutatesText:
		return MutationApply
	case d.SupportsSuggest:
		return MutationSuggest
	default:
		return MutationNone
	}
}
