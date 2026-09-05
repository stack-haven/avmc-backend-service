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

import "fmt"

// Preset bundles a Pipeline + Config + human-readable metadata.
//
// # Purpose
//
// Preset is a "named recipe" for a common normalization scenario.
// Application code can use Preset as a starting point and customize
// further (e.g., add more Middleware or adjust thresholds).
//
// # Built-in Presets
//
// The companion package `processor/presets` provides built-in
// factory functions for the standard Presets:
//
//   - Standard: full pipeline (Normalize + Disfluency + Alias +
//     Deterministic + Pinyin + Fuzzy + Context)
//   - HighAccuracy: Standard with tighter thresholds
//   - Fast: Normalize + Alias only
//   - ASR: Normalize + Disfluency + Alias + Pinyin
//   - OCR: Normalize + Alias + Deterministic
//
// # Engine Integration
//
// Use WithPreset to apply a Preset to an Engine:
//
//	preset, _ := presets.Standard(myLex, myConverter)
//	engine, _ := lexnorm.New(lexnorm.WithPreset(*preset))
//
// WithPreset sets the Pipeline, Config, and (in the future)
// default Middleware / Hooks. Individual Options like
// WithMiddleware / WithHooks can still be added on top.
type Preset struct {
	name        string
	description string
	pipeline    Pipeline
	config      Config
}

// NewPreset creates a Preset with the given fields.
//
// Application code typically uses the factory functions in the
// `processor/presets` package rather than calling NewPreset directly.
func NewPreset(name, description string, pipeline Pipeline, config Config) *Preset {
	return &Preset{
		name:        name,
		description: description,
		pipeline:    pipeline,
		config:      config,
	}
}

// Name returns the Preset's identifier (e.g., "standard", "asr").
func (p *Preset) Name() string { return p.name }

// Description returns a human-readable summary of the Preset.
func (p *Preset) Description() string { return p.description }

// Pipeline returns the Processor chain for this Preset.
func (p *Preset) Pipeline() Pipeline { return p.pipeline }

// Config returns the default Config for this Preset.
//
// Application code may modify a returned Config before passing to the
// Engine (e.g., adjust AutoApplyThreshold).
func (p *Preset) Config() Config { return p.config }

// String returns a one-line description of the Preset.
func (p *Preset) String() string {
	return fmt.Sprintf("Preset{%s: %s}", p.name, p.description)
}
