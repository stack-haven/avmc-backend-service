package biz

import (
	"strings"
)

// levenshteinDistance 计算两个字符串的编辑距离（Levenshtein，基于 rune）。
// 内联自原 correction_editdistance.go（该文件已废弃并删除）。
func levenshteinDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	la, lb := len(ar), len(br)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// 推断置信度阈值（开发说明第三十五节：第一期系统内置，不开放业务配置）。
const (
	autoThreshold    = 0.8 // >= 0.8 自动替换 REPLACE
	suggestThreshold = 0.6 // >= 0.6 建议 SUGGEST；< 0.6 保留 KEEP
)

// PinyinCorrectionStep 第六层：拼音纠错（HOMOPHONE 同音关系 → 标准词）。
// 同音误识别置信度较高（0.85），>= autoThreshold 自动替换。
type PinyinCorrectionStep struct{}

func (PinyinCorrectionStep) Name() string { return "pinyin_correction" }

func (PinyinCorrectionStep) Process(c *EnhancementContext) error {
	if c.Vocab == nil {
		return nil
	}
	for text, rels := range c.Vocab.relations {
		for _, rel := range rels {
			if rel.RelationType != "HOMOPHONE" {
				continue
			}
			target := c.resolveTarget(rel)
			if target == "" || c.isLocked(text) || !strings.Contains(c.Text, text) {
				continue
			}
			c.Text = strings.ReplaceAll(c.Text, text, target)
			c.lock(target)
			c.Changes = append(c.Changes, &EnhancementChange{
				OriginalText: text, ResultText: target, Action: ActionReplace,
				Type: "PINYIN", Source: "TENANT_DICTIONARY", Confidence: 0.85, Locked: false,
			})
		}
	}
	return nil
}

// FuzzyMatchingStep 第七层：模糊匹配（编辑距离，复用词典词）。
// 编辑距离置信度 = 1 - dist/len，按阈值决定 REPLACE/SUGGEST/KEEP。
type FuzzyMatchingStep struct{}

func (FuzzyMatchingStep) Name() string { return "fuzzy_matching" }

func (FuzzyMatchingStep) Process(c *EnhancementContext) error {
	if c.Vocab == nil {
		return nil
	}
	runes := []rune(c.Text)
	for text := range c.Vocab.entries {
		n := len([]rune(text))
		if n < 3 || n > 8 || c.isLocked(text) {
			continue
		}
		for i := 0; i+n <= len(runes); i++ {
			sub := string(runes[i : i+n])
			if sub == text {
				continue
			}
			// 跳过已被确定性层锁定的片段，避免对替换结果再产生模糊建议（如 金种籽→黑种籽）
			if c.isLocked(sub) {
				continue
			}
			dist := levenshteinDistance(sub, text)
			if dist == 0 || dist > 2 {
				continue
			}
			conf := 1.0 - float64(dist)/float64(n)
			action := actionForScore(conf)
			if action == ActionKeep {
				continue
			}
			// 仅 REPLACE 替换文本；SUGGEST 保留原文只记录建议
			if action == ActionReplace {
				c.Text = strings.ReplaceAll(c.Text, sub, text)
				c.lock(text)
			}
			c.Changes = append(c.Changes, &EnhancementChange{
				OriginalText: sub, ResultText: text, Action: action,
				Type: "FUZZY", Source: "TENANT_DICTIONARY", Confidence: conf, Locked: false,
			})
		}
	}
	return nil
}

// PhraseStandardizationStep 短语标准化（数量 + 单位，开发说明第四十节）。
// 例：200个种籽 → 200颗种籽。
type PhraseStandardizationStep struct{}

func (PhraseStandardizationStep) Name() string { return "phrase_standardization" }

var measureRules = []struct{ from, to string }{
	{"个种籽", "颗种籽"},
	{"个种子", "颗种子"},
}

func (PhraseStandardizationStep) Process(c *EnhancementContext) error {
	for _, rule := range measureRules {
		if strings.Contains(c.Text, rule.from) {
			c.Text = strings.ReplaceAll(c.Text, rule.from, rule.to)
			c.lock(rule.to)
			c.Changes = append(c.Changes, &EnhancementChange{
				OriginalText: rule.from, ResultText: rule.to, Action: ActionReplace,
				Type: "PHRASE", Source: "SYSTEM", Confidence: 1.0, Locked: true,
			})
		}
	}
	return nil
}

// ContextCorrectionStep 第八层：上下文纠错（骨架）。
// 结合上下文窗口判断推断候选（如「功课」→「攻克」），M7 提供骨架，完整实现后续接入。
type ContextCorrectionStep struct{}

func (ContextCorrectionStep) Name() string { return "context_correction" }

// contextRules 上下文纠错规则（当前句含上下文词时触发替换）。
// 例：「功课」在含「技术难点」的句子中 → 「攻克」。
var contextRules = []struct {
	from, context, to string
}{
	{"功课", "技术难点", "攻克"},
}

func (ContextCorrectionStep) Process(c *EnhancementContext) error {
	for _, rule := range contextRules {
		if strings.Contains(c.Text, rule.from) && strings.Contains(c.Text, rule.context) {
			c.Text = strings.ReplaceAll(c.Text, rule.from, rule.to)
			c.lock(rule.to)
			c.Changes = append(c.Changes, &EnhancementChange{
				OriginalText: rule.from, ResultText: rule.to, Action: ActionReplace,
				Type: "CONTEXT", Source: "SYSTEM", Confidence: 0.9, Locked: false,
			})
		}
	}
	return nil
}

// actionForScore 根据置信度决定推断动作（REPLACE/SUGGEST/KEEP）。
func actionForScore(score float64) string {
	switch {
	case score >= autoThreshold:
		return ActionReplace
	case score >= suggestThreshold:
		return ActionSuggest
	default:
		return ActionKeep
	}
}
