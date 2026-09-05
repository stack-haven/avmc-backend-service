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

package lexnorm_test

import (
	"context"
	"sync/atomic"

	"github.com/stack-haven/lexnorm"
)

// ----------------------------------------------------------------------------
// Shared test helpers (used by pipeline_test.go and engine_test.go).
// ----------------------------------------------------------------------------

// trackingProcessor counts how many times Process is invoked.
type trackingProcessor struct {
	name   string
	err    error
	called atomic.Int32
}

func (p *trackingProcessor) Name() string { return p.name }
func (p *trackingProcessor) Process(_ context.Context, _ *lexnorm.State) error {
	p.called.Add(1)
	return p.err
}

// cancellingProcessor cancels its bound context when Process is invoked.
type cancellingProcessor struct {
	name   string
	cancel context.CancelFunc
}

func (c *cancellingProcessor) Name() string { return c.name }
func (c *cancellingProcessor) Process(_ context.Context, _ *lexnorm.State) error {
	c.cancel()
	return nil
}

// replaceProcessor applies a fixed State.Replace on each invocation.
type replaceProcessor struct {
	name string
	span lexnorm.Span
	to   string
}

func (r *replaceProcessor) Name() string { return r.name }
func (r *replaceProcessor) Process(_ context.Context, s *lexnorm.State) error {
	return s.Replace(r.span, r.to, lexnorm.ChangeMeta{
		Confidence: 1.0,
		Source:     r.name,
	})
}

// panickingProcessor panics in Process (for Recover middleware tests).
type panickingProcessor struct{}

func (p *panickingProcessor) Name() string { return "panic" }
func (p *panickingProcessor) Process(_ context.Context, _ *lexnorm.State) error {
	panic("intentional panic for test")
}

// recordingProcessor appends its Name to a shared slice on each invocation.
type recordingProcessor struct {
	name  string
	calls *[]string
}

func (r *recordingProcessor) Name() string { return r.name }
func (r *recordingProcessor) Process(_ context.Context, _ *lexnorm.State) error {
	*r.calls = append(*r.calls, r.name)
	return nil
}

// orderRecordingPipeline records each processor's name when run, in order.
//
// It exposes a single internal recorder via Processors() so that the
// Engine's iteration logic actually runs.
type orderRecordingPipeline struct {
	name  string
	order *[]string
}

func (p *orderRecordingPipeline) Name() string { return p.name }
func (p *orderRecordingPipeline) Processors() []lexnorm.Processor {
	return []lexnorm.Processor{&orderRecorder{order: p.order}}
}
func (p *orderRecordingPipeline) Process(_ context.Context, _ *lexnorm.State) error {
	// Unused (Engine iterates Processors()).
	return nil
}

// orderRecorder is the internal Processor used by orderRecordingPipeline.
type orderRecorder struct {
	order *[]string
}

func (r *orderRecorder) Name() string { return "pipe" }
func (r *orderRecorder) Process(_ context.Context, _ *lexnorm.State) error {
	*r.order = append(*r.order, "pipe")
	return nil
}

// sliceEq compares two string slices for equality.
func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stringsContains is a tiny helper that avoids the strings import.
func stringsContains(s, substr string) bool {
	return len(substr) <= len(s) && (len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
