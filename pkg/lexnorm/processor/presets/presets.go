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

// Package presets provides factory functions for the standard
// ark-lexnorm Presets (Standard, HighAccuracy, Fast, ASR, OCR).
//
// # Why a Separate Package
//
// The Preset struct lives in the root lexnorm package. However, the
// built-in Preset factory functions (e.g., Standard) need to import
// the individual Processor sub-packages (normalize, disfluency,
// alias, etc.). If they were in lexnorm itself, that would create
// a circular import.
//
// This sub-package breaks the cycle by acting as the assembly point
// that depends on both lexnorm (for the Preset type) and the
// Processor sub-packages (for the concrete constructors).
//
// # Usage
//
//	import "github.com/stack-haven/lexnorm/processor/presets"
//
//	preset := presets.Standard(myLex, myConverter)
//	engine, _ := lexnorm.New(lexnorm.WithPreset(*preset))
//
// # Customization
//
// Preset values returned here are mutable. Application code can
// adjust Config thresholds before passing to the Engine:
//
//	p := presets.Standard(myLex, myConverter)
//	cfg := p.Config()
//	cfg.AutoApplyThreshold = 0.85
//	preset := lexnorm.NewPreset(p.Name(), p.Description(), p.Pipeline(), cfg)
package presets

import (
	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/ctxproc"
	"github.com/stack-haven/lexnorm/processor/deterministic"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/fuzzy"
	"github.com/stack-haven/lexnorm/processor/normalize"
	"github.com/stack-haven/lexnorm/processor/pinyin"
)

// Standard returns the full Standard Pipeline Preset.
//
// Pipeline:
//
//	Normalize → Disfluency → Alias → Deterministic
//	→ Pinyin → Fuzzy → Context
//
// This is the recommended starting point for most applications.
func Standard(lex lexicon.Lexicon, conv lexicon.PinyinConverter) *lexnorm.Preset {
	p := lexnorm.NewPipeline(
		normalize.New(),
		disfluency.New(),
		alias.New(lex),
		deterministic.New(lex),
		pinyin.New(lex, conv),
		fuzzy.New(lex),
		ctxproc.New(),
	)
	return lexnorm.NewPreset(
		"standard",
		"Full pipeline for general text normalization",
		p,
		lexnorm.DefaultConfig(),
	)
}

// HighAccuracy returns the Standard Pipeline with tighter thresholds
// (lower AutoApplyThreshold, higher SuggestThreshold) for
// applications where precision is critical.
//
// Use this when downstream consumers need a more conservative
// normalization (fewer false positives, more Suggestions for review).
func HighAccuracy(lex lexicon.Lexicon, conv lexicon.PinyinConverter) *lexnorm.Preset {
	p := lexnorm.NewPipeline(
		normalize.New(),
		disfluency.New(),
		alias.New(lex),
		deterministic.New(lex),
		pinyin.New(lex, conv),
		fuzzy.New(lex),
		ctxproc.New(),
	)
	cfg := lexnorm.DefaultConfig()
	cfg.AutoApplyThreshold = 0.85
	cfg.SuggestThreshold = 0.50
	return lexnorm.NewPreset(
		"high-accuracy",
		"Standard pipeline with conservative thresholds (more Suggestions, fewer Applies)",
		p,
		cfg,
	)
}

// Fast returns a minimal Pipeline for latency-sensitive applications.
//
// Pipeline:
//
//	Normalize → Alias
//
// Skips Disfluency / Deterministic / Pinyin / Fuzzy / Context for
// minimum overhead.
func Fast(lex lexicon.Lexicon) *lexnorm.Preset {
	p := lexnorm.NewPipeline(
		normalize.New(),
		alias.New(lex),
	)
	return lexnorm.NewPreset(
		"fast",
		"Minimal pipeline (Normalize + Alias) for low-latency use cases",
		p,
		lexnorm.DefaultConfig(),
	)
}

// ASR returns a Pipeline tuned for ASR transcript correction.
//
// Pipeline:
//
//	Normalize → Disfluency → Alias → Pinyin
//
// Emphasizes disfluency removal (filler words) and homophone matching,
// which are the dominant error classes for ASR output.
func ASR(lex lexicon.Lexicon, conv lexicon.PinyinConverter) *lexnorm.Preset {
	p := lexnorm.NewPipeline(
		normalize.New(),
		disfluency.New(),
		alias.New(lex),
		pinyin.New(lex, conv),
	)
	return lexnorm.NewPreset(
		"asr",
		"Pipeline tuned for ASR transcripts (Disfluency + Pinyin emphasis)",
		p,
		lexnorm.DefaultConfig(),
	)
}

// OCR returns a Pipeline tuned for OCR text correction.
//
// Pipeline:
//
//	Normalize → Alias → Deterministic
//
// Emphasizes alias / deterministic correction (typos introduced by
// OCR), without pinyin or fuzzy (which assume ASR-like patterns).
func OCR(lex lexicon.Lexicon) *lexnorm.Preset {
	p := lexnorm.NewPipeline(
		normalize.New(),
		alias.New(lex),
		deterministic.New(lex),
	)
	return lexnorm.NewPreset(
		"ocr",
		"Pipeline tuned for OCR text (Alias + Deterministic emphasis)",
		p,
		lexnorm.DefaultConfig(),
	)
}
