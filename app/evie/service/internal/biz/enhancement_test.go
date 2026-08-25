package biz

import (
	"testing"
)

// buildTestVocab 构造测试用 VocabularyContext（词条 + 关系）。
func buildTestVocab() *VocabularyContext {
	return &VocabularyContext{
		entries: map[string]*VocabularyEntry{
			"田华":   {ID: 1, StandardText: "田华", Category: "PERSON"},
			"金种籽":  {ID: 2, StandardText: "金种籽", Category: "PRODUCT"},
			"技术部":  {ID: 3, StandardText: "技术部", Category: "ORGANIZATION"},
		},
		relations: map[string][]*VocabularyRelation{
			"小田":  {{EntryID: 1, RelationType: "ALIAS", RelatedText: "小田", TargetEntryID: 1}},
			"金种子": {{EntryID: 2, RelationType: "CORRECTION", RelatedText: "金种子", TargetEntryID: 2}},
		},
	}
}

// applySteps 手动执行完整流水线步骤（确定性 + 推断）。
func applySteps(t *testing.T, c *EnhancementContext) {
	t.Helper()
	for _, step := range []EnhancementStep{
		&TextCleaningStep{}, &FillerStep{}, &VocabularyMatchingStep{},
		&AliasResolutionStep{}, &DeterministicReplacementStep{},
		&PhraseStandardizationStep{}, &PinyinCorrectionStep{},
		&FuzzyMatchingStep{}, &ContextCorrectionStep{},
	} {
		if err := step.Process(c); err != nil {
			t.Fatalf("step %s: %v", step.Name(), err)
		}
	}
}

// Case 1：企业别名解析（小田 → 田华）。
func TestEnhancementCase1Alias(t *testing.T) {
	c := &EnhancementContext{RawText: "给小田申请奖励", Text: "给小田申请奖励", Vocab: buildTestVocab()}
	applySteps(t, c)
	if c.Text != "给田华申请奖励" {
		t.Errorf("Case1 别名解析失败: %q", c.Text)
	}
	if len(c.Changes) == 0 || c.Changes[len(c.Changes)-1].Type != "ALIAS" {
		t.Errorf("expected ALIAS change, got %+v", c.Changes)
	}
}

// Case 2：确定性纠错（金种子 → 金种籽）。
func TestEnhancementCase2Correction(t *testing.T) {
	c := &EnhancementContext{RawText: "金种子", Text: "金种子", Vocab: buildTestVocab()}
	applySteps(t, c)
	if c.Text != "金种籽" {
		t.Errorf("Case2 确定性替换失败: %q", c.Text)
	}
	if len(c.Changes) == 0 || c.Changes[len(c.Changes)-1].Type != "CORRECTION" {
		t.Errorf("expected CORRECTION change, got %+v", c.Changes)
	}
}

// Case 3：口水词 + 别名（呃我想给那个技术部小田申请奖励 → 我想给技术部田华申请奖励）。
func TestEnhancementCase3FillerAndAlias(t *testing.T) {
	c := &EnhancementContext{
		RawText: "呃我想给那个技术部小田申请奖励",
		Text:    "呃我想给那个技术部小田申请奖励",
		Vocab:   buildTestVocab(),
	}
	applySteps(t, c)
	if c.Text != "我想给那个技术部田华申请奖励" {
		t.Errorf("Case3 口水词+别名失败: %q", c.Text)
	}
}

// Case 7：确定性结果锁定（locked=true，不被后续算法覆盖）。
func TestEnhancementCase7Locked(t *testing.T) {
	c := &EnhancementContext{RawText: "给小田申请奖励", Text: "给小田申请奖励", Vocab: buildTestVocab()}
	applySteps(t, c)
	for _, ch := range c.Changes {
		if ch.Type == "ALIAS" || ch.Type == "CORRECTION" {
			if !ch.Locked {
				t.Errorf("确定性变更 %+v 应 locked", ch)
			}
		}
	}
}

// 文本清洗：重复标点 + 异常空格。
func TestTextCleaningStep(t *testing.T) {
	c := &EnhancementContext{RawText: "我...我想  申请", Text: "我...我想  申请", Vocab: buildTestVocab()}
	_ = (TextCleaningStep{}).Process(c)
	if c.Text != "我.我想 申请" {
		t.Errorf("清洗失败: %q", c.Text)
	}
}

// 口水词：句首「呃」删除，句中的「嗯」保留（可能表示肯定）。
func TestFillerStep(t *testing.T) {
	c := &EnhancementContext{RawText: "呃，好的", Text: "呃，好的", Vocab: buildTestVocab()}
	_ = (FillerStep{}).Process(c)
	if c.Text != "，好的" {
		t.Errorf("口水词删除失败: %q", c.Text)
	}
}

// Case 4：上下文纠错（功课 + 技术难点 → 攻克）。
func TestEnhancementCase4Context(t *testing.T) {
	c := &EnhancementContext{
		RawText: "今天功课了一个技术难点",
		Text:    "今天功课了一个技术难点",
		Vocab:   buildTestVocab(),
	}
	applySteps(t, c)
	if c.Text != "今天攻克了一个技术难点" {
		t.Errorf("Case4 上下文纠错失败: %q", c.Text)
	}
}

// Case 5：短语标准化（200个种籽 → 200颗种籽）。
func TestEnhancementCase5Phrase(t *testing.T) {
	c := &EnhancementContext{RawText: "申请200个种籽", Text: "申请200个种籽", Vocab: buildTestVocab()}
	applySteps(t, c)
	if c.Text != "申请200颗种籽" {
		t.Errorf("Case5 短语标准化失败: %q", c.Text)
	}
}

// Case 6：低置信度 → SUGGEST（不强制替换，保留原文）。
func TestEnhancementCase6Suggest(t *testing.T) {
	// 4 字词 dist=1，conf=0.75，落在 SUGGEST 区间
	c := &EnhancementContext{
		RawText: "种籽奖厉",
		Text:    "种籽奖厉",
		Vocab: &VocabularyContext{
			entries:   map[string]*VocabularyEntry{"种籽奖励": {ID: 9, StandardText: "种籽奖励"}},
			relations: map[string][]*VocabularyRelation{},
		},
	}
	applySteps(t, c)
	// SUGGEST 不替换文本
	if c.Text != "种籽奖厉" {
		t.Errorf("低置信度应保留原文: %q", c.Text)
	}
	// 有 SUGGEST 变更
	found := false
	for _, ch := range c.Changes {
		if ch.Type == "FUZZY" && ch.Action == ActionSuggest {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SUGGEST change, got %+v", c.Changes)
	}
}
