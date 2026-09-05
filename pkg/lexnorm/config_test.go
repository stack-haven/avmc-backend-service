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
	"math"
	"testing"

	"github.com/stack-haven/lexnorm"
)

func TestDefaultConfig_Valid(t *testing.T) {
	cfg := lexnorm.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("DefaultConfig must be valid, got error: %v", err)
	}
	if cfg.AutoApplyThreshold != 0.95 {
		t.Errorf("default AutoApplyThreshold = %v, want 0.95", cfg.AutoApplyThreshold)
	}
	if cfg.SuggestThreshold != 0.65 {
		t.Errorf("default SuggestThreshold = %v, want 0.65", cfg.SuggestThreshold)
	}
	if cfg.DefaultErrorPolicy != lexnorm.ContinueOnError {
		t.Errorf("default DefaultErrorPolicy = %v, want ContinueOnError", cfg.DefaultErrorPolicy)
	}
	if cfg.MaxTextBytes != 0 {
		t.Errorf("default MaxTextBytes = %d, want 0 (unlimited)", cfg.MaxTextBytes)
	}
}

func TestConfig_Validate_Valid(t *testing.T) {
	tests := []struct {
		name string
		cfg  lexnorm.Config
	}{
		{"default", lexnorm.DefaultConfig()},
		{"zero thresholds", lexnorm.Config{}},
		{"thresholds equal", lexnorm.Config{AutoApplyThreshold: 0.5, SuggestThreshold: 0.5}},
		{"max text", lexnorm.Config{MaxTextBytes: 1024 * 1024}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}

func TestConfig_Validate_AutoApplyThreshold(t *testing.T) {
	tests := []struct {
		name string
		v    float64
	}{
		{"negative", -0.01},
		{"above 1", 1.01},
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"-Inf", math.Inf(-1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := lexnorm.DefaultConfig()
			cfg.AutoApplyThreshold = tc.v
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error for AutoApplyThreshold = %v", tc.v)
			}
			if !errors.Is(err, lexnorm.ErrInvalidConfig) {
				t.Errorf("error must wrap ErrInvalidConfig, got %v", err)
			}
		})
	}
}

func TestConfig_Validate_SuggestThreshold(t *testing.T) {
	cfg := lexnorm.DefaultConfig()
	cfg.SuggestThreshold = -0.01
	if err := cfg.Validate(); !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("SuggestThreshold < 0 must return ErrInvalidConfig, got %v", err)
	}

	cfg = lexnorm.DefaultConfig()
	cfg.SuggestThreshold = math.NaN()
	if err := cfg.Validate(); !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("SuggestThreshold NaN must return ErrInvalidConfig, got %v", err)
	}
}

func TestConfig_Validate_Ordering(t *testing.T) {
	cfg := lexnorm.DefaultConfig()
	cfg.AutoApplyThreshold = 0.5
	cfg.SuggestThreshold = 0.8 // > AutoApplyThreshold
	err := cfg.Validate()
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("SuggestThreshold > AutoApplyThreshold must return ErrInvalidConfig, got %v", err)
	}
}

func TestConfig_Validate_MaxTextBytes(t *testing.T) {
	cfg := lexnorm.DefaultConfig()
	cfg.MaxTextBytes = -1
	if err := cfg.Validate(); !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("MaxTextBytes < 0 must return ErrInvalidConfig, got %v", err)
	}
}

func TestConfig_NoSilentClamp(t *testing.T) {
	// Per D2 / NaN handling: validate must NOT silently clamp illegal
	// values to legal ones.
	cfg := lexnorm.Config{AutoApplyThreshold: 5.0}
	if err := cfg.Validate(); err == nil {
		t.Error("AutoApplyThreshold = 5.0 must fail validation (no silent clamp)")
	}
}
