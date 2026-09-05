// Package fuzzy_matching 实现模糊匹配策略（第 8 步，推断）。
//
// 算法与 evie/service/internal/biz/enhancement_inference.go 的 FuzzyMatchingStep 一致。
package fuzzy_matching

import (
	"context"
	"sort"

	"backend-service/pkg/textenhance/processors"
)

// Processor 模糊匹配策略。
type Processor struct {
	autoThreshold    float64
	suggestThreshold float64
	maxEditDistance  int

	// 按 category 设置阈值（覆盖默认 auto/suggest）。
	// 生产场景：
	//   - PERSON：人名 ASR 错字率高（3 字词 conf=0.667 是常态），需低阈值
	//   - ORGANIZATION / PRODUCT：固定术语，错字代价高，保持高阈值
	categoryAutoThreshold    map[string]float64
	categorySuggestThreshold map[string]float64

	// 防御：词条长度范围（不在此范围的词条不参与模糊匹配）
	minEntryLen int
	maxEntryLen int
}

// Option 配置 Processor 的函数。
type Option func(*Processor)

// WithAutoThreshold 设置自动替换阈值（默认 0.80）。
func WithAutoThreshold(v float64) Option {
	return func(p *Processor) {
		if v > 0 && v <= 1 { p.autoThreshold = v }
	}
}

// WithSuggestThreshold 设置建议阈值（默认 0.60）。
func WithSuggestThreshold(v float64) Option {
	return func(p *Processor) {
		if v > 0 && v <= 1 { p.suggestThreshold = v }
	}
}

// WithMaxEditDistance 设置最大编辑距离（默认 2）。
func WithMaxEditDistance(n int) Option {
	return func(p *Processor) {
		if n > 0 { p.maxEditDistance = n }
	}
}

// WithEntryLenRange 设置参与匹配的词条长度范围（默认 [3, 8]）。
func WithEntryLenRange(min, max int) Option {
	return func(p *Processor) {
		if min > 0 { p.minEntryLen = min }
		if max > 0 { p.maxEntryLen = max }
	}
}

// WithCategoryAutoThreshold 按 category 设置自动替换阈值（覆盖默认）。
//
// 示例：WithCategoryAutoThreshold("PERSON", 0.65) 让 3 字人名错字也能自动替换。
func WithCategoryAutoThreshold(category string, threshold float64) Option {
	return func(p *Processor) {
		if category == "" || threshold <= 0 || threshold > 1 {
			return
		}
		if p.categoryAutoThreshold == nil {
			p.categoryAutoThreshold = make(map[string]float64)
		}
		p.categoryAutoThreshold[category] = threshold
	}
}

// WithCategorySuggestThreshold 按 category 设置建议阈值。
func WithCategorySuggestThreshold(category string, threshold float64) Option {
	return func(p *Processor) {
		if category == "" || threshold <= 0 || threshold > 1 {
			return
		}
		if p.categorySuggestThreshold == nil {
			p.categorySuggestThreshold = make(map[string]float64)
		}
		p.categorySuggestThreshold[category] = threshold
	}
}

// NewFuzzyMatchingProcessor 构造模糊匹配策略。
func NewFuzzyMatchingProcessor(opts ...Option) *Processor {
	p := &Processor{
		autoThreshold:    0.80,
		suggestThreshold: 0.60,
		maxEditDistance:  2,
		minEntryLen:      3,
		maxEntryLen:      8,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// thresholdsFor 返回 entry category 对应的阈值（缺省回退到全局阈值）。
func (p *Processor) thresholdsFor(category string) (auto, suggest float64) {
	auto = p.autoThreshold
	suggest = p.suggestThreshold
	if v, ok := p.categoryAutoThreshold[category]; ok {
		auto = v
	}
	if v, ok := p.categorySuggestThreshold[category]; ok {
		suggest = v
	}
	return
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "fuzzy_matching" }

// Process 实现 processors.TextProcessor。
//
// 算法（与 evie/service FuzzyMatchingStep 一致）：
//   1. 遍历 ec.Vocab.Entries
//   2. 滑窗在 text 中找长度接近的子串
//   3. 计算编辑距离 → 置信度 = 1 - dist/len
//   4. 按阈值决定 REPLACE / SUGGEST / KEEP
func (p *Processor) Process(ctx context.Context, ec *processors.EnhancementContext) {
	if ec == nil || ec.Vocab == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	text := ec.Text
	runes := []rune(text)

	// 确定性顺序：按 standard_text 字典序排序。
	// 背景：qua API 返回顺序不保证稳定 → sync 后 entry ID 在不同进程中分配不同
	// → 同样按 ID 排序仍可能产生不同选词。
	// 只按 standard_text 字典序保证跨进程、跨 sync 一致。
	sortedEntries := make([]*processors.VocabularyEntry, 0, len(ec.Vocab.Entries))
	for _, e := range ec.Vocab.Entries {
		if e != nil {
			sortedEntries = append(sortedEntries, e)
		}
	}
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].StandardText < sortedEntries[j].StandardText
	})

	for _, entry := range sortedEntries {
		stdText := entry.StandardText
		n := len([]rune(stdText))
		if n < p.minEntryLen || n > p.maxEntryLen {
			continue
		}
		if ec.IsLocked(stdText) {
			continue
		}
		category := entry.Category
		autoTh, suggestTh := p.thresholdsFor(category)
		for i := 0; i+n <= len(runes); i++ {
			sub := string(runes[i : i+n])
			if sub == stdText {
				continue
			}
			if ec.IsLocked(sub) {
				continue
			}
			dist := levenshteinDistance(sub, stdText)
			if dist == 0 || dist > p.maxEditDistance {
				continue
			}
			conf := 1.0 - float64(dist)/float64(n)
			action := actionForScore(conf, suggestTh, autoTh)
			if action == processors.ActionKeep {
				continue
			}
			if action == processors.ActionReplace {
				ec.Text = replaceAll(ec.Text, sub, stdText)
				ec.Lock(stdText)
				// 同时锁 sub，防止同一 sub 被多个 stdText 重复处理
				ec.Lock(sub)
			}
			ec.Changes = append(ec.Changes, processors.Change{
				From:       sub,
				To:         stdText,
				Action:     action,
				Type:       processors.TypeFuzzy,
				Source:     processors.SourceTenantDict,
				Confidence: conf,
				Locked:     false,
			})
		}
	}
}

// actionForScore 根据置信度决定动作。
func actionForScore(conf, suggestThreshold, autoThreshold float64) string {
	if conf >= autoThreshold {
		return processors.ActionReplace
	}
	if conf >= suggestThreshold {
		return processors.ActionSuggest
	}
	return processors.ActionKeep
}

// replaceAll 转发到 processors.ReplaceAll 公共函数。
func replaceAll(s, old, new string) string {
	return processors.ReplaceAll(s, old, new)
}

// levenshteinDistance 计算两字符串编辑距离（基于 rune）。
//
// 从 evie/service enhancement_inference.go 复制（一字不改）。
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

func indexOf(s, sub string, start int) int {
	if start < 0 {
		start = 0
	}
	for i := start; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// 编译期断言
var _ processors.TextProcessor = (*Processor)(nil)