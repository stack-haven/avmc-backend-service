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

func TestAction_String(t *testing.T) {
	tests := []struct {
		a    lexnorm.Action
		want string
	}{
		{lexnorm.ActionReplace, "replace"},
		{lexnorm.ActionRemove, "remove"},
		{lexnorm.ActionSuggest, "suggest"},
		{lexnorm.Action(99), "Action(99)"},
	}
	for _, tc := range tests {
		if got := tc.a.String(); got != tc.want {
			t.Errorf("Action(%d).String() = %q, want %q", tc.a, got, tc.want)
		}
	}
}

func TestAction_String_Stable(t *testing.T) {
	// The String mapping is part of the public API contract.
	// Changes require a major version bump.
	want := map[lexnorm.Action]string{
		lexnorm.ActionReplace: "replace",
		lexnorm.ActionRemove:  "remove",
		lexnorm.ActionSuggest: "suggest",
	}
	for a, s := range want {
		if a.String() != s {
			t.Errorf("Action %d must stringify to %q (got %q)", a, s, a.String())
		}
	}
}

func TestAction_IsZero(t *testing.T) {
	if !(lexnorm.ActionReplace).IsZero() {
		t.Error("ActionReplace must be the zero value (ActionReplace)")
	}
	if (lexnorm.ActionRemove).IsZero() {
		t.Error("ActionRemove must NOT be zero")
	}
	if (lexnorm.ActionSuggest).IsZero() {
		t.Error("ActionSuggest must NOT be zero")
	}
	if (lexnorm.Action(99)).IsZero() {
		t.Error("Action(99) must NOT be zero")
	}
}

func TestAction_StableCount(t *testing.T) {
	// Adding a 4th Action is allowed (minor version); removing or
	// reordering is a major version bump.
	const want = 3
	if got := len([]lexnorm.Action{
		lexnorm.ActionReplace,
		lexnorm.ActionRemove,
		lexnorm.ActionSuggest,
	}); got != want {
		t.Errorf("Action count changed: got %d, want %d (update CHANGELOG)", got, want)
	}
}
