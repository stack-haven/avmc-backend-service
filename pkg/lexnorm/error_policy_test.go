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

func TestErrorPolicy_String(t *testing.T) {
	tests := []struct {
		p    lexnorm.ErrorPolicy
		want string
	}{
		{lexnorm.ContinueOnError, "continue"},
		{lexnorm.FailFast, "fail-fast"},
		{lexnorm.ErrorPolicy(99), "ErrorPolicy(99)"},
	}
	for _, tc := range tests {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("ErrorPolicy(%d).String() = %q, want %q", tc.p, got, tc.want)
		}
	}
}

func TestErrorPolicy_IsZero(t *testing.T) {
	if !(lexnorm.ContinueOnError).IsZero() {
		t.Error("ContinueOnError must be the zero value")
	}
	if (lexnorm.FailFast).IsZero() {
		t.Error("FailFast must NOT be zero")
	}
}

func TestErrorPolicy_StableCount(t *testing.T) {
	const want = 2
	if got := len([]lexnorm.ErrorPolicy{
		lexnorm.ContinueOnError,
		lexnorm.FailFast,
	}); got != want {
		t.Errorf("ErrorPolicy count = %d, want %d", got, want)
	}
}
