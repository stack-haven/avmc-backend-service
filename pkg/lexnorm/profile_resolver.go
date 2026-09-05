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
	"context"
	"fmt"

	"github.com/stack-haven/lexnorm/lexicon"
)

// ProfileResolver maps a ProfileID to its Runtime context.
//
// # 1.1 Introduction (D6 / 1.1 §4.6)
//
// ProfileResolver enables multi-Profile usage from a single Engine
// instance, avoiding the cost of one Engine per Profile.
//
// # Implementation
//
// The Resolver is application-defined (it lives outside the core package
// and reads from HR systems, configuration, databases, etc.). The Engine
// never assumes a particular source.
//
// # Contract
//
// Resolve must return a Runtime that is:
//
//   - Complete: all fields populated (Lexicon, Pipeline, Config non-nil).
//   - Immutable: the returned Runtime (and its components) MUST NOT be
//     mutated after Resolve returns. The Engine uses the Runtime as an
//     immutable snapshot for the duration of the Normalize call.
//   - Deterministic: same ProfileID + same underlying data must return
//     equivalent Runtimes (architecture invariant I9).
//
// # Concurrency
//
// Resolve may be called concurrently from multiple goroutines (one per
// Normalize call). Implementations must be safe for concurrent use.
type ProfileResolver interface {
	// Resolve returns the Runtime for the given ProfileID.
	//
	// Returns:
	//   - (*Runtime, nil) on success.
	//   - (nil, ErrRuntime) when the ProfileID is unknown or the
	//     underlying source is unavailable.
	//   - (nil, other err) for unexpected failures; the Engine will
	//     surface the error to the caller.
	Resolve(ctx context.Context, id ProfileID) (*Runtime, error)
}

// StaticResolver is a simple in-memory ProfileResolver for tests and
// simple deployments.
//
// StaticResolver is safe for concurrent read access (the map is not
// mutated after construction). For dynamic resolution, implement
// ProfileResolver directly.
type StaticResolver struct {
	profiles map[ProfileID]*Runtime
}

// NewStaticResolver creates a StaticResolver from a map of ProfileID → ProfileBundle.
//
// Returns an error if the map is empty or any Runtime is invalid.
func NewStaticResolver(profiles map[ProfileID]ProfileBundle) (*StaticResolver, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("static resolver requires at least one profile: %w", ErrInvalidConfig)
	}
	resolved := make(map[ProfileID]*Runtime, len(profiles))
	for id, bundle := range profiles {
		if err := bundle.validate(); err != nil {
			return nil, fmt.Errorf("profile %s: %w", id, err)
		}
		resolved[id] = newRuntimeFromBundle(id, bundle)
	}
	return &StaticResolver{profiles: resolved}, nil
}

// Resolve returns the Runtime for the given ProfileID.
func (r *StaticResolver) Resolve(_ context.Context, id ProfileID) (*Runtime, error) {
	rt, ok := r.profiles[id]
	if !ok {
		return nil, fmt.Errorf("profile %q not found: %w", id, ErrRuntime)
	}
	return rt, nil
}

// ProfileBundle groups the resources required for a single Profile.
//
// ProfileBundle is the user-facing container for Profile → Resource mapping
// in single-profile (WithProfiles) and StaticResolver setups. It is also
// the format returned by application ProfileResolver implementations.
type ProfileBundle struct {
	// Lexicon is the knowledge container for this Profile. Required.
	Lexicon lexicon.Lexicon

	// Pipeline is the composition of Processors for this Profile.
	// Required.
	Pipeline Pipeline

	// Config is the per-Profile configuration. If zero, DefaultConfig()
	// is used by Engine at runtime.
	Config Config
}

// validate ensures the bundle is complete.
func (b ProfileBundle) validate() error {
	if b.Lexicon == nil {
		return fmt.Errorf("nil Lexicon: %w", ErrInvalidConfig)
	}
	if b.Pipeline == nil {
		return fmt.Errorf("nil Pipeline: %w", ErrInvalidConfig)
	}
	cfg := b.resolvedConfig()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid Config: %w", err)
	}
	return nil
}

// resolvedConfig returns the effective Config, defaulting if zero.
func (b ProfileBundle) resolvedConfig() Config {
	if b.Config == (Config{}) {
		return DefaultConfig()
	}
	return b.Config
}
