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

func TestCertainty_String(t *testing.T) {
	tests := []struct {
		c    lexnorm.Certainty
		want string
	}{
		{lexnorm.CertaintyHigh, "high"},
		{lexnorm.CertaintyMedium, "medium"},
		{lexnorm.CertaintyLow, "low"},
		{lexnorm.Certainty(99), "Certainty(99)"},
	}
	for _, tc := range tests {
		if got := tc.c.String(); got != tc.want {
			t.Errorf("Certainty(%d).String() = %q, want %q", tc.c, got, tc.want)
		}
	}
}

func TestCertainty_Rank(t *testing.T) {
	tests := []struct {
		c    lexnorm.Certainty
		want int
	}{
		{lexnorm.CertaintyHigh, 3},
		{lexnorm.CertaintyMedium, 2},
		{lexnorm.CertaintyLow, 1},
		{lexnorm.Certainty(99), 0},
		{lexnorm.Certainty(0), 3}, // CertaintyHigh is the zero value
	}
	for _, tc := range tests {
		if got := tc.c.Rank(); got != tc.want {
			t.Errorf("Certainty(%d).Rank() = %d, want %d", tc.c, got, tc.want)
		}
	}
}

func TestCertainty_Rank_Order(t *testing.T) {
	// Higher rank = more certain. This is the contract used by Match
	// conflict resolution (D4: Longest → Higher Priority → Lex).
	if lexnorm.CertaintyHigh.Rank() <= lexnorm.CertaintyMedium.Rank() {
		t.Error("CertaintyHigh.Rank() must exceed CertaintyMedium.Rank()")
	}
	if lexnorm.CertaintyMedium.Rank() <= lexnorm.CertaintyLow.Rank() {
		t.Error("CertaintyMedium.Rank() must exceed CertaintyLow.Rank()")
	}
}

func TestCertainty_StableCount(t *testing.T) {
	// 1.2 simplification: 5 levels (1.0) → 3 levels.
	// CertaintyUnknown and CertaintyDeterministic were dropped.
	const want = 3
	if got := len([]lexnorm.Certainty{
		lexnorm.CertaintyHigh,
		lexnorm.CertaintyMedium,
		lexnorm.CertaintyLow,
	}); got != want {
		t.Errorf("Certainty count changed: got %d, want %d (1.2 simplifies from 1.0's 5)", got, want)
	}
}
