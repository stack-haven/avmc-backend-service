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

// Action describes the kind of text modification recorded in a Change.
//
// Action is set by the Processor that produced the change. It is distinct
// from Change.Applied: Action describes WHAT was done; Applied records
// WHETHER it was actually applied to the State.
//
// # Mapping to Decision
//
//   - ActionReplace / ActionRemove + Applied=true  ↔ DecisionApply
//   - ActionSuggest + Applied=false                ↔ DecisionSuggest
//   - No Change emitted                            ↔ DecisionSkip
//
// # Backward Compatibility
//
// Future Action values may be appended but existing constants must keep
// their numeric values (see [Decision] for the same invariant).
type Action uint8

const (
	// ActionReplace records a substitution: the Span [Start, End) in the
	// Original text was replaced with Change.To.
	ActionReplace Action = iota

	// ActionRemove records a deletion: the Span [Start, End) in the
	// Original text was removed. Change.To is the empty string.
	ActionRemove

	// ActionSuggest records a proposal that was NOT applied (Change.Applied
	// is false). The Caller may decide to apply it out-of-band.
	ActionSuggest
)

// String returns a stable lowercase identifier for the Action.
//
// The returned string is part of the public API (used in logs, Result
// serialization, and debug output) and must not change without a major
// version bump.
func (a Action) String() string {
	switch a {
	case ActionReplace:
		return "replace"
	case ActionRemove:
		return "remove"
	case ActionSuggest:
		return "suggest"
	}
	return fmt.Sprintf("Action(%d)", uint8(a))
}

// IsZero reports whether a is the zero value (ActionReplace).
func (a Action) IsZero() bool {
	return a == ActionReplace
}
