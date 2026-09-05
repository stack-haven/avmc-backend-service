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

import (
	"fmt"
	"math"
)

// Config carries the per-call (or per-engine) configuration for
// normalization.
//
// # M3 Scope
//
// Config currently exposes:
//   - AutoApplyThreshold / SuggestThreshold (for future Processors)
//   - DefaultErrorPolicy (used when not overridden per call)
//   - MaxTextBytes (input size limit)
//
// More fields will be added in M5/M10 as needed.
//
// # Validation
//
// Config.Validate() enforces all invariants. Validation failures return
// errors that wrap ErrInvalidConfig (via fmt.Errorf %w), enabling
// errors.Is detection.
//
// # NaN / Inf Handling
//
// NaN and ±Inf are explicitly rejected for all float fields, even when
// they would technically satisfy the range check. This avoids silent
// downstream issues from special float values.
type Config struct {
	// AutoApplyThreshold is the Confidence value at or above which a
	// Decision becomes Apply. Must satisfy 0 ≤ value ≤ 1.
	//
	// Defaults to 0.95 via DefaultConfig.
	AutoApplyThreshold float64

	// SuggestThreshold is the Confidence value at or above which a
	// Decision becomes Suggest. Must satisfy 0 ≤ SuggestThreshold ≤
	// AutoApplyThreshold.
	//
	// Defaults to 0.65 via DefaultConfig.
	SuggestThreshold float64

	// DefaultErrorPolicy is the error policy used when not overridden
	// per call. Defaults to ContinueOnError.
	DefaultErrorPolicy ErrorPolicy

	// MaxTextBytes limits the input text length passed to NewState.
	// 0 means unlimited. Negative values are invalid.
	MaxTextBytes int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		AutoApplyThreshold: 0.95,
		SuggestThreshold:   0.65,
		DefaultErrorPolicy: ContinueOnError,
		MaxTextBytes:       0, // unlimited
	}
}

// Validate enforces Config invariants. Returns nil when valid.
//
// On failure, returns an error wrapping ErrInvalidConfig. Callers should
// use errors.Is to detect this:
//
//	if err := cfg.Validate(); err != nil {
//	    if errors.Is(err, lexnorm.ErrInvalidConfig) { ... }
//	}
func (c Config) Validate() error {
	// AutoApplyThreshold
	if err := validateUnitInterval(c.AutoApplyThreshold, "AutoApplyThreshold"); err != nil {
		return err
	}
	// SuggestThreshold
	if err := validateUnitInterval(c.SuggestThreshold, "SuggestThreshold"); err != nil {
		return err
	}
	// Ordering: SuggestThreshold ≤ AutoApplyThreshold.
	if c.SuggestThreshold > c.AutoApplyThreshold {
		return fmt.Errorf(
			"SuggestThreshold (%v) > AutoApplyThreshold (%v): %w",
			c.SuggestThreshold, c.AutoApplyThreshold, ErrInvalidConfig,
		)
	}
	// MaxTextBytes
	if c.MaxTextBytes < 0 {
		return fmt.Errorf("MaxTextBytes (%d) < 0: %w", c.MaxTextBytes, ErrInvalidConfig)
	}
	return nil
}

// validateUnitInterval checks that v is a real number in [0, 1].
// NaN and ±Inf are rejected explicitly.
func validateUnitInterval(v float64, name string) error {
	if math.IsNaN(v) {
		return fmt.Errorf("%s is NaN: %w", name, ErrInvalidConfig)
	}
	if math.IsInf(v, 0) {
		return fmt.Errorf("%s is %v: %w", name, v, ErrInvalidConfig)
	}
	if v < 0 || v > 1 {
		return fmt.Errorf("%s (%v) out of [0,1]: %w", name, v, ErrInvalidConfig)
	}
	return nil
}
