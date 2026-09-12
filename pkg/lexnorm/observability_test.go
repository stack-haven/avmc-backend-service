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
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/ctxproc"
	"github.com/stack-haven/lexnorm/processor/normalize"
	"github.com/stack-haven/lexnorm/processor/presets"
)

func mustLexicon(t *testing.T) lexicon.Lexicon {
	t.Helper()
	lex, err := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e1", Text: "田华"},
	).Build()
	if err != nil {
		t.Fatal(err)
	}
	return lex
}

// TestStepTiming_CarriesCategory verifies the engine surfaces the
// declared capability metadata in Result.Steps (P3 observability).
func TestStepTiming_CarriesCategory(t *testing.T) {
	e, err := lexnorm.New(lexnorm.WithPreset(*presets.Fast(nil)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Normalize(context.Background(), " 田工 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("Steps = %d, want 2 (normalize + alias)", len(res.Steps))
	}
	wantCat := map[string]lexnorm.Category{
		"normalize": lexnorm.CategoryNormalization,
		"alias":     lexnorm.CategoryCanonical,
	}
	for _, st := range res.Steps {
		if st.Category != wantCat[st.Processor] {
			t.Errorf("step %s: Category = %q, want %q", st.Processor, st.Category, wantCat[st.Processor])
		}
		if !st.Deterministic {
			t.Errorf("step %s: Deterministic = false, want true", st.Processor)
		}
	}
}

// TestRuntimeInfo_CarriesCategories verifies RuntimeInfo exposes the
// per-Processor category map.
func TestRuntimeInfo_CarriesCategories(t *testing.T) {
	pipeline := lexnorm.NewPipeline(normalize.New(), ctxproc.New())
	e, err := lexnorm.New(lexnorm.WithPipeline(pipeline), lexnorm.WithLexicon(mustLexicon(t)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Normalize(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	rt := res.Runtime
	if rt.ProcessorCategories["normalize"] != string(lexnorm.CategoryNormalization) {
		t.Errorf("ProcessorCategories[normalize] = %q", rt.ProcessorCategories["normalize"])
	}
	if rt.ProcessorCategories["context"] != string(lexnorm.CategoryContextual) {
		t.Errorf("ProcessorCategories[context] = %q", rt.ProcessorCategories["context"])
	}
	// Defensive copy: mutating the returned map must not affect the engine.
	rt.ProcessorCategories["normalize"] = "tampered"
	res2, _ := e.Normalize(context.Background(), "hello")
	if res2.Runtime.ProcessorCategories["normalize"] != string(lexnorm.CategoryNormalization) {
		t.Error("RuntimeInfo categories map is not defensively copied")
	}
}
