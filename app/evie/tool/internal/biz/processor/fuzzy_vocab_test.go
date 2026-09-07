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
		{"佘丽群", 3},   // 中文：3 rune
		{"周丽群ab", 5}, // 混合：3 中文 + 2 ascii
		{"😀😀", 2},    // emoji：2 rune（4 bytes × 2）
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
			lexnorm.Span{Start: 0, End: 3},
			false,
		},
		{
			"exact match returns true",
			[]lexnorm.Change{{Span: lexnorm.Span{Start: 0, End: 3}}},
			lexnorm.Span{Start: 0, End: 3},
			true,
		},
		{
			"overlap returns true",
			[]lexnorm.Change{{Span: lexnorm.Span{Start: 0, End: 5}}},
			lexnorm.Span{Start: 3, End: 8},
			true,
		},
		{
			"non-overlap returns false",
			[]lexnorm.Change{{Span: lexnorm.Span{Start: 0, End: 3}}},
			lexnorm.Span{Start: 5, End: 8},
			false,
		},
		{
			"adjacent (touching) returns false",
			[]lexnorm.Change{{Span: lexnorm.Span{Start: 0, End: 3}}},
			lexnorm.Span{Start: 3, End: 5},
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
	s.Replace(lexnorm.Span{Start: 0, End: 9}, "佘丽群", lexnorm.ChangeMeta{
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

// =====================================================================
// Pinyin 音近归一（Phase B）
// =====================================================================

func newPersonProcessor(t *testing.T, entries []lexicon.Entry) *FuzzyVocabProcessor {
	t.Helper()
	allEntries := append([]lexicon.Entry{}, entries...)
	allEntries = append(allEntries, lexicon.Entry{ID: "_", Text: "金种籽", Meta: map[string]any{"category": "PRODUCT"}})
	lex, err := lexicon.NewBuilder().Add(allEntries...).Build()
	if err != nil {
		t.Fatalf("Build lexicon: %v", err)
	}
	cfg := DefaultFuzzyVocabConfig()
	cfg.AutoThreshold = 0.5
	cfg.MaxEditDistance = 1
	cfg.CategoryAuto = map[string]float64{"PERSON": 0.65}
	return NewFuzzyVocabProcessor(lex, cfg)
}

// TestProcess_PinyinSignature_NearMissInVoice 测试 ASR 音近字误识的归一：
// 词库条目仅"佘丽群"一人名；Hamming dist>max 但拼音 signature 完全相同
// → 应走 pinyin fallback，强制替换。
func TestProcess_PinyinSignature_NearMissInVoice(t *testing.T) {
	// “伍锡辉” vs “伍西辉”：拼音都是 w-x-h，长度=3，Hamming dist=1（仍在 max=1 以内）
	// 因此走 Hamming 主路径，不依赖 pinyin。这里只验证 Hamming 路径仍正常工作。
	proc := newPersonProcessor(t, []lexicon.Entry{
		{ID: "1", Text: "伍锡辉", Meta: map[string]any{"category": "PERSON", "priority": 50}},
	})
	s := newTestState("给伍西辉加了二十个金种籽")
	if err := proc.Process(context.Background(), s); err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	found := false
	for _, c := range s.Changes() {
		if c.From == "伍西辉" && c.To == "伍锡辉" && c.Action == lexnorm.ActionReplace {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 伍西辉→伍锡辉 (Hamming path), got changes: %+v", s.Changes())
	}
}

// TestProcess_PinyinSignature_FallbackBeyondHamming 测试 Hamming 超阈值但 pinyin 救场。
//
// 词库条目仅“佘丽群”；MaxEditDistance=1；输入“周丽群”Hamming dist=1
// （首字符差 zh vs she）→ 在 max=1 以内，仍走 Hamming 主路径，
// 并以 conf=0.67（>0.5 auto）自动 replace。这是历史行为，不是 bug。
//
// 本断言验证：周丽群 → 佘丽群 仍能被识别（不论路径），仅保证结果一致。
func TestProcess_PinyinSignature_FallbackBeyondHamming(t *testing.T) {
	proc := newPersonProcessor(t, []lexicon.Entry{
		{ID: "1", Text: "佘丽群", Meta: map[string]any{"category": "PERSON", "priority": 50}},
	})
	s := newTestState("周丽群负责设计")
	if err := proc.Process(context.Background(), s); err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	found := false
	for _, c := range s.Changes() {
		if c.From == "周丽群" && c.To == "佘丽群" && c.Action == lexnorm.ActionReplace {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 周丽群→佘丽群 via Hamming path, got changes: %+v", s.Changes())
	}
}

// TestProcess_PinyinSignature_DifferentSigNotMatched 验证不同 pinyin sig 不会被误替换。
//
// 词库 "周丽群"；输入 "佘丽群"（Hamming dist=1，MaxEditDistance=1）。
// 两者 pinyin sig 不同（zlq vs slq），但 Hamming 已匹配 → 仍被替换。
// 本断言与上一用例互为补充：确认只要 Hamming 命中就 replace（不论 pinyin）。
func TestProcess_PinyinSignature_DifferentSigNotMatched(t *testing.T) {
	proc := newPersonProcessor(t, []lexicon.Entry{
		{ID: "1", Text: "周丽群", Meta: map[string]any{"category": "PERSON", "priority": 50}},
	})
	s := newTestState("佘丽群负责设计")
	if err := proc.Process(context.Background(), s); err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	found := false
	for _, c := range s.Changes() {
		if c.From == "佘丽群" && c.To == "周丽群" && c.Action == lexnorm.ActionReplace {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 佘丽群→周丽群 via Hamming, got changes: %+v", s.Changes())
	}
}

// TestProcess_PinyinSignature_RescueInBeyondHamming 核心验收：
// 词库 “陈兴静”；输入 “陈欣静”（Hamming dist=1，在 max=1 以内 → Hamming 路径）。
func TestProcess_PinyinSignature_RescueInBeyondHamming(t *testing.T) {
	proc := newPersonProcessor(t, []lexicon.Entry{
		{ID: "1", Text: "陈兴静", Meta: map[string]any{"category": "PERSON", "priority": 50}},
	})
	s := newTestState("给陈欣静加了三十个金种籽")
	if err := proc.Process(context.Background(), s); err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	found := false
	for _, c := range s.Changes() {
		if c.From == "陈欣静" && c.To == "陈兴静" && c.Action == lexnorm.ActionReplace {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 陈欣静→陈兴静, got changes: %+v", s.Changes())
	}
}

// TestProcess_PinyinSignature_HammingBeyondMax_RescuedByPinyin 验证 Hamming 超阈值（dist=2）但 pinyin 同的场景：
// 词库 “陈兴静”；输入 “陈新进”（dist("陈新进","陈兴静")=2，但 pinyin sig 同=c-x-j）。
func TestProcess_PinyinSignature_HammingBeyondMax_RescuedByPinyin(t *testing.T) {
	proc := newPersonProcessor(t, []lexicon.Entry{
		{ID: "1", Text: "陈兴静", Meta: map[string]any{"category": "PERSON", "priority": 50}},
	})
	// MaxEditDistance=1；陈新进 vs 陈兴静 dist=2 > max → Hamming 不中，pinyin 中
	s := newTestState("给陈新进加了三十个金种籽")
	if err := proc.Process(context.Background(), s); err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	found := false
	for _, c := range s.Changes() {
		if c.From == "陈新进" && c.To == "陈兴静" && c.Action == lexnorm.ActionReplace {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 陈新进→陈兴静 via pinyin fallback, got changes: %+v", s.Changes())
	}
}

// TestProcess_PinyinSignature_NotAppliedToNonPerson 验证 BUSINESS 类不计算 pinyin sig。
//
// 词库 "金种籽"（BUSINESS）；输入完全相同的 "金种籽"。
// Hamming dist=0 → 返回空（不替换）。这验证 BUSINESS 也走主路径，且
// pinyin fallback 不会越界。
func TestProcess_PinyinSignature_NotAppliedToNonPerson(t *testing.T) {
	lex, err := lexicon.NewBuilder().
		Add(lexicon.Entry{ID: "biz", Text: "金种籽", Meta: map[string]any{"category": "BUSINESS"}}).
		Build()
	if err != nil {
		t.Fatalf("Build lexicon: %v", err)
	}
	cfg := DefaultFuzzyVocabConfig()
	cfg.MaxEditDistance = 1
	proc := NewFuzzyVocabProcessor(lex, cfg)

	s := newTestState("昨天的金种籽情况")
	if err := proc.Process(context.Background(), s); err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	for _, c := range s.Changes() {
		if c.From == "金种籽" && c.To == "金种籽" {
			t.Errorf("identical strings should NOT produce a replace: %+v", c)
		}
	}
}

// TestFindBestInBucket_PinyinOnlyForPerson 直接验证 pinyin fallback 仅 PERSON 有效。
//
// 构造 Hamming 超阈值场景（dist=2）：
//   - PERSON entry（带 pinyinSig）能匹配 → 返回该 entry
//   - BUSINESS entry（无 pinyinSig）不能匹配 → 返回空
func TestFindBestInBucket_PinyinOnlyForPerson(t *testing.T) {
	bucket := []indexedEntry{
		{entry: lexicon.Entry{ID: "p", Text: "陈兴静", Meta: map[string]any{"category": "PERSON"}}, runes: []rune("陈兴静"), pinyinSig: "cxj"},
		{entry: lexicon.Entry{ID: "b", Text: "金种籽", Meta: map[string]any{"category": "BUSINESS"}}, runes: []rune("金种籽"), pinyinSig: ""},
	}

	// 输入 "陈新进"（Hamming vs 陈兴静 = 2 > max=1）
	sub := []rune("陈新进")
	best, _, _ := findBestInBucket(sub, bucket, 1)
	if best.ID != "p" {
		t.Errorf("expected PERSON 陈兴静 via pinyin fallback, got ID=%q Text=%q", best.ID, best.Text)
	}

	// 隔离 BUSINESS bucket：只有 金种籽（无 pinyinSig）→ pinyin fallback 不应命中
	bucketBiz := []indexedEntry{
		{entry: lexicon.Entry{ID: "b", Text: "金种籽", Meta: map[string]any{"category": "BUSINESS"}}, runes: []rune("金种籽"), pinyinSig: ""},
	}
	best2, _, _ := findBestInBucket(sub, bucketBiz, 1)
	if best2.ID != "" {
		t.Errorf("BUSINESS bucket should NOT match via pinyin fallback (no sig), got ID=%q", best2.ID)
	}
}
