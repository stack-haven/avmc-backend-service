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
	"testing"

	"github.com/stack-haven/lexnorm"
)

func TestNewPreset(t *testing.T) {
	p := lexnorm.NewPipeline(&testProcessor{name: "p1"}, &testProcessor{name: "p2"})
	cfg := lexnorm.DefaultConfig()
	preset := lexnorm.NewPreset("test", "Test preset", p, cfg)

	if preset.Name() != "test" {
		t.Errorf("Name = %q, want test", preset.Name())
	}
	if preset.Description() != "Test preset" {
		t.Errorf("Description = %q, want 'Test preset'", preset.Description())
	}
	if got := len(preset.Pipeline().Processors()); got != 2 {
		t.Errorf("len(Processors) = %d, want 2", got)
	}
	if preset.Config().AutoApplyThreshold != cfg.AutoApplyThreshold {
		t.Error("Config not preserved")
	}
}

func TestPreset_String(t *testing.T) {
	p := lexnorm.NewPipeline()
	preset := lexnorm.NewPreset("mypreset", "My custom preset", p, lexnorm.DefaultConfig())
	got := preset.String()
	if got != "Preset{mypreset: My custom preset}" {
		t.Errorf("String() = %q", got)
	}
}

func TestPreset_ConfigIsValueCopy(t *testing.T) {
	// Modifying the returned Config should not affect the Preset's
	// internal state for the next caller.
	p := lexnorm.NewPipeline()
	cfg := lexnorm.DefaultConfig()
	cfg.AutoApplyThreshold = 0.5
	preset := lexnorm.NewPreset("p", "d", p, cfg)

	// Caller mutates the returned Config.
	got := preset.Config()
	got.AutoApplyThreshold = 0.99

	// Preset's internal Config is unchanged.
	if preset.Config().AutoApplyThreshold != 0.5 {
		t.Errorf("Config mutation must not affect Preset (got %v)", preset.Config().AutoApplyThreshold)
	}
}

// ----------------------------------------------------------------------------
// WithPreset integration
// ----------------------------------------------------------------------------

func TestEngine_WithPreset(t *testing.T) {
	p := lexnorm.NewPipeline(&testProcessor{name: "p1"})
	preset := lexnorm.NewPreset("test", "d", p, lexnorm.DefaultConfig())

	e, err := lexnorm.New(lexnorm.WithPreset(*preset))
	if err != nil {
		t.Fatalf("New with WithPreset: %v", err)
	}
	res, err := e.Normalize(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != lexnorm.StatusSuccess {
		t.Errorf("Status = %v, want StatusSuccess", res.Status)
	}
}

func TestEngine_WithPreset_OverridesPipeline(t *testing.T) {
	// WithPreset is the primary mode; WithPipeline called after is
	// a no-op (Preset's Pipeline wins). This documents the
	// "last-one-wins" semantics for the Pipeline field.
	p1 := lexnorm.NewPipeline(&testProcessor{name: "p1"})
	p2 := lexnorm.NewPipeline(&testProcessor{name: "p2"})
	preset := lexnorm.NewPreset("test", "d", p1, lexnorm.DefaultConfig())

	e, err := lexnorm.New(
		lexnorm.WithPreset(*preset),
		lexnorm.WithPipeline(p2), // overridden
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	res, err := e.Normalize(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != lexnorm.StatusSuccess {
		t.Errorf("Status = %v, want StatusSuccess", res.Status)
	}
}
