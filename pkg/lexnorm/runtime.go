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

// Runtime is the full, immutable snapshot of the runtime context for one
// Normalize call (1.1 §4.4).
//
// # Snapshot Consistency (Architecture Invariant I8)
//
// A Normalize call always operates on exactly one Runtime. Even when
// the Lexicon Store performs an atomic swap during the call, the
// in-flight call continues with the snapshot it captured at start. The
// next call sees the new snapshot.
//
// # Immutability
//
// Runtime is treated as immutable after ProfileResolver returns it.
// Components (Lexicon, Pipeline, Config) must also be safe for shared
// concurrent read access.
//
// # Concurrency
//
// A single Runtime may be shared across goroutines for READ purposes
// (e.g., Pipeline.Process on multiple States). For WRITE purposes (e.g.,
// Lexicon atomic swap), see Store (M7).
type Runtime struct {
	// Profile identifies the resolved Profile (ID + Version).
	Profile Profile

	// Lexicon is the knowledge container for this call.
	Lexicon lexicon.Lexicon

	// Pipeline is the composition of Processors for this call.
	Pipeline Pipeline

	// Config is the per-Profile configuration.
	Config Config

	// ProfileVersion is the version string of the resolved Profile.
	ProfileVersion string

	// LexiconVersion is the version string of the active Lexicon.
	LexiconVersion string

	// PipelineVersion is the version string of the active Pipeline.
	PipelineVersion string

	// ProcessorVersions maps Processor.Name() to Version() for all
	// Processors that implement the optional Versioner interface.
	ProcessorVersions map[string]string
}

// NewRuntime constructs a Runtime from a ProfileID and ProfileBundle.
//
// This is the canonical way to construct a Runtime in production code
// (e.g., from a custom ProfileResolver implementation). It validates
// the bundle first and returns ErrInvalidConfig on failure.
//
// # Version Capture
//
// NewRuntime captures LexiconVersion from bundle.Lexicon.Version() and
// PipelineVersion from bundle.Pipeline.(Versioner).Version() (empty if
// the Pipeline does not implement Versioner). ProcessorVersions are
// captured by iterating bundle.Pipeline.Processors().
func NewRuntime(id ProfileID, b ProfileBundle) (*Runtime, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}
	return newRuntimeFromBundle(id, b), nil
}

// newRuntimeFromBundle builds a Runtime from a ProfileBundle, capturing
// versions from the Lexicon, Pipeline, and Processors.
//
// Panics if the bundle is invalid (caller should validate first).
//
// Lexicon may be nil (e.g., when a Preset's Pipeline has its own
// Lexicon references but the Runtime doesn't need a separate one).
// In that case, the LexiconVersion / ProfileVersion fields are empty.
func newRuntimeFromBundle(id ProfileID, b ProfileBundle) *Runtime {
	cfg := b.resolvedConfig()
	lexVersion := ""
	if b.Lexicon != nil {
		lexVersion = b.Lexicon.Version()
	}
	rt := &Runtime{
		Profile:        Profile{ID: id, Version: lexVersion},
		Lexicon:        b.Lexicon,
		Pipeline:       b.Pipeline,
		Config:         cfg,
		ProfileVersion: lexVersion,
		LexiconVersion: lexVersion,
	}
	if v, ok := b.Pipeline.(Versioner); ok {
		rt.PipelineVersion = v.Version()
	}
	processors := b.Pipeline.Processors()
	if len(processors) > 0 {
		rt.ProcessorVersions = make(map[string]string, len(processors))
		for _, p := range processors {
			if v, ok := p.(Versioner); ok {
				rt.ProcessorVersions[p.Name()] = v.Version()
			}
		}
	}
	return rt
}

// info returns a RuntimeInfo (metadata-only view) for use in Result.
func (r *Runtime) info() RuntimeInfo {
	if r == nil {
		return RuntimeInfo{}
	}
	return RuntimeInfo{
		ProfileID:         r.Profile.ID,
		ProfileVersion:    r.ProfileVersion,
		LexiconVersion:    r.LexiconVersion,
		PipelineVersion:   r.PipelineVersion,
		ProcessorVersions: copyProcessorVersions(r.ProcessorVersions),
	}
}

func copyProcessorVersions(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// RuntimeInfo is the immutable snapshot of the runtime context used for
// one Normalize call.
//
// RuntimeInfo is part of Result. It is captured by the Engine at the start
// of Normalize, after the ProfileResolver has returned a Runtime, and is
// never modified afterward. Callers use RuntimeInfo for audit, replay,
// and observability.
//
// # Snapshot Consistency (Architecture Invariant I8)
//
// A Normalize call always operates on exactly one RuntimeInfo. Even when
// the Lexicon Store performs an atomic swap during the call, the
// in-flight call continues with the snapshot it captured at start. The
// next call sees the new snapshot.
//
// # Fields
//
//   - ProfileID + ProfileVersion: which Profile was active
//   - LexiconVersion: which Lexicon Snapshot was active
//   - PipelineVersion: which Pipeline was active
//   - ProcessorVersions: per-Processor version (only for Processors that
//     implement the optional Versioner interface)
type RuntimeInfo struct {
	// ProfileID is the identifier of the resolved Profile.
	ProfileID ProfileID

	// ProfileVersion is the version string of the resolved Profile.
	ProfileVersion string

	// LexiconVersion is the version string of the active Lexicon.
	LexiconVersion string

	// PipelineVersion is the version string of the active Pipeline.
	PipelineVersion string

	// ProcessorVersions maps Processor.Name() to Version() for all
	// Processors that implement the optional Versioner interface.
	// Processors without Versioner are omitted.
	ProcessorVersions map[string]string
}

// IsZero reports whether ri is the zero value (no runtime info).
func (ri RuntimeInfo) IsZero() bool {
	return ri.ProfileID.IsZero() &&
		ri.ProfileVersion == "" &&
		ri.LexiconVersion == "" &&
		ri.PipelineVersion == "" &&
		len(ri.ProcessorVersions) == 0
}

// ProcessorVersion returns the version of the named Processor, or "" if
// the Processor did not implement Versioner or is not in this snapshot.
func (ri RuntimeInfo) ProcessorVersion(name string) string {
	if ri.ProcessorVersions == nil {
		return ""
	}
	return ri.ProcessorVersions[name]
}

// String returns a compact human-readable summary suitable for logs.
//
// Format: "Profile{id@vN} Lexicon@vN Pipeline@vN [proc1@v1 proc2@v2 ...]"
//
// The processor list is sorted alphabetically for deterministic output.
func (ri RuntimeInfo) String() string {
	if ri.IsZero() {
		return "RuntimeInfo{}"
	}

	// Sort processor versions for determinism.
	processors := make([]string, 0, len(ri.ProcessorVersions))
	for name, ver := range ri.ProcessorVersions {
		if ver == "" {
			processors = append(processors, name)
		} else {
			processors = append(processors, fmt.Sprintf("%s@%s", name, ver))
		}
	}
	// Insertion sort: small N, allocation-free.
	for i := 1; i < len(processors); i++ {
		for j := i; j > 0 && processors[j-1] > processors[j]; j-- {
			processors[j-1], processors[j] = processors[j], processors[j-1]
		}
	}

	procStr := ""
	if len(processors) > 0 {
		procStr = fmt.Sprintf(" [%s]", joinComma(processors))
	}

	return fmt.Sprintf("Profile{%s@%s} Lexicon@%s Pipeline@%s%s",
		ri.ProfileID, ri.ProfileVersion,
		ri.LexiconVersion, ri.PipelineVersion,
		procStr,
	)
}

// joinComma joins strings with ", " (avoiding import of strings package
// in the hot path is not a goal; this just keeps the dependency surface
// minimal).
func joinComma(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	return out
}
