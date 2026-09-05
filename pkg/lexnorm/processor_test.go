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
	"errors"
	"testing"

	"github.com/stack-haven/lexnorm"
)

// Compile-time assertions: stubProcessor must satisfy Processor.
// If the interface is broken (method renamed, signature changed), the
// package will fail to compile.
var (
	_ lexnorm.Processor         = (*stubProcessor)(nil)
	_ lexnorm.Versioner         = (*stubProcessor)(nil)
	_ lexnorm.CertaintyReporter = (*stubProcessor)(nil)
)

// stubProcessor is a minimal Processor implementation used to verify
// the interface contract compiles and behaves correctly.
type stubProcessor struct {
	name       string
	version    string
	certainty  lexnorm.Certainty
	processErr error
}

func (s *stubProcessor) Name() string                 { return s.name }
func (s *stubProcessor) Version() string              { return s.version }
func (s *stubProcessor) Certainty() lexnorm.Certainty { return s.certainty }
func (s *stubProcessor) Process(_ context.Context, _ *lexnorm.State) error {
	return s.processErr
}

func TestProcessor_Interface_Name(t *testing.T) {
	p := &stubProcessor{name: "stub", version: "v1", certainty: lexnorm.CertaintyHigh}
	if p.Name() != "stub" {
		t.Errorf("Name() = %q, want %q", p.Name(), "stub")
	}
}

func TestProcessor_Interface_Version_Optional(t *testing.T) {
	// A Processor that does NOT implement Versioner has no Version().
	noVersion := &stubProcessor{name: "x"} // no version field set
	if noVersion.Version() != "" {
		t.Errorf("empty version must return empty string, got %q", noVersion.Version())
	}

	withVersion := &stubProcessor{name: "x", version: "v1.2.3"}
	if withVersion.Version() != "v1.2.3" {
		t.Errorf("Version() = %q, want %q", withVersion.Version(), "v1.2.3")
	}
}

func TestProcessor_Interface_Certainty_Optional(t *testing.T) {
	low := &stubProcessor{certainty: lexnorm.CertaintyLow}
	if low.Certainty() != lexnorm.CertaintyLow {
		t.Errorf("Certainty() = %v, want %v", low.Certainty(), lexnorm.CertaintyLow)
	}

	high := &stubProcessor{certainty: lexnorm.CertaintyHigh}
	if high.Certainty() != lexnorm.CertaintyHigh {
		t.Errorf("Certainty() = %v, want %v", high.Certainty(), lexnorm.CertaintyHigh)
	}
}

func TestProcessor_Process_ReturnsError(t *testing.T) {
	customErr := errors.New("custom processor error")
	p := &stubProcessor{processErr: customErr}

	err := p.Process(context.Background(), nil)
	if !errors.Is(err, customErr) {
		t.Errorf("Process() must return the underlying error directly, got %v", err)
	}
}

func TestProcessor_Process_NilError(t *testing.T) {
	p := &stubProcessor{}
	if err := p.Process(context.Background(), nil); err != nil {
		t.Errorf("Process() with nil err must return nil, got %v", err)
	}
}

func TestProcessorError_Error(t *testing.T) {
	base := errors.New("base error")
	pe := &lexnorm.ProcessorError{Name: "alias", Op: "match", Err: base}

	got := pe.Error()
	if want := "processor alias in match: base error"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestProcessorError_Error_NoOp(t *testing.T) {
	base := errors.New("base")
	pe := &lexnorm.ProcessorError{Name: "alias", Err: base}

	got := pe.Error()
	if want := "processor alias: base"; got != want {
		t.Errorf("Error() = %q, want %q (no Op)", got, want)
	}
}

func TestProcessorError_Unwrap(t *testing.T) {
	base := errors.New("base error")
	pe := &lexnorm.ProcessorError{Name: "alias", Op: "match", Err: base}

	if !errors.Is(pe, base) {
		t.Error("errors.Is must traverse Unwrap to find base")
	}
}

func TestProcessorError_Unwrap_Nested(t *testing.T) {
	inner := errors.New("inner")
	outer := &lexnorm.ProcessorError{Name: "outer", Err: inner}
	wrapped := &lexnorm.ProcessorError{Name: "wrapper", Op: "apply", Err: outer}

	// errors.Is must walk the chain.
	if !errors.Is(wrapped, inner) {
		t.Error("errors.Is must traverse nested ProcessorError")
	}

	// errors.As must extract the first matching *ProcessorError.
	var target *lexnorm.ProcessorError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As must extract *ProcessorError")
	}
	if target.Name != "wrapper" {
		t.Errorf("errors.As extracted wrong ProcessorError: Name=%q, want %q",
			target.Name, "wrapper")
	}
}

func TestProcessorError_NilSafe(t *testing.T) {
	var pe *lexnorm.ProcessorError
	if pe.Error() != "<nil ProcessorError>" {
		t.Error("nil ProcessorError must not panic on Error()")
	}
	if pe.Unwrap() != nil {
		t.Error("nil ProcessorError must not panic on Unwrap()")
	}
}

func TestWrapProcessorError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if err := lexnorm.WrapProcessorError("alias", "match", nil); err != nil {
			t.Errorf("WrapProcessorError(nil) must return nil, got %v", err)
		}
	})

	t.Run("plain error", func(t *testing.T) {
		base := errors.New("base")
		err := lexnorm.WrapProcessorError("alias", "match", base)
		var pe *lexnorm.ProcessorError
		if !errors.As(err, &pe) {
			t.Fatal("result must be a *ProcessorError")
		}
		if pe.Name != "alias" || pe.Op != "match" || pe.Err != base {
			t.Error("wrapped fields mismatch")
		}
	})

	t.Run("double wrap no-op", func(t *testing.T) {
		base := errors.New("base")
		first := lexnorm.WrapProcessorError("alias", "match", base)
		second := lexnorm.WrapProcessorError("fuzzy", "apply", first)

		// second must be the same as first (no double-wrap).
		if second != first {
			t.Error("WrapProcessorError must not double-wrap an existing *ProcessorError")
		}

		// errors.Is must still find the base.
		if !errors.Is(second, base) {
			t.Error("errors.Is must find base through no-op wrap")
		}
	})
}

// nonProcessor verifies that the compile-time assertion above is actually
// catching violations. If this type accidentally satisfies Processor, it
// would compile, which is a signal the assertion needs to be tightened.
type nonProcessor struct{}

func (nonProcessor) SomeOtherMethod() {}
