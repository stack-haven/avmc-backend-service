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

	"github.com/stack-haven/lexnorm/lexicon"
)

// Option configures an Engine at construction time.
//
// Options follow the Functional Options pattern. Each With* function
// returns an Option that mutates the engine's internal config.
//
// # Validation
//
// All Options are applied during New(); invalid combinations (e.g.,
// both WithProfileResolver and WithProfiles) produce ErrInvalidConfig
// from New.
//
// # Mutually Exclusive Modes
//
// The Engine supports three configuration modes:
//
//   - Single-profile: WithLexicon + WithPipeline
//   - Multi-profile:  WithProfiles (+ optional WithDefaultProfile)
//   - Dynamic:        WithProfileResolver
//
// Exactly one mode must be selected. Mixing modes produces ErrInvalidConfig.
type Option interface {
	apply(*engineConfig)
}

// optionFunc adapts a plain function to the Option interface.
type optionFunc func(*engineConfig)

func (f optionFunc) apply(c *engineConfig) { f(c) }

// engineConfig is the internal configuration built up by Options before
// New validates it and constructs the Engine.
type engineConfig struct {
	// Single-profile mode (mutually exclusive with profiles/resolver).
	lex  lexicon.Lexicon
	pipe Pipeline

	// Multi-profile mode (mutually exclusive with single/resolver).
	profiles       map[ProfileID]ProfileBundle
	defaultProfile ProfileID
	hasProfiles    bool

	// Dynamic mode (mutually exclusive with single/profiles).
	resolver ProfileResolver

	// HA mode (mutually exclusive with single/profiles/resolver/preset).
	// When set, Lexicon is captured from store.Current() at each call.
	lexiconStore *lexicon.Store

	// Preset mode (mutually exclusive with the four above).
	// Sets Pipeline + Config atomically from a named Preset.
	preset    *Preset
	hasPreset bool

	// Cross-cutting.
	middleware  []Middleware
	hooks       []Hook
	errorPolicy ErrorPolicy
	config      Config
	hasConfig   bool
}

// defaultEngineConfig returns the zero-state engineConfig with defaults.
func defaultEngineConfig() engineConfig {
	return engineConfig{
		errorPolicy: ContinueOnError,
		config:      DefaultConfig(),
		hasConfig:   true,
	}
}

// validate checks that the engineConfig is consistent.
//
// Returns ErrInvalidConfig (wrapped) on any violation.
func (c *engineConfig) validate() error {
	modes := 0
	if c.lexiconStore != nil {
		modes++
	}
	if !c.hasPreset && c.lex != nil {
		modes++
	}
	if c.hasProfiles {
		modes++
	}
	if c.resolver != nil {
		modes++
	}
	if c.hasPreset {
		modes++
	}
	if modes == 0 {
		return fmt.Errorf("Engine requires one of: WithLexicon+WithPipeline, WithProfiles, WithProfileResolver, WithLexiconStore+WithPipeline: %w", ErrInvalidConfig)
	}
	if modes > 1 {
		return fmt.Errorf("Engine modes are mutually exclusive (single/multi/dynamic/store): %w", ErrInvalidConfig)
	}

	// Single-profile validation: WithLexicon requires WithPipeline.
	if c.lexiconStore == nil && c.lex != nil {
		if c.pipe == nil {
			return fmt.Errorf("single-profile mode: WithPipeline required: %w", ErrInvalidConfig)
		}
	}

	// Store mode validation: Pipeline required; Lexicon must not be set.
	if c.lexiconStore != nil {
		if c.pipe == nil {
			return fmt.Errorf("Store mode: WithPipeline required: %w", ErrInvalidConfig)
		}
		if c.lex != nil {
			return fmt.Errorf("Store mode: WithLexicon must not be set (use WithLexiconStore): %w", ErrInvalidConfig)
		}
	}

	// Multi-profile validation.
	if c.hasProfiles {
		if len(c.profiles) == 0 {
			return fmt.Errorf("multi-profile mode: WithProfiles map must be non-empty: %w", ErrInvalidConfig)
		}
		if c.defaultProfile != "" {
			if _, ok := c.profiles[c.defaultProfile]; !ok {
				return fmt.Errorf("WithDefaultProfile %q not in profiles map: %w", c.defaultProfile, ErrInvalidConfig)
			}
		}
		for id, b := range c.profiles {
			if err := b.validate(); err != nil {
				return fmt.Errorf("profile %s: %w", id, err)
			}
		}
	}

	// Base config validation.
	if c.hasConfig {
		if err := c.config.Validate(); err != nil {
			return fmt.Errorf("invalid Config: %w", err)
		}
	}

	return nil
}

// ----------------------------------------------------------------------------
// Functional Options
// ----------------------------------------------------------------------------

// WithLexicon sets the Lexicon for single-profile mode.
//
// Must be paired with WithPipeline. Mutually exclusive with WithProfiles
// and WithProfileResolver.
func WithLexicon(lex lexicon.Lexicon) Option {
	return optionFunc(func(c *engineConfig) { c.lex = lex })
}

// WithPipeline sets the Pipeline for single-profile mode.
//
// Must be paired with WithLexicon. Mutually exclusive with WithProfiles
// and WithProfileResolver.
func WithPipeline(p Pipeline) Option {
	return optionFunc(func(c *engineConfig) { c.pipe = p })
}

// WithProfiles sets multiple named profiles (multi-profile mode).
//
// Mutually exclusive with single-profile mode and WithProfileResolver.
// Use WithDefaultProfile to choose which profile is used when a
// Normalize call does not specify a CallOption.
func WithProfiles(profiles map[ProfileID]ProfileBundle) Option {
	return optionFunc(func(c *engineConfig) {
		c.profiles = profiles
		c.hasProfiles = true
	})
}

// WithDefaultProfile sets the default ProfileID for multi-profile mode.
//
// The ID must exist in the map passed to WithProfiles.
func WithDefaultProfile(id ProfileID) Option {
	return optionFunc(func(c *engineConfig) { c.defaultProfile = id })
}

// WithProfileResolver sets a dynamic resolver for ProfileID → Runtime.
//
// Mutually exclusive with single-profile mode, WithProfiles, and
// WithLexiconStore.
func WithProfileResolver(r ProfileResolver) Option {
	return optionFunc(func(c *engineConfig) { c.resolver = r })
}

// WithLexiconStore configures the Engine for HA mode using a Lexicon
// Store (see lexicon.Store).
//
// # Behavior
//
// Each Normalize call captures the Store's current Lexicon snapshot
// (via Store.Current()) at the start of the call. The captured Lexicon
// is used for the duration of the call (Request Consistency,
// architecture invariant I8): concurrent Store updates do NOT
// affect in-flight Normalize calls.
//
// # Required Companion Option
//
// WithPipeline must also be provided to specify the processing chain.
//
// # Last Known Good (LKG)
//
// If Store.TryUpdate fails, the previous Lexicon remains in effect.
// The Engine therefore benefits from Store's LKG semantics
// automatically.
//
// # Mutually Exclusive With
//
// WithLexicon, WithProfiles, WithProfileResolver.
func WithLexiconStore(s *lexicon.Store) Option {
	return optionFunc(func(c *engineConfig) { c.lexiconStore = s })
}

// WithErrorPolicy sets the default ErrorPolicy.
//
// Default is ContinueOnError if not specified.
func WithErrorPolicy(p ErrorPolicy) Option {
	return optionFunc(func(c *engineConfig) { c.errorPolicy = p })
}

// WithMiddleware appends Middleware to the engine's middleware chain.
//
// Middleware are applied outermost-first (the first one passed is the
// outermost layer). All Middleware are optional; multiple can be passed.
func WithMiddleware(mw ...Middleware) Option {
	return optionFunc(func(c *engineConfig) { c.middleware = append(c.middleware, mw...) })
}

// WithHooks appends Hook functions for observability events.
//
// Multiple Hooks can be registered; they are called in registration
// order on each event.
func WithHooks(h ...Hook) Option {
	return optionFunc(func(c *engineConfig) { c.hooks = append(c.hooks, h...) })
}

// WithConfig sets the base Config applied to single-profile mode and
// to any Profile in multi-profile mode whose Config is zero.
//
// Per-Profile Config takes precedence over this base.
func WithConfig(cfg Config) Option {
	return optionFunc(func(c *engineConfig) {
		c.config = cfg
		c.hasConfig = true
	})
}

// WithPreset configures the Engine to use a Preset's Pipeline and
// Config atomically.
//
// # Behavior
//
// WithPreset is equivalent to calling both WithPipeline and WithConfig
// with the Preset's values, but is more discoverable and guarantees
// the Pipeline and Config are applied together.
//
// # Mutually Exclusive With
//
// WithLexicon, WithProfiles, WithProfileResolver, WithLexiconStore.
func WithPreset(p Preset) Option {
	return optionFunc(func(c *engineConfig) {
		c.pipe = p.Pipeline()
		c.config = p.Config()
		c.hasConfig = true
		c.preset = &p
		c.hasPreset = true
	})
}
