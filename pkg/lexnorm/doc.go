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

// Package lexnorm provides a general-purpose, composable, deterministic
// text normalization engine.
//
// # Overview
//
// ark-lexnorm processes raw text through a composable chain of small,
// well-defined processing units (Processor). It is designed for scenarios
// such as ASR/OCR transcript correction, meeting/customer service
// transcription, search query normalization, NLP/Agent input preprocessing,
// and document cleaning.
//
// The engine is intentionally domain-neutral: it does not assume ASR, HR,
// CRM, or any specific business context. Domain-specific knowledge is
// plugged in via LexiconSource, and the normalization flow is fully
// customizable via Pipeline.
//
// # Quick Start
//
//	engine, err := lexnorm.New(
//	    lexnorm.WithLexicon(lex),
//	    lexnorm.WithPipeline(pipeline),
//	)
//	if err != nil {
//	    return err
//	}
//	result, err := engine.Normalize(ctx, "raw text")
//
// # Architecture
//
// The engine is built around 9 core abstractions:
//
//   - Profile:    normalization context identifier
//   - Lexicon:    knowledge container (immutable, atomic swap)
//   - LexiconSource: knowledge source abstraction (composable)
//   - Processor:  minimal normalization capability unit
//   - Pipeline:   processor composition mechanism (interface)
//   - State:      per-request working area (single-goroutine exclusive)
//   - Runtime:    immutable per-request snapshot
//   - Result:     normalization outcome with full RuntimeInfo
//   - Engine:     facade for the runtime environment
//
// # Design Principles
//
//   - Determinism:    same input + same snapshot = same output
//   - Controllability: every Processor is opt-in/opt-out/reorderable
//   - Explainability:  every change carries full audit trail
//   - Composability:   every Processor is independently usable
//   - Degradability:   failure does not lose original text
//   - Zero business coupling: no tenant/ASR/HR/Employee/Document in core
//
// # Constraints
//
//   - Zero third-party dependencies (only Go standard library)
//   - 15 architectural invariants (see .agents/REVIEW.md)
//   - 7 locked decisions D1-D7 (see .agents/RULES.md)
//
// # Versioning
//
// Current target: v1.0.0 (M12 milestone).
// See CHANGELOG.md for version history.
package lexnorm
