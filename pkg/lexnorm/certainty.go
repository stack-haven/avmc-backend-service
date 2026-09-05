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

// Certainty is a Processor-self-declared confidence tier.
//
// Certainty is intentionally coarse (3 levels): it serves as a hint for
// Standard Preset construction and decision grading, not as a per-call
// signal. Per-call confidence lives in Change.Confidence (float64).
//
// # 1.0 → 1.2 Simplification
//
// Earlier drafts (1.0) had 5 levels including CertaintyUnknown and
// CertaintyDeterministic. 1.2 simplifies to 3 levels because:
//
//   - CertaintyUnknown is meaningless in a declaration context
//     (Processors either declare or they don't).
//   - CertaintyDeterministic is semantically identical to CertaintyHigh
//     when sorted.
//
// # Backward Compatibility
//
// Future Certainty values may be appended but existing constants must keep
// their numeric values.
type Certainty uint8

const (
	// CertaintyHigh indicates the Processor produces deterministic,
	// low-risk modifications (e.g., Alias, Deterministic). Default for
	// most rule-based Processors.
	CertaintyHigh Certainty = iota

	// CertaintyMedium indicates the Processor produces plausible but
	// occasionally incorrect modifications (e.g., Pinyin, Fuzzy).
	CertaintyMedium

	// CertaintyLow indicates the Processor produces probabilistic
	// modifications that require human review (e.g., Context, LLM).
	CertaintyLow
)

// String returns a stable lowercase identifier for the Certainty.
func (c Certainty) String() string {
	switch c {
	case CertaintyHigh:
		return "high"
	case CertaintyMedium:
		return "medium"
	case CertaintyLow:
		return "low"
	}
	return fmt.Sprintf("Certainty(%d)", uint8(c))
}

// Rank returns a numeric rank for sorting (higher = more certain).
//
// Useful for deterministic ordering: a Higher Rank always wins a
// length-tied conflict.
//
//	Ranking (1.2):
//	  CertaintyHigh   → 3
//	  CertaintyMedium → 2
//	  CertaintyLow    → 1
//	  Unknown         → 0
func (c Certainty) Rank() int {
	switch c {
	case CertaintyHigh:
		return 3
	case CertaintyMedium:
		return 2
	case CertaintyLow:
		return 1
	default:
		return 0
	}
}
