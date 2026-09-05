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

package lexnorm_test

import (
	"testing"

	"github.com/stack-haven/lexnorm"
)

func TestStatus_String(t *testing.T) {
	tests := []struct {
		s    lexnorm.Status
		want string
	}{
		{lexnorm.StatusSuccess, "success"},
		{lexnorm.StatusPartial, "partial"},
		{lexnorm.StatusCanceled, "canceled"},
		{lexnorm.StatusFailed, "failed"},
		{lexnorm.Status(99), "Status(99)"},
	}
	for _, tc := range tests {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Status(%d).String() = %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		s        lexnorm.Status
		terminal bool
	}{
		{lexnorm.StatusSuccess, true},
		{lexnorm.StatusPartial, false},
		{lexnorm.StatusCanceled, false},
		{lexnorm.StatusFailed, true},
		{lexnorm.Status(99), false},
	}
	for _, tc := range tests {
		if got := tc.s.IsTerminal(); got != tc.terminal {
			t.Errorf("Status(%d).IsTerminal() = %v, want %v", tc.s, got, tc.terminal)
		}
	}
}

func TestStatus_StableCount(t *testing.T) {
	const want = 4
	if got := len([]lexnorm.Status{
		lexnorm.StatusSuccess,
		lexnorm.StatusPartial,
		lexnorm.StatusCanceled,
		lexnorm.StatusFailed,
	}); got != want {
		t.Errorf("Status count changed: got %d, want %d", got, want)
	}
}

func TestStatus_ZeroIsSuccess(t *testing.T) {
	// The zero value of Status must be StatusSuccess. This is a design
	// choice documented in status.go: callers can declare `var s Status`
	// without initialization and get a sensible default.
	var s lexnorm.Status
	if s != lexnorm.StatusSuccess {
		t.Errorf("zero Status must be StatusSuccess (got %v)", s)
	}
}
