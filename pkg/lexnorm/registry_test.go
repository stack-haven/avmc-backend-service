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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/processor/normalize"
)

// ----------------------------------------------------------------------------
// Construction & basic
// ----------------------------------------------------------------------------

func TestNewRegistry_Empty(t *testing.T) {
	r := lexnorm.NewRegistry()
	if r.Len() != 0 {
		t.Errorf("new Registry must be empty, got Len=%d", r.Len())
	}
	if got := r.Names(); len(got) != 0 {
		t.Errorf("Names must be empty, got %v", got)
	}
}

func TestRegistry_NilSafe(t *testing.T) {
	var r *lexnorm.Registry
	if got := r.Len(); got != 0 {
		t.Errorf("nil Registry Len = %d, want 0", got)
	}
	if _, ok := r.Get("anything"); ok {
		t.Error("nil Registry Get must return false")
	}
	if got := r.Names(); got != nil {
		t.Errorf("nil Registry Names = %v, want nil", got)
	}
}

// ----------------------------------------------------------------------------
// Register / Get / Unregister
// ----------------------------------------------------------------------------

func TestRegistry_RegisterGet(t *testing.T) {
	r := lexnorm.NewRegistry()
	r.Register(normalize.Descriptor)

	d, ok := r.Get("normalize")
	if !ok {
		t.Fatal("Get(normalize) must return true")
	}
	if d.Name != "normalize" {
		t.Errorf("Name = %q, want normalize", d.Name)
	}
	if d.Certainty != lexnorm.CertaintyHigh {
		t.Errorf("Certainty = %v, want CertaintyHigh", d.Certainty)
	}
}

func TestRegistry_Get_Unknown(t *testing.T) {
	r := lexnorm.NewRegistry()
	if _, ok := r.Get("nonexistent"); ok {
		t.Error("Get(unknown) must return false")
	}
}

func TestRegistry_Overwrite(t *testing.T) {
	r := lexnorm.NewRegistry()
	r.Register(normalize.Descriptor)

	// Re-register with same name (no error).
	r.Register(lexnorm.Descriptor{
		Name:      "normalize",
		Certainty: lexnorm.CertaintyLow,
		New: func(_ json.RawMessage) (lexnorm.Processor, error) {
			return normalize.New(), nil
		},
	})
	if r.Len() != 1 {
		t.Errorf("Len = %d, want 1 (overwrite, not duplicate)", r.Len())
	}
	d, _ := r.Get("normalize")
	if d.Certainty != lexnorm.CertaintyLow {
		t.Error("overwrite must replace existing entry")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := lexnorm.NewRegistry()
	r.Register(normalize.Descriptor)
	r.Unregister("normalize")
	if r.Len() != 0 {
		t.Errorf("Len = %d after Unregister, want 0", r.Len())
	}
	if _, ok := r.Get("normalize"); ok {
		t.Error("Get after Unregister must return false")
	}
	// Unregister of unknown is no-op (no panic).
	r.Unregister("nonexistent")
}

func TestRegistry_RegisterEmptyName_Ignored(t *testing.T) {
	r := lexnorm.NewRegistry()
	r.Register(lexnorm.Descriptor{Name: ""})
	if r.Len() != 0 {
		t.Errorf("empty-name Descriptor must be ignored, got Len=%d", r.Len())
	}
}

func TestRegistry_Names_Sorted(t *testing.T) {
	r := lexnorm.NewRegistry()
	r.Register(lexnorm.Descriptor{Name: "zebra", New: func(_ json.RawMessage) (lexnorm.Processor, error) { return nil, nil }})
	r.Register(lexnorm.Descriptor{Name: "alpha", New: func(_ json.RawMessage) (lexnorm.Processor, error) { return nil, nil }})
	r.Register(lexnorm.Descriptor{Name: "mango", New: func(_ json.RawMessage) (lexnorm.Processor, error) { return nil, nil }})

	got := r.Names()
	want := []string{"alpha", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("len(Names) = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Names()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// ----------------------------------------------------------------------------
// Build
// ----------------------------------------------------------------------------

func TestRegistry_Build(t *testing.T) {
	r := lexnorm.NewRegistry()
	r.Register(normalize.Descriptor)

	p, err := r.Build("normalize", nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if p.Name() != "normalize" {
		t.Errorf("Name = %q, want normalize", p.Name())
	}
}

func TestRegistry_Build_UnknownName(t *testing.T) {
	r := lexnorm.NewRegistry()
	_, err := r.Build("nonexistent", nil)
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("Build(unknown) must return ErrInvalidConfig, got %v", err)
	}
}

func TestRegistry_Build_WithConfig(t *testing.T) {
	// Build with a non-nil config (used by descriptors that read it).
	r := lexnorm.NewRegistry()
	r.Register(lexnorm.Descriptor{
		Name: "test",
		New: func(cfg json.RawMessage) (lexnorm.Processor, error) {
			if len(cfg) == 0 {
				return nil, errors.New("expected config")
			}
			return &testProcessor{name: "test"}, nil
		},
		Default: func() any { return "default-cfg" },
	})

	p, err := r.Build("test", json.RawMessage(`{"key":"value"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "test" {
		t.Errorf("Name = %q, want test", p.Name())
	}
}

func TestRegistry_Build_UsesDefaultWhenCfgEmpty(t *testing.T) {
	r := lexnorm.NewRegistry()
	called := false
	r.Register(lexnorm.Descriptor{
		Name: "test",
		New: func(cfg json.RawMessage) (lexnorm.Processor, error) {
			if len(cfg) > 0 {
				called = true
			}
			return &testProcessor{name: "test"}, nil
		},
		Default: func() any { return "default" },
	})

	_, err := r.Build("test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("Build with empty cfg should invoke New with Default-encoded config")
	}
}

func TestRegistry_Build_NilRegistry(t *testing.T) {
	var r *lexnorm.Registry
	_, err := r.Build("anything", nil)
	if !errors.Is(err, lexnorm.ErrInvalidConfig) {
		t.Errorf("nil Registry.Build must return ErrInvalidConfig, got %v", err)
	}
}

func TestRegistry_Build_NewReturnsError(t *testing.T) {
	r := lexnorm.NewRegistry()
	r.Register(lexnorm.Descriptor{
		Name: "bad",
		New: func(_ json.RawMessage) (lexnorm.Processor, error) {
			return nil, errors.New("construction failed")
		},
	})
	_, err := r.Build("bad", nil)
	if err == nil {
		t.Error("Build must propagate New error")
	}
}

// ----------------------------------------------------------------------------
// Independence (Invariant I6)
// ----------------------------------------------------------------------------

func TestRegistry_IndependentOfEngine(t *testing.T) {
	// Registry must be usable without any Engine or Pipeline.
	r := lexnorm.NewRegistry()
	r.Register(normalize.Descriptor)

	// No Engine created; Registry operations work.
	p, err := r.Build("normalize", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Use the Processor directly (without Engine).
	s, _ := lexnorm.NewState(context.Background(), "  hello  ", nil, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if s.Text() != "hello" {
		t.Errorf("Text = %q, want %q", s.Text(), "hello")
	}
}

// ----------------------------------------------------------------------------
// Concurrency
// ----------------------------------------------------------------------------

func TestRegistry_ConcurrentRegisterGet(t *testing.T) {
	r := lexnorm.NewRegistry()
	r.Register(normalize.Descriptor)

	const N = 50
	done := make(chan struct{})
	for i := 0; i < N; i++ {
		go func() {
			_, _ = r.Get("normalize")
			done <- struct{}{}
		}()
	}
	for i := 0; i < N; i++ {
		<-done
	}
}

// ----------------------------------------------------------------------------
// Test helper
// ----------------------------------------------------------------------------

type testProcessor struct{ name string }

func (p *testProcessor) Name() string                                      { return p.name }
func (p *testProcessor) Process(_ context.Context, _ *lexnorm.State) error { return nil }
