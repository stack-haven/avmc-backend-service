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

// Status describes the overall outcome of a Normalize call.
//
// # Mapping from Failure Modes
//
//   - All Processors succeeded, no errors  → StatusSuccess
//   - Some Processors failed (ErrorPolicy = ContinueOnError)
//     → StatusPartial
//   - Context canceled during processing   → StatusCanceled
//   - Fundamental failure (missing Lexicon,
//     ProfileResolver returned an error,
//     etc.)                                → StatusFailed
//
// # Backward Compatibility
//
// Future Status values may be appended but existing constants must keep
// their numeric values. The numeric zero is StatusSuccess (the zero value
// is the success status, by design).
type Status uint8

const (
	// StatusSuccess indicates all Processors completed without errors.
	StatusSuccess Status = iota

	// StatusPartial indicates some Processors failed but the overall
	// normalization produced a usable result. The Text is preserved
	// (see invariant I10) and Result.Errors contains the failures.
	StatusPartial

	// StatusCanceled indicates the call was canceled via context.
	// Result.Text contains the partial work done before cancellation.
	StatusCanceled

	// StatusFailed indicates a fundamental failure that prevents forming
	// a useful result (e.g., Lexicon missing, ProfileResolver error).
	// Use this sparingly: prefer StatusPartial when Text can still be
	// produced.
	StatusFailed
)

// String returns a stable lowercase identifier for the Status.
func (s Status) String() string {
	switch s {
	case StatusSuccess:
		return "success"
	case StatusPartial:
		return "partial"
	case StatusCanceled:
		return "canceled"
	case StatusFailed:
		return "failed"
	}
	return fmt.Sprintf("Status(%d)", uint8(s))
}

// IsTerminal reports whether s represents a final, non-recoverable state.
//
//   - StatusSuccess → true (the desired outcome)
//   - StatusFailed  → true (fundamental failure)
//   - StatusPartial → false (work may continue or be retried)
//   - StatusCanceled → false (caller may retry with a fresh context)
func (s Status) IsTerminal() bool {
	return s == StatusSuccess || s == StatusFailed
}
