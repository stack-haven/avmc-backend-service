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

// ErrorPolicy controls how the Engine handles Processor errors.
//
// # Mapping to Result.Status
//
//   - ContinueOnError (default): errors are recorded in Result.Errors,
//     processing continues. Final Status is StatusSuccess (if no errors)
//     or StatusPartial.
//   - FailFast: the first Processor error stops the Pipeline. Final
//     Status is StatusCanceled (if context-related) or StatusFailed.
type ErrorPolicy uint8

const (
	// ContinueOnError continues processing all Processors even if some
	// fail. The Result.Status becomes StatusPartial if any error
	// occurred.
	ContinueOnError ErrorPolicy = iota

	// FailFast stops processing on the first Processor error. The
	// Result.Status becomes StatusFailed (or StatusCanceled if the
	// error is context.Canceled).
	FailFast
)

// String returns a stable lowercase identifier for the ErrorPolicy.
func (p ErrorPolicy) String() string {
	switch p {
	case ContinueOnError:
		return "continue"
	case FailFast:
		return "fail-fast"
	}
	return fmt.Sprintf("ErrorPolicy(%d)", uint8(p))
}

// IsZero reports whether p is the zero value (ContinueOnError).
func (p ErrorPolicy) IsZero() bool {
	return p == ContinueOnError
}
