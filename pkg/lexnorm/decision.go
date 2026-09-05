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

import "fmt"

// Decision is the disposition a Processor proposes for a candidate match.
//
// Decision is distinct from Action: Decision is the high-level intent
// (skip / suggest / apply); Action is the concrete change kind recorded
// in a Change record (replace / remove / suggest). Most callers use
// Decision for policy configuration (thresholds) and Action for Change
// inspection.
//
// # Mapping to Change
//
//   - DecisionApply  → Change.Action = ActionReplace | ActionRemove,
//     Change.Applied = true
//   - DecisionSuggest → Change.Action = ActionSuggest,
//     Change.Applied = false
//   - DecisionSkip   → no Change emitted
//
// # Backward Compatibility
//
// Future Decision values may be appended but existing constants must keep
// their numeric values. Reordering or removal is a major version bump.
type Decision uint8

const (
	// DecisionSkip indicates the candidate was rejected (no Change emitted).
	DecisionSkip Decision = iota

	// DecisionSuggest indicates the candidate is proposed but not applied
	// (Change.Action = ActionSuggest, Change.Applied = false).
	DecisionSuggest

	// DecisionApply indicates the candidate is accepted and the Change
	// was applied to the State.
	DecisionApply
)

// String returns a stable lowercase identifier for the Decision.
//
// The returned string is part of the public API and must not change
// without a major version bump.
func (d Decision) String() string {
	switch d {
	case DecisionSkip:
		return "skip"
	case DecisionSuggest:
		return "suggest"
	case DecisionApply:
		return "apply"
	}
	return fmt.Sprintf("Decision(%d)", uint8(d))
}

// IsZero reports whether d is the zero value (DecisionSkip).
func (d Decision) IsZero() bool {
	return d == DecisionSkip
}
