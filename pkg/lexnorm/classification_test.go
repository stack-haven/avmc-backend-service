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

// Classification tests: every built-in Processor must declare the
// correct formal Category, DefaultOrder, and capability metadata
// (spec §12 分类测试 / Descriptor 测试).

import (
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/ctxproc"
	"github.com/stack-haven/lexnorm/processor/deterministic"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/fuzzy"
	"github.com/stack-haven/lexnorm/processor/llm"
	"github.com/stack-haven/lexnorm/processor/normalize"
	"github.com/stack-haven/lexnorm/processor/pinyin"
)

func TestBuiltinProcessor_Classification(t *testing.T) {
	cases := []struct {
		name         string
		proc         lexnorm.Processor
		category     lexnorm.Category
		order        int
		mutatesText  bool
		supportsSugg bool
		determinstic bool
	}{
		{"normalize", normalize.New(), lexnorm.CategoryNormalization, 1, true, false, true},
		{"disfluency", disfluency.New(), lexnorm.CategoryNoise, 2, true, true, true},
		{"alias", alias.New(nil), lexnorm.CategoryCanonical, 3, true, false, true},
		{"deterministic", deterministic.New(nil), lexnorm.CategoryDeterministic, 4, true, false, true},
		{"pinyin", pinyin.New(nil, nil), lexnorm.CategoryPhonetic, 5, true, true, true},
		{"fuzzy", fuzzy.New(nil), lexnorm.CategoryApproximate, 6, true, true, true},
		{"context", ctxproc.New(), lexnorm.CategoryContextual, 7, false, true, true},
		{"llm", llm.New(), lexnorm.CategorySemantic, 8, false, false, false},
	}
	for _, c := range cases {
		d, ok := lexnorm.DescriptorOf(c.proc)
		if !ok {
			t.Errorf("%s: built-in Processor must be fully described", c.name)
			continue
		}
		if d.Category != c.category {
			t.Errorf("%s: Category = %q, want %q", c.name, d.Category, c.category)
		}
		if d.DefaultOrder != c.order {
			t.Errorf("%s: DefaultOrder = %d, want %d", c.name, d.DefaultOrder, c.order)
		}
		if d.MutatesText != c.mutatesText {
			t.Errorf("%s: MutatesText = %v, want %v", c.name, d.MutatesText, c.mutatesText)
		}
		if d.SupportsSuggest != c.supportsSugg {
			t.Errorf("%s: SupportsSuggest = %v, want %v", c.name, d.SupportsSuggest, c.supportsSugg)
		}
		if d.Deterministic != c.determinstic {
			t.Errorf("%s: Deterministic = %v, want %v", c.name, d.Deterministic, c.determinstic)
		}
		if d.Name != c.name {
			t.Errorf("%s: Descriptor.Name = %q", c.name, d.Name)
		}
		if d.Version == "" {
			t.Errorf("%s: Descriptor.Version must be declared", c.name)
		}
	}
}

// TestBuiltinProcessor_DefaultOrderMatchesSpec ensures the declared
// DefaultOrder values reproduce the spec §7 default pipeline sequence.
func TestBuiltinProcessor_DefaultOrderMatchesSpec(t *testing.T) {
	procs := []lexnorm.Processor{
		normalize.New(), disfluency.New(), alias.New(nil),
		deterministic.New(nil), pinyin.New(nil, nil), fuzzy.New(nil),
		ctxproc.New(), llm.New(),
	}
	prev := 0
	for _, p := range procs {
		d, _ := lexnorm.DescriptorOf(p)
		if d.DefaultOrder <= prev {
			t.Errorf("%s DefaultOrder %d must be > previous %d (spec §7 order)",
				d.Name, d.DefaultOrder, prev)
		}
		prev = d.DefaultOrder
	}
}
