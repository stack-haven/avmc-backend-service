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

// MutationMode declares how a Processor may change the State text.
//
// It is metadata declared on the Processor's Descriptor. The engine
// does not enforce it per-call (a Processor could still call
// State.Suggest even in MutationNone); it exists for documentation,
// audit tooling, and Pipeline composition hints.
type MutationMode string

const (
	// MutationNone declares the Processor never mutates State.Text.
	MutationNone MutationMode = "none"

	// MutationSuggest declares the Processor only produces Suggestions
	// (Change.Applied == false).
	MutationSuggest MutationMode = "suggest"

	// MutationApply declares the Processor only applies changes
	// (Change.Applied == true).
	MutationApply MutationMode = "apply"

	// MutationMixed declares the Processor may both apply and suggest,
	// depending on confidence thresholds or configuration.
	MutationMixed MutationMode = "mixed"
)

// Determinism declares whether a Processor produces identical results
// for identical (input, Runtime Snapshot, config) triples.
//
// It is metadata on the Descriptor. The determinism TEST requirement
// applies to every Processor declaring DeterministicTrue.
type Determinism string

const (
	// DeterministicTrue: same input + same snapshot + same config always
	// yields the same Result. All rule-based built-in Processors are in
	// this class.
	DeterministicTrue Determinism = "deterministic"

	// DeterministicProbabilistic: results are stable for fixed seeds or
	// fixed model states but may vary across environments (e.g., a local
	// ML model with sampling).
	DeterministicProbabilistic Determinism = "probabilistic"

	// DeterministicGenerative: results may legitimately vary between
	// identical calls (e.g., a hosted LLM). Processors in this class
	// must not be relied on for byte-stable replay.
	DeterministicGenerative Determinism = "generative"
)
