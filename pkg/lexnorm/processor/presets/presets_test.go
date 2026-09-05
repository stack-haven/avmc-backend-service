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

package presets_test

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/internal/lexutil"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/presets"
)

func sampleLex() lexicon.Lexicon {
	return lexutil.NewMemLexicon([]lexicon.Entry{
		{ID: "e1", Text: "周丽群", Variants: []lexicon.Variant{
			{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
		}},
	}, "v1")
}

// mapConverter is a trivial PinyinConverter that returns text unchanged.
type mapConverter struct{}

func (mapConverter) ToPinyin(text string) []string {
	return []string{text}
}

func TestStandard(t *testing.T) {
	p := presets.Standard(sampleLex(), mapConverter{})
	if p.Name() != "standard" {
		t.Errorf("Name = %q, want standard", p.Name())
	}
	// 7 Processors: Normalize + Disfluency + Alias + Deterministic + Pinyin + Fuzzy + Context
	if got := len(p.Pipeline().Processors()); got != 7 {
		t.Errorf("len(Processors) = %d, want 7", got)
	}
	// Engine integration smoke test.
	e, err := lexnorm.New(lexnorm.WithPreset(*p))
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Normalize(context.Background(), "周莉群")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "周丽群" {
		t.Errorf("Text = %q, want %q", res.Text, "周丽群")
	}
}

func TestHighAccuracy(t *testing.T) {
	p := presets.HighAccuracy(sampleLex(), mapConverter{})
	if p.Name() != "high-accuracy" {
		t.Errorf("Name = %q, want high-accuracy", p.Name())
	}
	cfg := p.Config()
	if cfg.AutoApplyThreshold >= 0.95 {
		t.Errorf("HighAccuracy should have lower AutoApplyThreshold, got %v", cfg.AutoApplyThreshold)
	}
}

func TestFast(t *testing.T) {
	p := presets.Fast(sampleLex())
	if p.Name() != "fast" {
		t.Errorf("Name = %q, want fast", p.Name())
	}
	// 2 Processors: Normalize + Alias
	if got := len(p.Pipeline().Processors()); got != 2 {
		t.Errorf("len(Processors) = %d, want 2", got)
	}
}

func TestASR(t *testing.T) {
	p := presets.ASR(sampleLex(), mapConverter{})
	if p.Name() != "asr" {
		t.Errorf("Name = %q, want asr", p.Name())
	}
	// 4 Processors: Normalize + Disfluency + Alias + Pinyin
	if got := len(p.Pipeline().Processors()); got != 4 {
		t.Errorf("len(Processors) = %d, want 4", got)
	}
}

func TestOCR(t *testing.T) {
	p := presets.OCR(sampleLex())
	if p.Name() != "ocr" {
		t.Errorf("Name = %q, want ocr", p.Name())
	}
	// 3 Processors: Normalize + Alias + Deterministic
	if got := len(p.Pipeline().Processors()); got != 3 {
		t.Errorf("len(Processors) = %d, want 3", got)
	}
}

func TestPresets_ConfigCustomizable(t *testing.T) {
	p := presets.Standard(sampleLex(), mapConverter{})

	// Caller mutates Config before passing to Engine.
	cfg := p.Config()
	cfg.AutoApplyThreshold = 0.5
	cfg.SuggestThreshold = 0.3
	custom := lexnorm.NewPreset(p.Name(), p.Description(), p.Pipeline(), cfg)

	e, _ := lexnorm.New(lexnorm.WithPreset(*custom))
	res, _ := e.Normalize(context.Background(), "周莉群")
	if res.Text != "周丽群" {
		t.Errorf("Text = %q, want %q", res.Text, "周丽群")
	}
}
