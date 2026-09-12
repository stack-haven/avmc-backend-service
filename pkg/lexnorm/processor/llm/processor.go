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

package llm

import (
	"context"
	"encoding/json"

	"github.com/stack-haven/lexnorm"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "llm"

	// Version is the semantic version of this Processor.
	Version = "v0"
)

// Processor is the Semantic Refinement placeholder.
//
// It establishes the integration boundary for model-driven refinement
// without coupling the core to any SDK: the actual implementation is
// expected to wrap an application-provided client (see the package
// documentation for the planned Client interface). As shipped, Process
// is a no-op so the Processor can safely sit in a Pipeline.
//
// # Semantics
//
// Semantic Refinement is characterized by lower certainty, higher
// mutation risk, and possible non-determinism; it belongs at the tail
// of a Pipeline (default order 8). Per D1 it is never part of the
// Standard Preset.
type Processor struct{}

// New returns the placeholder Semantic Processor.
func New() *Processor { return &Processor{} }

// Name implements lexnorm.Processor.
func (p *Processor) Name() string { return Name }

// Version implements lexnorm.Versioner.
func (p *Processor) Version() string { return Version }

// Certainty implements lexnorm.CertaintyReporter.
func (p *Processor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyLow }

// Process is a no-op in the placeholder implementation.
func (p *Processor) Process(_ context.Context, _ *lexnorm.State) error {
	return nil
}

// Descriptor is the Registry Descriptor for this Processor.
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyLow,
	New: func(_ json.RawMessage) (lexnorm.Processor, error) {
		return New(), nil
	},
	Default: func() any { return nil },

	Version:           Version,
	Category:          lexnorm.CategorySemantic,
	MutatesText:       false,
	SupportsSuggest:   false,
	SupportsProtected: false,
	Deterministic:     false,
	Determinism:       lexnorm.DeterministicGenerative,
	DefaultOrder:      8,
	Description:       "Semantic refinement via models; placeholder boundary, no SDK coupling.",
}

// Descriptor implements lexnorm.DescribedProcessor: it exposes the
// capability metadata of this Processor (category, mutation mode,
// determinism) for Registry queries, audit tooling, and docs.
func (p *Processor) Descriptor() lexnorm.Descriptor { return Descriptor }
