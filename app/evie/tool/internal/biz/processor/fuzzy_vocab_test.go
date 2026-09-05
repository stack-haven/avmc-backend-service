// Package processor · fuzzy_vocab_test.go
// fuzzy_vocab processor 单元测试（纯函数 + 端到端）。
//
// 覆盖：
//   - findBestMatch（编辑距离）
//   - thresholdFor（category 阈值回退）
//   - categoryOf（Meta 提取）
//   - runeLen / byteOffsetOfRune（UTF-8 安全）
//   - hasChangeAtSpan（重叠区间检测）
//   - Process（端到端：alias 已命中的位置 fuzzy 不再提议）
package processor

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

// =====================================================================
// findBestMatch：编辑距离 + confidence
// =====================================================================

func TestFindBestMatch(t *testing.T) {
	tests := []struct {
		name       string
		sub        string
		bucket     []lexicon.Entry
		maxDist    int
		wantText   string
		wantDist   int
		wantConfGT float64 // confidence >=
	}{
		{
			name:     "exact match returns zero (no replacement)",
			sub:      "佘丽群",
			bucket:   []lexicon.Entry{{ID: "1", Text: "佘丽群"}},
			maxDist:  1,
			wantText: "",
			wantDist: 0,
		},
		{
			name:       "single edit returns dist=1",
			sub:        "周丽群",
			bucket:     []lexicon.Entry{{ID: "1", Text: "佘丽群"}},
			maxDist:    2,
			wantText:   "佘丽群",
			wantDist:   1,
			wantConfGT: 0.5,
		},
		{
			name:       "best of multiple candidates",
			sub:        "周莉群",
			bucket:     []lexicon.Entry{{ID: "1", Text: "佘丽群"}, {ID: "2", Text: "佘莉群"}},
			maxDist:    2,
			wantText:   "佘莉群",
			wantDist:   1,
			wantConfGT: 0.5,
		},
		{
			name:     "distance too high returns empty",
			sub:      "abcdef",
			bucket:   []lexicon.Entry{{ID: "1", Text: "佘丽群"}},
			maxDist:  1,
			wantText: "",
			wantDist: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEntry, gotConf, gotDist := findBestMatch(tt.sub, tt.bucket, tt.maxDist)
			if gotEntry.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", gotEntry.Text, tt.wantText)
			}
			if gotDist != tt.wantDist {
				t.Errorf("dist = %d, want %d", gotDist, tt.wantDist)
			}
			if tt.wantConfGT > 0 && gotConf < tt.wantConfGT {
				t.Errorf("conf = %f, want >= %f", gotConf, tt.wantConfGT)
			}
		})
	}
}

// =====================================================================
// thresholdFor：category 阈值回退
// =====================================================================

func TestThresholdFor(t *testing.T) {
	tests := []struct {
		name     string
		catMap   map[string]float64
		fallback float64
		category string
		want     float64
	}{
		{
			name:     "empty category returns fallback",
			catMap:   map[string]float64{"PERSON": 0.65},
			fallback: 0.80,
			category: "",
			want:     0.80,
		},
		{
			name:     "matched category returns map value",
			catMap:   map[string]float64{"PERSON": 0.65},
			fallback: 0.80,
			category: "PERSON",
			want:     0.65,
		},
		{
			name:     "unmatched category returns fallback",
			catMap:   map[string]float64{"PERSON": 0.65},
			fallback: 0.80,
			category: "PRODUCT",
			want:     0.80,
		},
		{
			name:     "nil map returns fallback",
			catMap:   nil,
			fallback: 0.50,
			category: "PERSON",
			want:     0.50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := thresholdFor(tt.catMap, tt.fallback, tt.category)
			if got != tt.want {
				t.Errorf("thresholdFor() = %f, want %f", got, tt.want)
			}
		})
	}
}

// =====================================================================
// categoryOf：Meta 提取
// =====================================================================

func TestCategoryOf(t *testing.T) {
	tests := []struct {
		name string
		e    lexicon.Entry
		want string
	}{
		{"nil meta", lexicon.Entry{}, ""},
		{
			"with category",
			lexicon.Entry{Meta: map[string]any{"category": "PERSON"}},
			"PERSON",
		},
		{
			"non-string category",
			lexicon.Entry{Meta: map[string]any{"category": 42}},
			"",
		},
		{
			"missing key",
			lexicon.Entry{Meta: map[string]any{"other": "x"}},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categoryOf(tt.e)
			if got != tt.want {
				t.Errorf("categoryOf() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =====================================================================
// runeLen / byteOffsetOfRune：UTF-8 安全
// =====================================================================

func TestRuneLen(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"ascii", 5},
		{"佘丽群", 3},    // 中文：3 rune
		{"周丽群ab", 5}, // 混合：3 中文 + 2 ascii
		{"😀😀", 2},      // emoji：2 rune（4 bytes × 2）
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := runeLen(tt.in)
			if got != tt.want {
				t.Errorf("runeLen(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestByteOffsetOfRune(t *testing.T) {
	// "佘丽群" = "佘"(3) + "丽"(3) + "群"(3) = 9 bytes
	tests := []struct {
		runeIdx int
		want    int
	}{
		{0, 0},
		{1, 3},
		{2, 6},
		{3, 9}, // 越界 → 返回完整 byte 长度
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := byteOffsetOfRune("佘丽群", tt.runeIdx)
			if got != tt.want {
				t.Errorf("byteOffsetOfRune(idx=%d) = %d, want %d", tt.runeIdx, got, tt.want)
			}
		})
	}
}

// =====================================================================
// hasChangeAtSpan：重叠区间检测
// =====================================================================

func TestHasChangeAtSpan(t *testing.T) {
	tests := []struct {
		name    string
		changes []lexnorm.Change
		span    lexnorm.Span
		want    bool
	}{
		{
			"empty changes returns false",
			nil,
			lexnorm.Span{0, 3},
			false,
		},
		{
			"exact match returns true",
			[]lexnorm.Change{{Span: lexnorm.Span{0, 3}}},
			lexnorm.Span{0, 3},
			true,
		},
		{
			"overlap returns true",
			[]lexnorm.Change{{Span: lexnorm.Span{0, 5}}},
			lexnorm.Span{3, 8},
			true,
		},
		{
			"non-overlap returns false",
			[]lexnorm.Change{{Span: lexnorm.Span{0, 3}}},
			lexnorm.Span{5, 8},
			false,
		},
		{
			"adjacent (touching) returns false",
			[]lexnorm.Change{{Span: lexnorm.Span{0, 3}}},
			lexnorm.Span{3, 5},
			false, // 区间 [3,5) 与 [0,3) 不重叠（半开区间）
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasChangeAtSpan(tt.changes, tt.span)
			if got != tt.want {
				t.Errorf("hasChangeAtSpan() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =====================================================================
// Process：端到端覆盖 P6 修复（alias 已命中位置 fuzzy 不再提议）
// =====================================================================

func TestProcess_SkipsAlreadyAppliedSpan(t *testing.T) {
	// 词库："佘丽群"（PERSON 类，距离阈值 0.65）
	lex, err := lexicon.NewBuilder().
		Add(lexicon.Entry{
			ID:   "1",
			Text: "佘丽群",
			Meta: map[string]any{"category": "PERSON", "priority": 50},
		}).
		Build()
	if err != nil {
		t.Fatalf("Build lexicon: %v", err)
	}

	cfg := DefaultFuzzyVocabConfig()
	cfg.AutoThreshold = 0.5
	cfg.MaxEditDistance = 1
	cfg.CategoryAuto = map[string]float64{"PERSON": 0.65}

	p := NewFuzzyVocabProcessor(lex, cfg)

	// 手动构造 lexnorm.State，注入一个已存在的 Change（模拟 alias 已命中）
	s := newTestState("周丽群")
	s.Replace(lexnorm.Span{0, 9}, "佘丽群", lexnorm.ChangeMeta{
		Source: "alias", Confidence: 1.0, RuleID: "alias", Reason: "pre",
	})

	if err := p.Process(context.Background(), s); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// 期望：fuzzy 不再提议"周丽群"→"佘丽群"（因为 alias 已经占了 [0,9)）
	for _, c := range s.Changes() {
		if c.Source == "fuzzy_vocab" && c.Span.Start == 0 {
			t.Errorf("fuzzy_vocab still applied at [0,9) but alias already there: %+v", c)
		}
	}
}

func TestProcess_AppliesFuzzy(t *testing.T) {
	// 词库："佘丽群"（无 alias 命中）
	lex, err := lexicon.NewBuilder().
		Add(lexicon.Entry{
			ID:   "1",
			Text: "佘丽群",
			Meta: map[string]any{"category": "PERSON", "priority": 50},
		}).
		Build()
	if err != nil {
		t.Fatalf("Build lexicon: %v", err)
	}

	cfg := DefaultFuzzyVocabConfig()
	cfg.AutoThreshold = 0.5
	cfg.MaxEditDistance = 1
	cfg.CategoryAuto = map[string]float64{"PERSON": 0.65}

	p := NewFuzzyVocabProcessor(lex, cfg)

	s := newTestState("周丽群")
	if err := p.Process(context.Background(), s); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// 期望：fuzzy 提议"周丽群"→"佘丽群"
	found := false
	for _, c := range s.Changes() {
		if c.From == "周丽群" && c.To == "佘丽群" && c.Action == lexnorm.ActionReplace {
			found = true
		}
	}
	if !found {
		t.Errorf("expected fuzzy replace 周丽群→佘丽群, got changes: %+v", s.Changes())
	}
}

// newTestState 构造一个最小可用的 lexnorm.State（仅供测试）。
func newTestState(text string) *lexnorm.State {
	cfg := lexnorm.DefaultConfig()
	lex, err := lexicon.NewBuilder().Build()
	if err != nil {
		panic(err)
	}
	s, err := lexnorm.NewState(context.Background(), text, lex, cfg)
	if err != nil {
		panic(err)
	}
	return s
}
