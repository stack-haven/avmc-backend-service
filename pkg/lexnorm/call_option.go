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

// CallOption customizes a single Normalize call.
//
// CallOptions follow the Functional Options pattern, parallel to Option.
// Each With* function returns a CallOption that mutates the call's
// internal config.
//
// # Mutability
//
// CallOption changes affect only the current call; the Engine's global
// state is not modified (architecture invariant: per-call isolation).
//
// # Resolution Priority
//
// Per-call overrides (via CallOption) take precedence over the Engine's
// default Runtime. The Runtime is captured at the start of Normalize and
// remains stable for the duration of the call.
type CallOption interface {
	apply(*callConfig)
}

// callOptionFunc adapts a plain function to the CallOption interface.
type callOptionFunc func(*callConfig)

func (f callOptionFunc) apply(c *callConfig) { f(c) }

// callConfig is the per-call configuration built up by CallOptions.
type callConfig struct {
	// profileID overrides the ProfileID for this call (multi-profile mode).
	profileID ProfileID

	// runtime overrides the entire Runtime for this call (advanced).
	runtime *Runtime

	// hasRuntime is true if runtime was set via WithRuntime.
	hasRuntime bool
}

// defaultCallConfig returns the zero-state callConfig.
func defaultCallConfig() callConfig {
	return callConfig{}
}

// ----------------------------------------------------------------------------
// Call Options
// ----------------------------------------------------------------------------

// WithProfileID selects which ProfileID to use for this call (multi-profile mode).
//
// The ID must be registered with the Engine (via WithProfiles or via
// the ProfileResolver). Returns ErrRuntime from Normalize if not found.
func WithProfileID(id ProfileID) CallOption {
	return callOptionFunc(func(c *callConfig) { c.profileID = id })
}

// WithRuntime overrides the entire Runtime for this call.
//
// This is the most powerful override: it bypasses ProfileResolver and
// the Engine's profile configuration. Useful for testing and for
// callers that want to inject a pre-built Runtime.
//
// When WithRuntime is used, the Runtime's ProfileID is recorded in the
// Result.RuntimeInfo.
func WithRuntime(rt *Runtime) CallOption {
	return callOptionFunc(func(c *callConfig) {
		c.runtime = rt
		c.hasRuntime = true
	})
}
