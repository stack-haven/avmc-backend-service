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

// Package llm is a PLACEHOLDER for the optional LLM-based Processor
// extension (M11, D1 decision: LLM is optional, NOT in Standard Preset).
//
// # M11 Status: SKELETON ONLY
//
// This file provides the interface skeleton only. Full implementation
// will be added in a later iteration. The skeleton exists so that:
//
//  1. The interface boundary is established (no SDK coupling in core).
//  2. Application code can begin prototyping LLM-based Processors.
//  3. The pipeline topology (where LLM would sit, if at all) is
//     documented.
//
// # Why a Placeholder
//
// The core package does not bundle any LLM SDK (OpenAI, Anthropic,
// etc.) to keep the dependency surface minimal. The actual
// implementation requires:
//
//   - A LLM SDK (application-provided, behind the Client interface)
//   - Prompt templates (application-specific)
//   - Token / cost management
//   - Provider selection / failover
//   - Caching / batching
//
// These concerns are out of scope for the core package.
//
// # Planned Interface (for documentation only)
//
// The intended interface is:
//
//	type Client interface {
//	    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
//	}
//
//	type CompletionRequest struct {
//	    Prompt      string
//	    MaxTokens   int
//	    Temperature float64
//	}
//
//	type CompletionResponse struct {
//	    Text   string
//	    Usage  TokenUsage
//	}
//
//	type Processor struct {
//	    client Client
//	    // ...
//	}
//
//	func New(client Client) *Processor { ... }
//
// # Usage (when implemented)
//
//	client := myOpenAIClient   // application implementation
//	p := llm.New(client)
//	pipeline := lexnorm.NewPipeline(/* ..., */ p, /* ..., */)
//
// # D1 Reminder
//
// Per D1: LLM is NOT part of the Standard Preset. Application code
// that wants LLM Refine must construct a custom Pipeline including
// the LLM Processor explicitly.
package llm

// (No implementation yet. See package documentation for the planned
// interface and usage.)
