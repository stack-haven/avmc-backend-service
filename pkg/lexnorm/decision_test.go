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

func TestDecision_String(t *testing.T) {
	tests := []struct {
		d    lexnorm.Decision
		want string
	}{
		{lexnorm.DecisionSkip, "skip"},
		{lexnorm.DecisionSuggest, "suggest"},
		{lexnorm.DecisionApply, "apply"},
		{lexnorm.Decision(99), "Decision(99)"},
	}
	for _, tc := range tests {
		if got := tc.d.String(); got != tc.want {
			t.Errorf("Decision(%d).String() = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestDecision_IsZero(t *testing.T) {
	if !(lexnorm.DecisionSkip).IsZero() {
		t.Error("DecisionSkip must be the zero value")
	}
	if (lexnorm.DecisionSuggest).IsZero() {
		t.Error("DecisionSuggest must NOT be zero")
	}
	if (lexnorm.DecisionApply).IsZero() {
		t.Error("DecisionApply must NOT be zero")
	}
}

func TestDecision_StableCount(t *testing.T) {
	const want = 3
	if got := len([]lexnorm.Decision{
		lexnorm.DecisionSkip,
		lexnorm.DecisionSuggest,
		lexnorm.DecisionApply,
	}); got != want {
		t.Errorf("Decision count changed: got %d, want %d", got, want)
	}
}
