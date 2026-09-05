// fuzzy_matching_test.go
//
// 验证 production 场景：3 字人名错字（conf=0.667）按 PERSON 阈值能自动替换。
package fuzzy_matching

import (
	"context"
	"strings"
	"testing"

	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/processors"
)

func TestFuzzyMatching_PersonCategoryThreshold(t *testing.T) {
	// 用 0.80 全局阈值 + PERSON 0.65 类别阈值
	p := NewFuzzyMatchingProcessor(
		WithCategoryAutoThreshold("PERSON", 0.65),
		WithCategorySuggestThreshold("PERSON", 0.55),
	)

	snap := processors.NewVocabularySnapshot(
		[]*processors.VocabularyEntry{
			{ID: 1, StandardText: "佘丽群", Category: "PERSON", Priority: 50},
			{ID: 2, StandardText: "测试1", Category: "PERSON", Priority: 50},
			{ID: 3, StandardText: "金种籽", Category: "PRODUCT", Priority: 100},
		},
		nil,
	)

	tests := []struct {
		name       string
		input      string
		wantText   string
		wantChange bool // 期望有 REPLACE 类 change
	}{
		{
			name:       "PERSON 1 字差自动替换",
			input:      "周丽群提交了报告",
			wantText:   "佘丽群提交了报告",
			wantChange: true,
		},
		{
			name:       "PERSON 1 字差-2",
			input:      "测试播在测试",
			wantText:   "测试1在测试",
			wantChange: true,
		},
		{
			name:       "PRODUCT 1 字差不替换（保持默认 0.80 阈值）",
			input:      "金种仔情况",
			wantText:   "金种仔情况", // 不替换（conf=0.667 < 0.80）
			wantChange: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ec := processors.NewEnhancementContext(tc.input, snap, nil)
			p.Process(context.Background(), ec)

			got := ec.GetText()
			if got != tc.wantText {
				t.Errorf("text mismatch:\n  got:  %q\n  want: %q", got, tc.wantText)
			}

			hasReplace := false
			for _, c := range ec.GetChanges() {
				if c.Action == processors.ActionReplace {
					hasReplace = true
					break
				}
			}
			if tc.wantChange && !hasReplace {
				t.Errorf("expected REPLACE action, got changes: %+v", ec.GetChanges())
			}
			if !tc.wantChange && hasReplace {
				t.Errorf("unexpected REPLACE action: %+v", ec.GetChanges())
			}
		})
	}
}

func TestFuzzyMatching_DefaultThresholdUnchanged(t *testing.T) {
	// 不带 WithCategoryAutoThreshold 时，所有类别都用默认 0.80
	p := NewFuzzyMatchingProcessor()

	snap := processors.NewVocabularySnapshot(
		[]*processors.VocabularyEntry{
			{ID: 1, StandardText: "佘丽群", Category: "PERSON", Priority: 50},
		},
		nil,
	)

	ec := processors.NewEnhancementContext("周丽群", snap, nil)
	p.Process(context.Background(), ec)

	// 默认阈值下 3 字 1 字差 → SUGGEST 不替换
	if ec.GetText() != "周丽群" {
		t.Errorf("expected no replace with default threshold, got %q", ec.GetText())
	}

	// 应有 SUGGEST change
	hasSuggest := false
	for _, c := range ec.GetChanges() {
		if c.Action == processors.ActionSuggest {
			hasSuggest = true
			break
		}
	}
	if !hasSuggest {
		t.Error("expected SUGGEST action")
	}
}

func TestFuzzyMatching_PersonPriorityReplacesBeforeProduct(t *testing.T) {
	// 测试：当文本中同时包含 PERSON 和 PRODUCT 错字时，各自按自己的阈值处理
	p := NewFuzzyMatchingProcessor(
		WithCategoryAutoThreshold("PERSON", 0.65),
	)

	snap := processors.NewVocabularySnapshot(
		[]*processors.VocabularyEntry{
			{ID: 1, StandardText: "佘丽群", Category: "PERSON", Priority: 50},
			{ID: 2, StandardText: "金种籽", Category: "PRODUCT", Priority: 100},
		},
		nil,
	)

	ec := processors.NewEnhancementContext("周丽群给金种仔开会", snap, nil)
	p.Process(context.Background(), ec)

	// PERSON 替换
	if !strings.Contains(ec.GetText(), "佘丽群") {
		t.Errorf("expected PERSON replace, got %q", ec.GetText())
	}
	// PRODUCT 不替换（默认 0.80）
	if !strings.Contains(ec.GetText(), "金种仔") {
		t.Errorf("expected PRODUCT not replaced, got %q", ec.GetText())
	}
}

// TestFuzzyMatching_SubLockAfterReplace 回归测试：同一 sub 被多个 stdText 匹配时，
// 第一次 REPLACE 后应锁 sub，避免重复 REPLACE 生成多个冗余 change。
// 生产案例："测试播" 与 qua 字典中"测试1/测试3/测试有" 都距离=1，同时匹配。
func TestFuzzyMatching_SubLockAfterReplace(t *testing.T) {
	p := NewFuzzyMatchingProcessor(
		WithCategoryAutoThreshold("PERSON", 0.55),
	)

	snap := processors.NewVocabularySnapshot(
		[]*processors.VocabularyEntry{
			{ID: 1, StandardText: "测试1", Category: "PERSON", Priority: 50},
			{ID: 2, StandardText: "测试3", Category: "PERSON", Priority: 50},
			{ID: 3, StandardText: "测试有", Category: "PERSON", Priority: 50},
		},
		nil,
	)

	ec := processors.NewEnhancementContext("测试播完成", snap, nil)
	p.Process(context.Background(), ec)

	// 统计 REPLACE change 个数：应该是 1（不是 3）
	replaceCount := 0
	for _, c := range ec.GetChanges() {
		if c.Action == processors.ActionReplace {
			replaceCount++
		}
	}
	if replaceCount > 1 {
		t.Errorf("expected at most 1 REPLACE (sub locked after first), got %d:\n%+v",
			replaceCount, ec.GetChanges())
	}

	// 文本只被替换 1 次
	if ec.GetText() == "测试播完成" {
		t.Error("expected at least one REPLACE applied")
	}
}

// TestFuzzyMatching_DeterministicOrder 验证多次跑同一输入，结果一致。
//
// 历史问题：fuzzy_matching 之前用 map 迭代遍历 entries，每次随机顺序。
// 多候选场景（"测试播" 可被多个 qua 用户匹配）下输出 REPLACE 不一致。
// 修复：按 entry.ID 排序遍历。
func TestFuzzyMatching_DeterministicOrder(t *testing.T) {
	// 构造 4 个候选 "测试*" 词条，距离都是 1（与 "测试播" 差 1 字）
	makeSnap := func() *processors.VocabularySnapshot {
		entries := []*processors.VocabularyEntry{
			{ID: 1, StandardText: "测试A", Category: "PERSON", Priority: 50},
			{ID: 2, StandardText: "测试B", Category: "PERSON", Priority: 50},
			{ID: 3, StandardText: "测试C", Category: "PERSON", Priority: 50},
			{ID: 4, StandardText: "测试D", Category: "PERSON", Priority: 50},
		}
		snap := processors.NewVocabularySnapshot(entries, nil)
		return snap
	}
	run := func() string {
		ec := textenhance.NewEnhancementContext("测试播", makeSnap(), nil)
		p := NewFuzzyMatchingProcessor(WithCategoryAutoThreshold("PERSON", 0.5))
		p.Process(context.Background(), ec)
		return ec.GetText()
	}
	// 跑 10 次，结果应完全一致
	first := run()
	for i := 0; i < 10; i++ {
		got := run()
		if got != first {
			t.Errorf("iter %d: got %q, want %q (non-deterministic)", i, got, first)
		}
	}
}
