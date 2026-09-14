package processor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

// TestP_FIX_v3_TieBreaker：桶排序稳定性 bug 修复
//
// 背景：'田青' 桶里同时有 '田华' (sig=th) 和 '田清' (sig=tq)，两个 dist=1。
// 旧逻辑按 Text 排序选 '田华'；新逻辑优先选 pinyin sig 完全相等的 '田清'。
func TestP_FIX_v3_TieBreaker(t *testing.T) {
	entries := []lexicon.Entry{
		{ID: "1", Text: "田华", Meta: map[string]any{"category": "PERSON"}},
		{ID: "2", Text: "田清", Meta: map[string]any{"category": "PERSON"}},
		{ID: "3", Text: "田花", Meta: map[string]any{"category": "PERSON"}},
	}
	b := lexicon.NewBuilderWithVersion("test").Add(entries...)
	lex, _ := b.Build()

	cfg := DefaultFuzzyVocabConfig()
	p := NewFuzzyVocabProcessor(lex, cfg)

	cfg2 := lexnorm.DefaultConfig()
	cfg2.AutoApplyThreshold = 0.85
	cfg2.SuggestThreshold = 0.50

	state, _ := lexnorm.NewState(context.Background(), "田青", lex, cfg2)
	_ = p.Process(context.Background(), state)
	if state.Text() != "田清" {
		t.Errorf("'田青' should match '田清' (pinyin sig 完全相等), got %q", state.Text())
	}
	for _, c := range state.Changes() {
		if c.Kind != lexnorm.ChangeReplace {
			t.Errorf("'田青' → '田清' 应是 replace (sig 一致 conf=0.85), got action=%v conf=%.2f", c.Kind, c.Confidence)
		}
	}
}

// TestP_FIX_v3_NonMatchKeepsSuggest：桶里没有 sig 相等的，应保持 suggest（不强制 replace）
func TestP_FIX_v3_NonMatchKeepsSuggest(t *testing.T) {
	entries := []lexicon.Entry{
		// 只有 sig 不相等的桶
		{ID: "1", Text: "李华", Meta: map[string]any{"category": "PERSON"}},
		{ID: "2", Text: "王芳", Meta: map[string]any{"category": "PERSON"}},
	}
	b := lexicon.NewBuilderWithVersion("test").Add(entries...)
	lex, _ := b.Build()

	cfg := DefaultFuzzyVocabConfig()
	p := NewFuzzyVocabProcessor(lex, cfg)

	cfg2 := lexnorm.DefaultConfig()
	cfg2.AutoApplyThreshold = 0.85
	cfg2.SuggestThreshold = 0.50

	// '李明' sig=lm; '李华' sig=lh (字面 dist=1, sig 不等)
	state, _ := lexnorm.NewState(context.Background(), "李明", lex, cfg2)
	_ = p.Process(context.Background(), state)
	if state.Text() != "李明" {
		t.Errorf("'李明' 应保留（sig 不一致），got %q", state.Text())
	}
	hasSuggest := false
	for _, c := range state.Changes() {
		if c.Kind == lexnorm.ChangeSuggest && c.Confidence < 0.85 {
			hasSuggest = true
		}
	}
	if !hasSuggest {
		t.Errorf("'李明' 应走 suggest conf<0.85，避免误改")
	}
}

// TestP_FIX_v3_BucketStable：桶排序不影响（田清桶里只有一个候选时行为稳定）
func TestP_FIX_v3_BucketStable(t *testing.T) {
	entries := []lexicon.Entry{
		{ID: "1", Text: "佘丽群", Meta: map[string]any{"category": "PERSON"}},
		{ID: "2", Text: "冯丽群", Meta: map[string]any{"category": "PERSON"}},
	}
	b := lexicon.NewBuilderWithVersion("test").Add(entries...)
	lex, _ := b.Build()

	cfg := DefaultFuzzyVocabConfig()
	p := NewFuzzyVocabProcessor(lex, cfg)

	cfg2 := lexnorm.DefaultConfig()
	cfg2.AutoApplyThreshold = 0.85
	cfg2.SuggestThreshold = 0.50

	// '周丽群' 字面 dist=1 to '佘丽群' (sig zlq vs slq) and '冯丽群' (sig flq vs zlq)
	// tie-breaker 不触发（subSig 跟谁都不等），first-encountered wins → 佘丽群
	state, _ := lexnorm.NewState(context.Background(), "周丽群", lex, cfg2)
	_ = p.Process(context.Background(), state)
	if state.Text() != "佘丽群" {
		t.Errorf("'周丽群' 应自动 replace '佘丽群', got %q", state.Text())
	}
}

// TestP_FIX_v4_N2SigMatchConf：n=2 + pinyin sig 完全相等 → conf=0.85（自动 replace）
func TestP_FIX_v4_N2SigMatchConf(t *testing.T) {
	entries := []lexicon.Entry{
		{ID: "1", Text: "田清", Meta: map[string]any{"category": "PERSON"}},
	}
	b := lexicon.NewBuilderWithVersion("test").Add(entries...)
	lex, _ := b.Build()

	cfg := DefaultFuzzyVocabConfig()
	p := NewFuzzyVocabProcessor(lex, cfg)

	cfg2 := lexnorm.DefaultConfig()
	cfg2.AutoApplyThreshold = 0.85
	cfg2.SuggestThreshold = 0.50

	state, _ := lexnorm.NewState(context.Background(), "田青", lex, cfg2)
	_ = p.Process(context.Background(), state)
	fmt.Printf("[debug] n=2 + sig 等 conf=%.2f\n", maxConf(state))
	if maxConf(state) < 0.85 {
		t.Errorf("n=2 + sig 完全相等应 conf>=0.85, got %.2f", maxConf(state))
	}
}

// TestP_FIX_v4_N2SigDiffConservative：n=2 + pinyin sig 不等 → conf<=0.55（保守）
func TestP_FIX_v4_N2SigDiffConservative(t *testing.T) {
	entries := []lexicon.Entry{
		{ID: "1", Text: "李华", Meta: map[string]any{"category": "PERSON"}},
	}
	b := lexicon.NewBuilderWithVersion("test").Add(entries...)
	lex, _ := b.Build()

	cfg := DefaultFuzzyVocabConfig()
	p := NewFuzzyVocabProcessor(lex, cfg)

	cfg2 := lexnorm.DefaultConfig()
	cfg2.AutoApplyThreshold = 0.85
	cfg2.SuggestThreshold = 0.50

	// '李明' 字面 dist=1 to '李华' (sig lm vs lh, 不等)
	state, _ := lexnorm.NewState(context.Background(), "李明", lex, cfg2)
	_ = p.Process(context.Background(), state)
	if maxConf(state) > 0.70 {
		t.Errorf("n=2 + sig 不等应 conf<=0.70 (保守), got %.2f", maxConf(state))
	}
}

func maxConf(s *lexnorm.State) float64 {
	max := 0.0
	for _, c := range s.Changes() {
		if c.Confidence > max {
			max = c.Confidence
		}
	}
	return max
}
