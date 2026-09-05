// Package processor · fuzzy_vocab.go
// 自定义 fuzzy processor：基于词库的编辑距离模糊匹配。
//
// 背景：lexnorm 内置 fuzzy (processor/fuzzy) 仅匹配 Lexicon 中
// VariantApproximate 的预登记变体（Aho-Corasick 命中）。
//
// evie/tool 业务需求是：词库里任何一个标准词，与文本中任意同长度子串
// 计算 Levenshtein 距离，超过阈值即自动 REPLACE（典型场景：
// ASR 把"佘丽群"识别成"周丽群"，距离=1 → 替换为标准词）。
//
// 这是「词库驱动的模糊匹配」，与 lexnorm 的「变体预登记模式」不同，
// 故独立实现，使用 lexnorm 提供的 LevenshteinDistance + 区间锁能力。
package processor

import (
	"context"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"

	v1 "backend-service/api/evie/tool/v1"
)

// FuzzyVocabConfig 业务可配置的 fuzzy 阈值。
type FuzzyVocabConfig struct {
	AutoThreshold    float64            // 全局自动替换阈值（默认 0.80）
	SuggestThreshold float64            // 全局建议阈值（默认 0.60）
	CategoryAuto     map[string]float64 // 按 category 覆盖（PERSON=0.65）
	CategorySuggest  map[string]float64
	MinEntryLen      int // 参与匹配的最短词条（默认 2）
	MaxEntryLen      int // 参与匹配的最长词条（默认 8）
	MaxEditDistance  int // 最大编辑距离（默认 2）
}

// DefaultFuzzyVocabConfig 默认阈值（与原 fuzzy_matching 一致）。
func DefaultFuzzyVocabConfig() FuzzyVocabConfig {
	return FuzzyVocabConfig{
		AutoThreshold:    0.80,
		SuggestThreshold: 0.60,
		CategoryAuto: map[string]float64{
			"PERSON": 0.65, // 人名 ASR 错字率高
		},
		CategorySuggest: map[string]float64{
			"PERSON": 0.55,
		},
		MinEntryLen:     2,
		MaxEntryLen:     8,
		MaxEditDistance: 2,
	}
}

// FuzzyVocabProcessor 实现 lexnorm.Processor：基于词库的编辑距离模糊匹配。
type FuzzyVocabProcessor struct {
	lex    lexicon.Lexicon
	config FuzzyVocabConfig

	// 预处理：按 entry.Text 长度分桶，O(1) 查 n 长度桶
	byLen map[int][]lexicon.Entry
}

// NewFuzzyVocabProcessor 构造 processor。
func NewFuzzyVocabProcessor(lex lexicon.Lexicon, cfg FuzzyVocabConfig) *FuzzyVocabProcessor {
	p := &FuzzyVocabProcessor{lex: lex, config: cfg}
	if lex != nil {
		p.buildIndex()
	}
	return p
}

// Name 实现 lexnorm.Processor。
func (p *FuzzyVocabProcessor) Name() string { return "fuzzy_vocab" }

// Version 实现 lexnorm.Versioner。
func (p *FuzzyVocabProcessor) Version() string { return "v1" }

// Certainty 实现 lexnorm.CertaintyReporter。
func (p *FuzzyVocabProcessor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyMedium }

func (p *FuzzyVocabProcessor) buildIndex() {
	p.byLen = make(map[int][]lexicon.Entry)
	p.lex.All(func(e lexicon.Entry) bool {
		n := runeLen(e.Text)
		if n < p.config.MinEntryLen || n > p.config.MaxEntryLen {
			return true
		}
		p.byLen[n] = append(p.byLen[n], e)
		return true
	})
	// 每个桶内按 entry.Text 排序（确定性遍历）
	for k := range p.byLen {
		bucket := p.byLen[k]
		sort.Slice(bucket, func(i, j int) bool { return bucket[i].Text < bucket[j].Text })
		p.byLen[k] = bucket
	}
}

// Process 实现 lexnorm.Processor。
func (p *FuzzyVocabProcessor) Process(_ context.Context, s *lexnorm.State) error {
	if p.lex == nil || len(p.byLen) == 0 {
		return nil
	}
	original := s.Original()
	runes := []rune(original)
	if len(runes) == 0 {
		return nil
	}

	autoApply := s.Config().AutoApplyThreshold
	suggest := s.Config().SuggestThreshold
	maxEdit := p.config.MaxEditDistance

	// 遍历文本：每个起点 i 试 minLen..maxLen 长度
	for i := 0; i < len(runes); i++ {
		for n := p.config.MinEntryLen; n <= p.config.MaxEntryLen && i+n <= len(runes); n++ {
			bucket := p.byLen[n]
			if len(bucket) == 0 {
				continue
			}
			sub := string(runes[i : i+n])

			// 查找最佳匹配：编辑距离 ≤ maxEdit 且 confidence ≥ suggest
			bestEntry, bestConf, bestDist := findBestMatch(sub, bucket, maxEdit)
			if bestEntry.ID == "" {
				continue
			}

			// 应用阈值（按 category 覆盖）
			autoTh := thresholdFor(p.config.CategoryAuto, autoApply, categoryOf(bestEntry))
			sugTh := thresholdFor(p.config.CategorySuggest, suggest, categoryOf(bestEntry))

			// 跳过完全相同（避免无意义变更）
			if sub == bestEntry.Text {
				continue
			}

			span := lexnorm.Span{Start: byteOffsetOfRune(original, i), End: byteOffsetOfRune(original, i+n)}
			meta := lexnorm.ChangeMeta{
				Source:     p.Name(),
				Confidence: bestConf,
				RuleID:     "edit_distance",
				EntryID:    string(bestEntry.ID),
				Reason:     fmt.Sprintf("fuzzy: %q → %q (dist=%d, conf=%.2f)", sub, bestEntry.Text, bestDist, bestConf),
			}

			switch {
			case bestConf >= autoTh:
				_ = s.Replace(span, bestEntry.Text, meta)
			case bestConf >= sugTh:
				_ = s.Suggest(span, bestEntry.Text, meta)
			}
		}
	}
	return nil
}

// Descriptor 实现 lexnorm.DescriptorProvider。
func (p *FuzzyVocabProcessor) Descriptor() lexnorm.Descriptor {
	return lexnorm.Descriptor{
		Name:      p.Name(),
		Certainty: lexnorm.CertaintyMedium,
	}
}

// findBestMatch 在候选 bucket 中找编辑距离最小的 entry。
//
// 返回：entry / confidence (=1 - dist/n) / dist
func findBestMatch(sub string, bucket []lexicon.Entry, maxDist int) (lexicon.Entry, float64, int) {
	var best lexicon.Entry
	bestDist := maxDist + 1
	for _, e := range bucket {
		if e.Text == sub {
			return lexicon.Entry{}, 0, 0 // 完全相同，不做替换
		}
		d := lexicon.LevenshteinDistance(sub, e.Text)
		if d < bestDist {
			bestDist = d
			best = e
		}
	}
	if bestDist > maxDist || best.ID == "" {
		return lexicon.Entry{}, 0, bestDist
	}
	conf := 1.0 - float64(bestDist)/float64(runeLen(best.Text))
	return best, conf, bestDist
}

// thresholdFor 查 category 阈值；无 category 或无覆盖时回退到全局。
func thresholdFor(catMap map[string]float64, fallback float64, category string) float64 {
	if category == "" {
		return fallback
	}
	if v, ok := catMap[category]; ok {
		return v
	}
	return fallback
}

// categoryOf 从 Entry.Meta 拿 category 字符串。
func categoryOf(e lexicon.Entry) string {
	if e.Meta == nil {
		return ""
	}
	if v, ok := e.Meta["category"].(string); ok {
		return v
	}
	return ""
}

// runeLen 返回 UTF-8 字符串的 rune 数。
func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// byteOffsetOfRune 返回 runes[i] 在原始 UTF-8 字符串中的字节偏移。
func byteOffsetOfRune(s string, runeIdx int) int {
	i := 0
	for ; runeIdx > 0 && i < len(s); runeIdx-- {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return i
}

// Ensure compile-time interface assertion.
var _ lexnorm.Processor = (*FuzzyVocabProcessor)(nil)
var _ lexnorm.Versioner = (*FuzzyVocabProcessor)(nil)
var _ lexnorm.CertaintyReporter = (*FuzzyVocabProcessor)(nil)

// dummy 引用防 lint 警告（v1.EnhanceChange 用于可能的 audit 字段预留）。
var _ = v1.EnhanceChange{}
