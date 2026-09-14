package processor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

// TestP10_TwoCharNoiseIsolation 验证 2 字子串 fuzzy 不再噪音轰炸。
//
// 修复前问题：2 字子串 vs 任意 2 字词条 Hamming 距离 ≤ 2，
// maxDist=2 一律命中 → 所有 2 字滑动窗口都被 suggest 同一个桶排序第一的 PERSON。
//
// 修复后：2 字 effectiveMaxDist=1 + pinyin 兜底关闭 + conf 阶梯。
func TestP10_TwoCharNoiseIsolation(t *testing.T) {
	// 模拟 qua 真实 PERSON 词典（含大量 2 字姓名 + 脏数据）
	persons := []string{
		"龚千友", "冯春晓", "龚建军", "伍锡辉", "何焓", "林宇豪", "阳巡宇",
		"邓梓", "陈兴静", "陈科沆", "朱凤", "佘丽群", "杨城冰月",
		"田清", "田花", "袁孟莲", "王婷", "冯渡", "向中华", "向鑫",
		"李云龙", "孙策", "郭源潮", "肖可可", "华子哥", "杜岩",
		"叶海嫣", "贺卡计划", "可靠", "小河", "阿松大",
		"入职咯", "测试账号", "张三", "李四",
	}
	b := lexicon.NewBuilder()
	for i, p := range persons {
		b.Add(lexicon.Entry{
			ID:   lexicon.EntryID(fmt.Sprintf("p%d", i)),
			Text: p,
			Meta: map[string]any{"category": "PERSON"},
		})
	}
	lex, _ := b.Build()
	cfg := lexnorm.Config{AutoApplyThreshold: 0.85, SuggestThreshold: 0.50}
	p := NewFuzzyVocabProcessor(lex, DefaultFuzzyVocabConfig())

	tests := []struct {
		input string
	}{
		{"周丽群汇报"},      // 4 字含 3 字目标
		{"腾讯"},            // 2 字普通词（不应被 suggest）
		{"袁梦莲和羊城冰月一起开会"}, // 多个 3/4 字目标
		{"问题"},            // 2 字普通词（不应被 suggest）
	}

	for _, tt := range tests {
		s, _ := lexnorm.NewState(context.Background(), tt.input, lex, cfg)
		_ = p.Process(context.Background(), s)

		t.Logf("=== %q → %q ===", tt.input, s.Text())
		// 统计 suggest 数
		suggests := 0
		replaces := 0
		for _, c := range s.Changes() {
			if c.Action == lexnorm.ActionSuggest {
				suggests++
				t.Logf("  SUGGEST %q → %q (conf=%.2f)", c.From, c.To, c.Confidence)
			} else if c.Action == lexnorm.ActionReplace {
				replaces++
				t.Logf("  REPLACE %q → %q (conf=%.2f)", c.From, c.To, c.Confidence)
			}
		}
		t.Logf("  → replaces=%d suggests=%d", replaces, suggests)

		// 关键断言 1：2 字普通词不应有 suggest（"腾讯"、"问题"）
		if len([]rune(tt.input)) == 2 {
			if suggests > 0 {
				t.Errorf("2 字普通词 %q 不应有 suggest，实际 %d 个", tt.input, suggests)
			}
		}

		// 关键断言 2：不应有 conf=0.55（dist=2/n=2）的 2 字 suggest
		for _, c := range s.Changes() {
			if len([]rune(c.From)) == 2 && c.Confidence < 0.55 {
				t.Errorf("2 字 suggest conf 异常低: %q → %q conf=%.2f", c.From, c.To, c.Confidence)
			}
		}
	}
}

// TestP10_ThreeCharHighQualityReplace 验证 3 字 dist=1 仍能自动 Apply。
func TestP10_ThreeCharHighQualityReplace(t *testing.T) {
	b := lexicon.NewBuilder()
	b.Add(lexicon.Entry{
		ID:   "1",
		Text: "佘丽群",
		Meta: map[string]any{"category": "PERSON"},
	})
	b.Add(lexicon.Entry{
		ID:   "2",
		Text: "何焓",
		Meta: map[string]any{"category": "PERSON"},
	})
	lex, _ := b.Build()
	cfg := lexnorm.Config{AutoApplyThreshold: 0.85, SuggestThreshold: 0.50}
	p := NewFuzzyVocabProcessor(lex, DefaultFuzzyVocabConfig())

	s, _ := lexnorm.NewState(context.Background(), "周丽群", lex, cfg)
	_ = p.Process(context.Background(), s)

	// 期望：apply "周丽群" → "佘丽群"（3 字 dist=1 conf=0.95）
	found := false
	for _, c := range s.Changes() {
		if c.From == "周丽群" && c.To == "佘丽群" && c.Action == lexnorm.ActionReplace {
			found = true
		}
	}
	if !found {
		t.Errorf("期望 fuzzy 自动 apply '周丽群' → '佘丽群'，实际 changes: %+v", s.Changes())
	}
}

// TestP10_TwoCharSuggestKeepsQuality 验证 2 字 ASR 错字仍能 suggest（不 Apply）。
func TestP10_TwoCharSuggestKeepsQuality(t *testing.T) {
	b := lexicon.NewBuilder()
	b.Add(lexicon.Entry{
		ID:   "1",
		Text: "佘丽",
		Meta: map[string]any{"category": "PERSON"},
	})
	lex, _ := b.Build()
	cfg := lexnorm.Config{AutoApplyThreshold: 0.85, SuggestThreshold: 0.50}
	p := NewFuzzyVocabProcessor(lex, DefaultFuzzyVocabConfig())

	s, _ := lexnorm.NewState(context.Background(), "周丽", lex, cfg)
	_ = p.Process(context.Background(), s)

	// 期望：suggest "周丽" → "佘丽"（dist=1, conf=0.70）
	found := false
	for _, c := range s.Changes() {
		if c.From == "周丽" && c.To == "佘丽" && c.Action == lexnorm.ActionSuggest {
			found = true
		}
	}
	if !found {
		t.Errorf("期望 fuzzy suggest '周丽' → '佘丽'，实际 changes: %+v", s.Changes())
	}

	// 关键：不应 Apply（2 字不应自动改）
	for _, c := range s.Changes() {
		if c.From == "周丽" && c.To == "佘丽" && c.Action == lexnorm.ActionReplace {
			t.Errorf("2 字不应自动 Apply，但发生了: %+v", c)
		}
	}
}
