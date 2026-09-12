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
	"strings"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/pinyin"
	"github.com/stack-haven/lexnorm/processor/presets"
)

// --- 4f: filler removal consumes one adjacent space (no double space) ---

func TestDisfluency_NoDoubleSpace(t *testing.T) {
	e, _ := lexnorm.New(lexnorm.WithPreset(*presets.Standard(mustLexicon(t), nil)))
	res, err := e.Normalize(context.Background(), "你好 嗯 世界")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, "  ") {
		t.Errorf("double space remains: %q", res.Text)
	}
	if res.Text != "你好 世界" {
		t.Errorf("text = %q, want 你好 世界", res.Text)
	}
}

// --- 4h: noise safety policy ---

func TestDisfluency_AmbiguousFillerGuarded(t *testing.T) {
	// Spec §2 counter-example: "那个" in "那个文件给我" must survive —
	// it is a demonstrative directly attached to a noun.
	p := disfluency.New()
	st, _ := lexnorm.NewState(context.Background(), "那个文件给我", nil, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if st.Text() != "那个文件给我" {
		t.Errorf("text = %q, want unchanged", st.Text())
	}
	// Standalone usage is still removed.
	st2, _ := lexnorm.NewState(context.Background(), "嗯，那个 ，继续", nil, lexnorm.DefaultConfig())
	_ = p.Process(context.Background(), st2)
	if strings.Contains(st2.Text(), "那个") {
		t.Errorf("standalone 那个 should be removed: %q", st2.Text())
	}
}

func TestDisfluency_AggressiveMode_RestoresLegacy(t *testing.T) {
	p := disfluency.New().WithAggressiveFillers()
	st, _ := lexnorm.NewState(context.Background(), "那个文件给我", nil, lexnorm.DefaultConfig())
	_ = p.Process(context.Background(), st)
	if st.Text() != "文件给我" {
		t.Errorf("aggressive mode should remove 那个 unconditionally: %q", st.Text())
	}
}

func TestDisfluency_RepeatedRun_SuggestOnly(t *testing.T) {
	// 哈 is NOT a filler: the run is left intact, only a collapse
	// suggestion is emitted (folding laughter automatically would be
	// wrong — that judgment is context).
	p := disfluency.New()
	st, _ := lexnorm.NewState(context.Background(), "哈哈哈哈这个测试", nil, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if st.Text() != "哈哈哈哈这个测试" {
		t.Errorf("repeated-run detection must not modify text: %q", st.Text())
	}
	sugg := 0
	for _, c := range st.Changes() {
		if c.Applied {
			t.Error("repeated-run suggestion must not be applied")
		}
		if c.To == "哈" {
			sugg++
		}
	}
	if sugg != 1 {
		t.Errorf("expected one collapse suggestion, changes = %+v", st.Changes())
	}
}

func TestDisfluency_FillerRunRemoved(t *testing.T) {
	// Contrast: 啊 IS an unambiguous filler, so a stutter run of it is
	// removed by the filler policy (legacy behavior preserved).
	p := disfluency.New()
	st, _ := lexnorm.NewState(context.Background(), "啊啊啊啊这个测试", nil, lexnorm.DefaultConfig())
	_ = p.Process(context.Background(), st)
	if st.Text() != "这个测试" {
		t.Errorf("filler run should be removed: %q", st.Text())
	}
}

func TestDisfluency_RepeatedPhrase_SuggestOnly(t *testing.T) {
	p := disfluency.New()
	st, _ := lexnorm.NewState(context.Background(), "好的好的好的，明白了", nil, lexnorm.DefaultConfig())
	_ = p.Process(context.Background(), st)
	found := false
	for _, c := range st.Changes() {
		if c.To == "好的" && !c.Applied {
			found = true
		}
	}
	if !found {
		t.Errorf("expected phrase collapse suggestion, changes = %+v", st.Changes())
	}
}

// --- 4e: phonetic processor ---

// perSyllableConverter mimics the sanctioned multi-form converter
// evolution path (one pinyin form per character).
type perSyllableConverter struct{}

func (perSyllableConverter) ToPinyin(text string) []string {
	var forms []string
	for _, r := range text {
		if p, ok := map[rune]string{
			'金': "jin", '种': "zhong", '籽': "zi", '仔': "zi",
			'袁': "yuan", '孟': "meng", '梦': "meng", '莲': "lian",
		}[r]; ok {
			forms = append(forms, p)
		}
	}
	return forms
}

func TestPinyin_MultiCharEntry_NeverCharReplaced(t *testing.T) {
	// Regression: a multi-character entry reached through the
	// per-character path must never replace a single character with the
	// whole canonical text (text corruption).
	lex, _ := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e1", Text: "金种籽",
			Variants: []lexicon.Variant{{Text: "x", Kind: lexicon.VariantHomophone, Confidence: 0.97}}},
	).Build()
	p := pinyin.New(lex, perSyllableConverter{})
	st, _ := lexnorm.NewState(context.Background(), "金种籽项目启动", lex, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if st.Text() != "金种籽项目启动" {
		t.Errorf("canonical input corrupted: %q", st.Text())
	}
}

func TestPinyin_HomophoneWholeWordConsumed(t *testing.T) {
	// D-2 fix: Variant{Homophone}.Text is now consumed (whole-word).
	lex, _ := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e-homo", Text: "袁孟莲",
			Variants: []lexicon.Variant{{Text: "袁梦莲", Kind: lexicon.VariantHomophone, Confidence: 0.9}}},
	).Build()
	p := pinyin.New(lex, nil)
	st, _ := lexnorm.NewState(context.Background(), "袁梦莲来了", lex, lexnorm.DefaultConfig())
	if err := p.Process(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	// conf 0.9 < AutoApplyThreshold(0.95) → Suggest.
	sugg := 0
	for _, c := range st.Changes() {
		if c.From == "袁梦莲" && c.To == "袁孟莲" && !c.Applied {
			sugg++
		}
	}
	if sugg != 1 {
		t.Errorf("expected one homophone suggestion, changes = %+v", st.Changes())
	}
	// High confidence → applied.
	lex2, _ := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e-homo", Text: "袁孟莲",
			Variants: []lexicon.Variant{{Text: "袁梦莲", Kind: lexicon.VariantHomophone, Confidence: 0.97}}},
	).Build()
	p2 := pinyin.New(lex2, nil)
	st2, _ := lexnorm.NewState(context.Background(), "袁梦莲来了", lex2, lexnorm.DefaultConfig())
	_ = p2.Process(context.Background(), st2)
	if st2.Text() != "袁孟莲来了" {
		t.Errorf("high-confidence homophone should apply: %q", st2.Text())
	}
}

func TestPinyin_SubstringHomophoneVariantSkipped(t *testing.T) {
	// A homophone variant that is a substring of its canonical would
	// duplicate the canonical; it must be ignored.
	lex, _ := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e1", Text: "万康盛鼎集团",
			Variants: []lexicon.Variant{{Text: "万康盛鼎", Kind: lexicon.VariantHomophone, Confidence: 0.99}}},
	).Build()
	p := pinyin.New(lex, nil)
	st, _ := lexnorm.NewState(context.Background(), "万康盛鼎集团开会", lex, lexnorm.DefaultConfig())
	_ = p.Process(context.Background(), st)
	if st.Text() != "万康盛鼎集团开会" {
		t.Errorf("substring homophone variant corrupted text: %q", st.Text())
	}
}

// --- 4g: panic recovery (default, not opt-in) ---

type boomProcessor struct{}

func (boomProcessor) Name() string { return "panic" }
func (boomProcessor) Process(_ context.Context, _ *lexnorm.State) error {
	panic("boom")
}

func TestEngine_PanicDegradesNotCrashes(t *testing.T) {
	lex, _ := lexicon.NewBuilder().Build()
	e, err := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(lexnorm.NewPipeline(boomProcessor{})),
	)
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Normalize(context.Background(), "原始文本")
	if err == nil {
		t.Fatal("panic must surface as an error")
	}
	if !errors.Is(err, lexnorm.ErrRuntime) {
		t.Errorf("err should wrap ErrRuntime: %v", err)
	}
	if res.Original != "原始文本" {
		t.Errorf("original text must be preserved: %q", res.Original)
	}
	if res.Status != lexnorm.StatusFailed && res.Status != lexnorm.StatusPartial {
		t.Errorf("status = %v, want failed/partial", res.Status)
	}
}
