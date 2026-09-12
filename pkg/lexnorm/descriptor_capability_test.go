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
	"context"
	"encoding/json"
	"testing"
)

func TestCategory_Values(t *testing.T) {
	want := map[Category]string{
		CategoryNormalization: "normalization",
		CategoryNoise:         "noise",
		CategoryCanonical:     "canonicalization",
		CategoryDeterministic: "deterministic",
		CategoryPhonetic:      "phonetic",
		CategoryApproximate:   "approximate",
		CategoryContextual:    "contextual",
		CategorySemantic:      "semantic",
	}
	for cat, s := range want {
		if string(cat) != s {
			t.Errorf("Category(%v) string = %q, want %q", cat, string(cat), s)
		}
	}
}

func TestCategory_DefaultOrder(t *testing.T) {
	// Spec §7 default pipeline order.
	order := []Category{
		CategoryNormalization, CategoryNoise, CategoryCanonical,
		CategoryDeterministic, CategoryPhonetic, CategoryApproximate,
		CategoryContextual, CategorySemantic,
	}
	for i, cat := range order {
		if got := cat.DefaultOrder(); got != i+1 {
			t.Errorf("%v.DefaultOrder() = %d, want %d", cat, got, i+1)
		}
	}
	if Category("bogus").DefaultOrder() != 99 {
		t.Error("unknown category should sort last")
	}
}

func TestMutationMode_And_Determinism_Values(t *testing.T) {
	if MutationNone != "none" || MutationSuggest != "suggest" ||
		MutationApply != "apply" || MutationMixed != "mixed" {
		t.Error("MutationMode constant values drifted")
	}
	if DeterministicTrue != "deterministic" ||
		DeterministicProbabilistic != "probabilistic" ||
		DeterministicGenerative != "generative" {
		t.Error("Determinism constant values drifted")
	}
}

func TestDescriptor_MutationView(t *testing.T) {
	cases := []struct {
		d    Descriptor
		want MutationMode
	}{
		{Descriptor{}, MutationNone},
		{Descriptor{MutatesText: true}, MutationApply},
		{Descriptor{SupportsSuggest: true}, MutationSuggest},
		{Descriptor{MutatesText: true, SupportsSuggest: true}, MutationMixed},
	}
	for i, c := range cases {
		if got := c.d.Mutation(); got != c.want {
			t.Errorf("case %d: Mutation() = %v, want %v", i, got, c.want)
		}
	}
}

// plainProcessor satisfies only Processor (no metadata interfaces).
type plainProcessor struct{}

func (plainProcessor) Name() string { return "plain" }

func (plainProcessor) Process(_ context.Context, _ *State) error { return nil }

// fullProcessor additionally implements Versioner + CertaintyReporter.
type fullProcessor struct{ plainProcessor }

func (fullProcessor) Version() string      { return "v9" }
func (fullProcessor) Certainty() Certainty { return CertaintyLow }

// describedProcessor implements DescribedProcessor.
type describedProcessor struct{ fullProcessor }

func (describedProcessor) Descriptor() Descriptor {
	return Descriptor{
		Name:          "described",
		Version:       "v1",
		Category:      CategoryNoise,
		Certainty:     CertaintyHigh,
		MutatesText:   true,
		Deterministic: true,
	}
}

func TestDescriptorOf_FullyDescribed(t *testing.T) {
	d, ok := DescriptorOf(describedProcessor{})
	if !ok {
		t.Fatal("DescribedProcessor should return ok=true")
	}
	if d.Name != "described" || d.Category != CategoryNoise || d.Version != "v1" {
		t.Errorf("unexpected descriptor: %+v", d)
	}
}

func TestDescriptorOf_PartialInference(t *testing.T) {
	d, ok := DescriptorOf(fullProcessor{})
	if ok {
		t.Error("plain Processor should return ok=false")
	}
	if d.Name != "plain" || d.Version != "v9" || d.Certainty != CertaintyLow {
		t.Errorf("partial descriptor mismatch: %+v", d)
	}
}

func TestDescriptorOf_Minimal(t *testing.T) {
	d, ok := DescriptorOf(plainProcessor{})
	if ok {
		t.Error("minimal Processor should return ok=false")
	}
	if d.Name != "plain" {
		t.Errorf("Name = %q, want plain", d.Name)
	}
	if _, ok := DescriptorOf(nil); ok {
		t.Error("nil Processor should return ok=false")
	}
}

// TestDescriptor_KeyedLiteralsStillCompile guards the compat guarantee:
// extending Descriptor with capability fields must not break existing
// keyed-literal construction (the shape every processor package uses).
func TestDescriptor_KeyedLiteralsStillCompile(t *testing.T) {
	d := Descriptor{
		Name:      "x",
		Certainty: CertaintyHigh,
		New:       func(_ json.RawMessage) (Processor, error) { return plainProcessor{}, nil },
		Default:   func() any { return nil },
	}
	if d.Name != "x" || d.Category != "" {
		t.Errorf("unexpected zero-value behavior: %+v", d)
	}
}
