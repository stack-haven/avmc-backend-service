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

// Package ctxproc implements the Context Processor.
//
// # M9 Status: Skeleton (No-Op)
//
// M9 provides the Context Processor interface and identity, but not
// the actual disambiguation logic. The reason: real context-aware
// correction requires application-specific resources that the core
// package cannot bundle:
//
//   - **LLM-based disambiguation** (D1: LLM is an optional extension).
//   - **Domain-specific rules** (medical, legal, financial, ...).
//   - **Statistical / ML models** (BERT-style classifiers, ...).
//   - **User-feedback loops** (active learning from corrections).
//
// The core package does not bundle any of these. Application code
// should provide a custom Context Processor that wraps the desired
// resource.
//
// # Package Name
//
// The package is named `ctxproc` (not `context`) to avoid collision
// with the Go standard library `context` package. The directory is
// `processor/ctxproc/`.
//
// # Interface
//
// The default Context Processor is a no-op: it preserves the State
// unchanged. Application code can either:
//
//  1. Wrap this default and add custom logic in a Middleware or Hook
//     to inspect / modify the State.
//  2. Provide a fully custom Processor (via the Processor interface)
//     that uses the desired LLM / ML / rule-based system.
//
// # Order in Standard Pipeline
//
// Context runs LAST in the Standard Pipeline, after Pinyin and Fuzzy.
// Its purpose is to disambiguate among multiple Suggestions emitted
// by earlier Processors (e.g., choosing between 同音 candidates based
// on surrounding context).
//
// # Future Enhancements
//
// When the Lexicon provides a context-aware scoring function
// (e.g., per-domain weights), M12 may upgrade this Processor to use
// that scoring. For now, it remains a placeholder.
package ctxproc

import (
	"context"
	"encoding/json"

	"github.com/stack-haven/lexnorm"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "context"

	// Version is the semantic version of this Processor.
	Version = "v1"
)

// Processor is the default (no-op) Context Processor.
//
// The default implementation performs no text modifications. Replace
// this Processor with a custom one (or wrap it via Middleware / Hook)
// to add real context-aware logic.
type Processor struct{}

// New returns a new Context Processor.
func New() *Processor {
	return &Processor{}
}

// Name implements lexnorm.Processor.
func (p *Processor) Name() string { return Name }

// Version implements lexnorm.Versioner.
func (p *Processor) Version() string { return Version }

// Certainty implements lexnorm.CertaintyReporter.
//
// Context is low-certainty by design: it operates on Suggestions
// from upstream Processors and may further reduce confidence.
func (p *Processor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyLow }

// Process is a no-op in the default implementation.
//
// It does not modify the State; the State passes through unchanged.
// Application code should provide a custom implementation for real
// context-aware disambiguation.
func (p *Processor) Process(_ context.Context, _ *lexnorm.State) error {
	return nil
}

// Descriptor is the Registry Descriptor for the default no-op Context
// Processor. Application code should provide a custom Descriptor
// pointing to a real LLM / ML / rule-based Processor.
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyLow,
	New: func(_ json.RawMessage) (lexnorm.Processor, error) {
		return New(), nil
	},
	Default: func() any { return nil },
}
