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

// Business-scenario tests for the spec's execution order Step 7:
//
//	ASR:   小田 → 田华,  个种子 → 颗种籽
//	会议:  张总 → 张强,  老王 → 王强,  小田 → 田华

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/presets"
)

func scenarioEngine(t *testing.T) *lexnorm.Engine {
	t.Helper()
	lex, err := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e-tianhua", Text: "田华",
			Variants: []lexicon.Variant{
				{Text: "小田", Kind: lexicon.VariantAlias, Confidence: 0.9, Source: "manual-title"},
			}},
		lexicon.Entry{ID: "e-zhangqiang", Text: "张强",
			Variants: []lexicon.Variant{
				{Text: "张总", Kind: lexicon.VariantAlias, Confidence: 0.85, Source: "manual-title"},
			}},
		lexicon.Entry{ID: "e-wangqiang", Text: "王强",
			Variants: []lexicon.Variant{
				{Text: "老王", Kind: lexicon.VariantAlias, Confidence: 0.85, Source: "manual-title"},
			}},
		lexicon.Entry{ID: "e-kezhongzi", Text: "颗种籽",
			Variants: []lexicon.Variant{
				{Text: "个种子", Kind: lexicon.VariantCorrection, Confidence: 1.0, Source: "confirmed-map"},
			}},
	).Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := lexnorm.New(lexnorm.WithPreset(*presets.Standard(lex, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestBusinessScenario_ASR(t *testing.T) {
	e := scenarioEngine(t)
	res, err := e.Normalize(context.Background(), "小田今天帮我查一下个种子的情况")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "田华今天帮我查一下颗种籽的情况" {
		t.Fatalf("ASR scenario: text = %q", res.Text)
	}
	changes := map[string]string{}
	for _, c := range res.Changes {
		if c.Applied {
			changes[c.From] = c.To
		}
	}
	if changes["小田"] != "田华" || changes["个种子"] != "颗种籽" {
		t.Errorf("ASR scenario changes = %v", changes)
	}
	// Traceability: alias change must carry the lexicon EntryID.
	for _, c := range res.Changes {
		if c.From == "小田" && c.EntryID != "e-tianhua" {
			t.Errorf("alias change EntryID = %q, want e-tianhua", c.EntryID)
		}
		if c.From == "个种子" && c.RuleID != "deterministic" {
			t.Errorf("correction change RuleID = %q", c.RuleID)
		}
	}
}

func TestBusinessScenario_Meeting(t *testing.T) {
	e := scenarioEngine(t)
	res, err := e.Normalize(context.Background(), "张总让老王和小田对齐一下个种子的进度")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "张强让王强和田华对齐一下颗种籽的进度" {
		t.Fatalf("meeting scenario: text = %q", res.Text)
	}
	// Multiple targets in one sentence, all traced.
	applied := 0
	for _, c := range res.Changes {
		if c.Applied {
			applied++
		}
	}
	if applied != 4 {
		t.Errorf("applied changes = %d, want 4 (张总/老王/小田/个种子)", applied)
	}
}

// TestBusinessScenario_SuggestNotApplied covers the "颗种籽" homophone
// path degrading to Suggest below the auto-apply threshold (spec §10
// Phonetic / Approximate uncertainty policy).
func TestBusinessScenario_PhoneticSuggestsBelowThreshold(t *testing.T) {
	lex, err := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e-kezhongzi", Text: "颗种籽",
			Variants: []lexicon.Variant{
				{Text: "科种籽", Kind: lexicon.VariantHomophone, Confidence: 0.8},
			}},
	).Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := lexnorm.New(lexnorm.WithPreset(*presets.Standard(lex, nil)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Normalize(context.Background(), "科种籽发芽了")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "科种籽发芽了" {
		t.Fatalf("homophone conf 0.8 must not auto-apply: %q", res.Text)
	}
	if len(res.Suggestions) != 1 || res.Suggestions[0].To != "颗种籽" {
		t.Fatalf("expected one Suggestion 颗种籽, got %+v", res.Suggestions)
	}
}
