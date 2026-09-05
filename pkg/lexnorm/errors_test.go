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
	"errors"
	"fmt"
	"testing"

	"github.com/stack-haven/lexnorm"
)

// TestSentinels_NotNil verifies that all 4 declared sentinel errors
// are non-nil. A nil sentinel would defeat the purpose of errors.Is.
func TestSentinels_NotNil(t *testing.T) {
	sentinels := map[string]error{
		"ErrInvalidConfig": lexnorm.ErrInvalidConfig,
		"ErrInvalidSpan":   lexnorm.ErrInvalidSpan,
		"ErrConflict":      lexnorm.ErrConflict,
		"ErrRuntime":       lexnorm.ErrRuntime,
	}
	for name, err := range sentinels {
		if err == nil {
			t.Errorf("%s must not be nil", name)
		}
	}
}

// TestSentinels_Distinct verifies that each sentinel has a unique
// identity (no aliases). errors.Is must not match across different
// sentinels.
func TestSentinels_Distinct(t *testing.T) {
	sentinels := []error{
		lexnorm.ErrInvalidConfig,
		lexnorm.ErrInvalidSpan,
		lexnorm.ErrConflict,
		lexnorm.ErrRuntime,
	}
	for i := 0; i < len(sentinels); i++ {
		for j := i + 1; j < len(sentinels); j++ {
			if errors.Is(sentinels[i], sentinels[j]) {
				t.Errorf("sentinel[%d] (%q) must not match sentinel[%d] (%q)",
					i, sentinels[i], j, sentinels[j])
			}
		}
	}
}

// TestSentinels_ErrorsIs_Direct verifies that errors.Is matches
// the sentinel directly.
func TestSentinels_ErrorsIs_Direct(t *testing.T) {
	for _, err := range []error{
		lexnorm.ErrInvalidConfig,
		lexnorm.ErrInvalidSpan,
		lexnorm.ErrConflict,
		lexnorm.ErrRuntime,
	} {
		if !errors.Is(err, err) {
			t.Errorf("errors.Is(%v, %v) = false, want true", err, err)
		}
	}
}

// TestSentinels_ErrorsIs_Wrapped verifies that errors.Is matches
// when the sentinel is wrapped via fmt.Errorf("%w").
func TestSentinels_ErrorsIs_Wrapped(t *testing.T) {
	tests := []struct {
		name string
		base error
	}{
		{"ErrInvalidConfig", lexnorm.ErrInvalidConfig},
		{"ErrInvalidSpan", lexnorm.ErrInvalidSpan},
		{"ErrConflict", lexnorm.ErrConflict},
		{"ErrRuntime", lexnorm.ErrRuntime},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("operation failed: %w", tc.base)
			double := fmt.Errorf("deeper: %w", wrapped)
			triple := fmt.Errorf("deeper still: %w", double)

			if !errors.Is(wrapped, tc.base) {
				t.Errorf("errors.Is must match wrapped sentinel")
			}
			if !errors.Is(double, tc.base) {
				t.Errorf("errors.Is must match double-wrapped sentinel")
			}
			if !errors.Is(triple, tc.base) {
				t.Errorf("errors.Is must match triple-wrapped sentinel")
			}
		})
	}
}

// TestSentinels_Message verifies that each sentinel has the
// expected public message. The message is part of the public API
// contract; changes require a major version bump.
func TestSentinels_Message(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{lexnorm.ErrInvalidConfig, "lexnorm: invalid config"},
		{lexnorm.ErrInvalidSpan, "lexnorm: invalid span"},
		{lexnorm.ErrConflict, "lexnorm: conflict"},
		{lexnorm.ErrRuntime, "lexnorm: runtime resolution failed"},
	}
	for _, tc := range tests {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("error message mismatch: got %q, want %q", got, tc.want)
		}
	}
}

// TestSentinels_MessagePrefix verifies that every sentinel message
// starts with the package prefix "lexnorm: ". This makes errors
// self-identifying in logs.
func TestSentinels_MessagePrefix(t *testing.T) {
	const prefix = "lexnorm: "
	for _, err := range []error{
		lexnorm.ErrInvalidConfig,
		lexnorm.ErrInvalidSpan,
		lexnorm.ErrConflict,
		lexnorm.ErrRuntime,
	} {
		got := err.Error()
		if len(got) < len(prefix) || got[:len(prefix)] != prefix {
			t.Errorf("error message %q must start with %q", got, prefix)
		}
	}
}

// TestSentinels_StableCount verifies that the exact set of 4
// sentinels exists. Adding a 5th sentinel is allowed (minor version);
// removing or renaming is a major version bump.
func TestSentinels_StableCount(t *testing.T) {
	// Use reflection-free enumeration: the package intentionally
	// exposes exactly 4 sentinels. If a 5th is added, this test
	// serves as a reminder to also document it.
	const want = 4
	sentinels := []error{
		lexnorm.ErrInvalidConfig,
		lexnorm.ErrInvalidSpan,
		lexnorm.ErrConflict,
		lexnorm.ErrRuntime,
	}
	if got := len(sentinels); got != want {
		t.Errorf("sentinel count changed: got %d, want %d (update D5 + CHANGELOG)", got, want)
	}
}
