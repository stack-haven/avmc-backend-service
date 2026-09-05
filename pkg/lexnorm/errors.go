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

import "errors"

// Sentinel errors used across the engine.
//
// All sentinel errors are stable as of v1.0 and must not be renamed,
// removed, or have their semantics changed without a major version bump.
// New sentinel errors may be added in minor versions.
//
// See .agents/REVIEW.md (D5) for the full list of declared sentinels.
var (
	// ErrInvalidConfig indicates that configuration validation failed
	// at construction time (e.g., empty pipeline, illegal threshold,
	// NaN/Inf Confidence).
	//
	// Returned by:
	//   - New(opts ...Option) when any Option is illegal (D2 fail-fast).
	//   - Config.Validate() on invalid configuration.
	ErrInvalidConfig = errors.New("lexnorm: invalid config")

	// ErrInvalidSpan indicates that a span argument is out of range or
	// has invalid bounds (e.g., Start < 0, End > len(text), Start > End).
	//
	// Returned by State methods (Replace, Suggest, Lock) when the
	// span argument violates invariants.
	ErrInvalidSpan = errors.New("lexnorm: invalid span")

	// ErrConflict indicates a conflict in protected region management
	// (e.g., attempting to Replace a Locked span, or overlapping Replace
	// in a single Step).
	//
	// Returned by State.Replace when the target span overlaps a
	// previously Locked region.
	ErrConflict = errors.New("lexnorm: conflict")

	// ErrRuntime indicates that Runtime resolution failed (e.g.,
	// ProfileResolver returned an error, or the requested ProfileID is
	// not registered).
	//
	// Distinct from ErrInvalidConfig to allow callers to differentiate
	// between "bad configuration" and "bad runtime context".
	ErrRuntime = errors.New("lexnorm: runtime resolution failed")
)
