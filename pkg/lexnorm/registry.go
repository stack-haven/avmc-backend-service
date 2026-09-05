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
	"encoding/json"
	"fmt"
	"sync"
)

// Registry holds a collection of Processor Descriptors for dynamic
// construction.
//
// # Architecture Invariant (I6)
//
// Registry is NOT required to use lexnorm. The Engine works directly
// with concrete Processors and Pipelines. Registry is provided for
// applications that need dynamic / configuration-driven Processor
// construction (e.g., YAML-defined Pipelines).
//
// # Independence
//
// Registry does not depend on Engine. It can be used in any context
// (e.g., configuration loaders, plugin systems, tests).
//
// # Concurrency
//
// Registry is safe for concurrent Register / Get / Build calls. The
// underlying New function provided in each Descriptor must also be
// safe for concurrent invocation (typically true for stateless
// Processors).
type Registry struct {
	mu          sync.RWMutex
	descriptors map[string]Descriptor
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{descriptors: make(map[string]Descriptor)}
}

// Register adds a Descriptor to the Registry.
//
// If a Descriptor with the same Name is already registered, it is
// overwritten. Register is safe for concurrent use.
func (r *Registry) Register(d Descriptor) {
	if r == nil || d.Name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.descriptors[d.Name] = d
}

// Unregister removes a Descriptor by name. No-op if not found.
func (r *Registry) Unregister(name string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.descriptors, name)
}

// Get returns the Descriptor for the given name.
//
// The second return is false if no Descriptor is registered.
func (r *Registry) Get(name string) (Descriptor, bool) {
	if r == nil {
		return Descriptor{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.descriptors[name]
	return d, ok
}

// Names returns a sorted list of all registered Descriptor names.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.descriptors))
	for name := range r.descriptors {
		names = append(names, name)
	}
	// Sort for deterministic output.
	sortStrings(names)
	return names
}

// Len returns the number of registered Descriptors.
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.descriptors)
}

// Build constructs a Processor by name, using the provided JSON config.
//
// If cfg is nil or empty and the Descriptor provides a Default function,
// the default configuration is used (encoded as JSON).
//
// Returns ErrInvalidConfig (wrapped) if the name is not registered
// or the New function fails.
func (r *Registry) Build(name string, cfg json.RawMessage) (Processor, error) {
	if r == nil {
		return nil, fmt.Errorf("nil Registry: %w", ErrInvalidConfig)
	}
	d, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("processor %q not registered: %w", name, ErrInvalidConfig)
	}
	if d.New == nil {
		return nil, fmt.Errorf("processor %q has no New function: %w", name, ErrInvalidConfig)
	}

	// Use Default if cfg is empty and Default is available.
	if len(cfg) == 0 && d.Default != nil {
		defaultVal, err := json.Marshal(d.Default())
		if err != nil {
			return nil, fmt.Errorf("processor %q: marshal default: %w", name, err)
		}
		cfg = defaultVal
	}

	return d.New(cfg)
}

// sortStrings sorts a string slice in place. Kept local to avoid
// importing "sort" (used only for Names()).
func sortStrings(s []string) {
	// Insertion sort: N is small (number of Descriptors).
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
