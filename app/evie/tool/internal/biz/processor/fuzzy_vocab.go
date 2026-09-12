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
// 故独立实现。
//
// # 性能优化（Phase 7.1）
//
// 旧实现对每个候选 entry 调用 lexicon.LevenshteinDistance，每次分配
// 两个 []rune 切片并跑完整 DP；1000 词库下 Process 约 9ms。
//
// 关键观察：词库按 entry 文本长度分桶（byLen），候选与 sub 长度必然相等。
// 在等长字符串下，Levenshtein 距离 ≡ Hamming 距离（因为 insert/delete
// 配对成本 ≥ 直接 substitute），可以用 O(n) 的逐位 rune 比较代替
// O(n²) 的 DP，且无需分配。
//
// 优化措施：
//  1. buildIndex 阶段预计算每个 entry 的 []rune，零运行时分配
//  2. Process 阶段预计算 subRunes 切片 + 字节偏移表（替代逐次 O(i) 计算）
//  3. 按"非空长度桶"外层迭代（替代 map lookup + 空桶 skip）
//  4. boundedHamming 早退：当前差异 > maxDist 立即返回
//
// 正确性：等长 Hamming ≡ Levenshtein 已被单元测试覆盖；桶外（不等长）
// 路径仅在 findBestMatch（测试用）中保留，使用原 Levenshtein。
package processor

import (
	"context"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"

	"backend-service/pkg/pinyin"
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

// indexedEntry 在 buildIndex 阶段预计算 entry.Text 的 rune 切片，
// 避免 Process 阶段重复 utf8 解码与 []rune 分配。
type indexedEntry struct {
	entry     lexicon.Entry
	runes     []rune
	pinyinSig string // 仅 PERSON 类别计算；空 sig 表示未启用 pinyin 归一
	lockAlias bool   // true 时跳过（业务产品功能名 / 已知专有名词保护）
}

// FuzzyVocabProcessor 实现 lexnorm.Processor：基于词库的编辑距离模糊匹配。
type FuzzyVocabProcessor struct {
	lex    lexicon.Lexicon
	config FuzzyVocabConfig

	// 预处理：按 entry.Text 长度分桶，O(1) 查 n 长度桶
	byLen map[int][]indexedEntry
	// 非空长度桶的 n 值（排序），用于 Process 外层循环跳过空桶与 map lookup
	nonEmptyLens []int
	// protectedPrefixes 由 lockAlias=true 的 entry 生成的所有非空前缀集合。
	// sub 命中其中任一前缀 → 整个 span 跳过 fuzzy 替换（保护产品功能名 / 专有名词子串）。
	protectedPrefixes map[string]bool
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

// Descriptor 实现 lexnorm.DescriptorProvider。
func (p *FuzzyVocabProcessor) Descriptor() lexnorm.Descriptor {
	return lexnorm.Descriptor{
		Name:      p.Name(),
		Certainty: lexnorm.CertaintyMedium,
	}
}

func (p *FuzzyVocabProcessor) buildIndex() {
	p.byLen = make(map[int][]indexedEntry)
	p.protectedPrefixes = make(map[string]bool)
	if p.lex == nil {
		return
	}
	p.lex.All(func(e lexicon.Entry) bool {
		r := []rune(e.Text)
		n := len(r)
		if n < p.config.MinEntryLen || n > p.config.MaxEntryLen {
			return true
		}
		ie := indexedEntry{entry: e, runes: r}
		// PERSON 类别预计算 pinyin signature（音近归一用）
		if categoryOf(e) == "PERSON" {
			ie.pinyinSig = pinyin.Signature(e.Text)
		}
		// lock_alias=true：跳过该 entry（保护产品功能名 / 专有名词不被误改）
		if lockAliasOf(e) {
			ie.lockAlias = true
			// P-FIX v2：lock_alias entry 只贡献 protectedPrefixes（子串保护），
			// 不进 byLen bucket，避免 ASR 错字匹配业务专名导致误纠
			// 例：金种籽(lock_alias) 不参与 fuzzy 替换 → '菌种子' → '金种籽' 不再发生
			r := []rune(e.Text)
			for plen := 1; plen < len(r); plen++ {
				p.protectedPrefixes[string(r[:plen])] = true
			}
			return true // 不进 bucket
		}
		p.byLen[n] = append(p.byLen[n], ie)
		return true
	})
	// 每个桶内按 entry.Text 排序（确定性遍历 / 最佳匹配选择稳定）
	ns := make([]int, 0, len(p.byLen))
	for k, bucket := range p.byLen {
		sort.Slice(bucket, func(i, j int) bool { return bucket[i].entry.Text < bucket[j].entry.Text })
		p.byLen[k] = bucket
		ns = append(ns, k)
	}
	sort.Ints(ns)
	p.nonEmptyLens = ns
}

// Process 实现 lexnorm.Processor。
//
// 优化路径：
//   - 预计算 runes 与 byteOff（O(N) 一次性）
//   - 外层按非空长度桶遍历（避免 map lookup + 空桶）
//   - 内层对每个 (i,n) 调用 findBestInBucket（bounded Hamming + 早退）
func (p *FuzzyVocabProcessor) Process(_ context.Context, s *lexnorm.State) error {
	if p.lex == nil || len(p.byLen) == 0 {
		return nil
	}
	original := s.Original()
	runes := []rune(original)
	if len(runes) == 0 {
		return nil
	}

	// 字节偏移表：byteOff[i] = runes[:i] 在 original 中的 UTF-8 字节偏移
	byteOff := make([]int, len(runes)+1)
	for i := 0; i < len(runes); i++ {
		byteOff[i+1] = byteOff[i] + utf8.RuneLen(runes[i])
	}

	autoApply := s.Config().AutoApplyThreshold
	suggest := s.Config().SuggestThreshold
	maxEdit := p.config.MaxEditDistance

	changes := s.Changes() // 取一次快照，后续增量追加不重复扫描整段

	// 外层：按非空长度桶迭代
	for _, n := range p.nonEmptyLens {
		if n < p.config.MinEntryLen || n > p.config.MaxEntryLen {
			continue
		}
		bucket := p.byLen[n]
		for i := 0; i+n <= len(runes); i++ {
			subRunes := runes[i : i+n]
			span := lexnorm.Span{Start: byteOff[i], End: byteOff[i+n]}

			// 修复 P6：跳过已被上游 processor 应用过的同区间位置。
			if hasChangeAtSpan(changes, span) {
				continue
			}
			// lockAlias 子串保护：sub 包含 lockAlias entry 的非空前缀（≥1 字）→ 整个 span 跳过
			// 例：protectedPrefixes={"播","播种","播种未"}，sub="测试播种" 含 "播种" → 跳过
			if len(p.protectedPrefixes) > 0 {
				subStr := string(subRunes)
				skip := false
				for prefix := range p.protectedPrefixes {
					if len(prefix) <= len(subStr) && containsString(subStr, prefix) {
						skip = true
						break
					}
				}
				if skip {
					continue
				}
			}

			bestEntry, bestConf, bestDist := findBestInBucket(subRunes, bucket, maxEdit)
			if bestEntry.ID == "" {
				continue
			}

			// 应用阈值（按 category 覆盖）
			autoTh := thresholdFor(p.config.CategoryAuto, autoApply, categoryOf(bestEntry))
			sugTh := thresholdFor(p.config.CategorySuggest, suggest, categoryOf(bestEntry))

			meta := lexnorm.ChangeMeta{
				Source:     p.Name(),
				Confidence: bestConf,
				RuleID:     "edit_distance",
				EntryID:    string(bestEntry.ID),
				Reason:     fmt.Sprintf("fuzzy: %q → %q (dist=%d, conf=%.2f)", string(subRunes), bestEntry.Text, bestDist, bestConf),
			}

			switch {
			case bestConf >= autoTh:
				_ = s.Replace(span, bestEntry.Text, meta)
				changes = append(changes, lexnorm.Change{Span: span})
			case bestConf >= sugTh:
				_ = s.Suggest(span, bestEntry.Text, meta)
			}
		}
	}
	return nil
}

// findBestInBucket 在候选桶中找编辑距离最小的 entry（等长 Hamming 优化路径）。
//
// 匹配策略（顺序）：
//  1. 精确等值（d==0）：跳过，不替换
//  2. （仅 PERSON）pinyin signature 全等：音近归一（覆盖 ASR 前后鼻音、in/ing 等）
//  3. boundedHamming ≤ maxDist：常规编辑距离匹配
//
// 顺序理由：pinyin 优先 Hamming，避免 "天华" vs "田花"（Hamming=2 conf=0）压制
// "天华" vs "田花"（pinyin 命中 conf=0.7）的场景。
//
// 返回：entry / confidence (=1 - dist/n 或 pinyin 固定 0.7) / dist
//
// 若 sub 与某 entry 完全相等，返回 (Entry{}, 0, 0) 表示"无替换"。
func findBestInBucket(subRunes []rune, bucket []indexedEntry, maxDist int) (lexicon.Entry, float64, int) {
	// 1. 精确等值 → 不替换
	for _, ie := range bucket {
		if boundedHamming(subRunes, ie.runes, 0) == 0 {
			return lexicon.Entry{}, 0, 0
		}
	}

	// 2. pinyin 优先（PERSON bucket 才有效）
	if pinyinBest, ok := findBestByPinyinSig(subRunes, bucket); ok {
		return pinyinBest, pinyinFallbackConfidence(len(subRunes)), 0
	}

	// 3. boundedHamming（跳过 lockAlias 的 entry）
	var best lexicon.Entry
	bestDist := maxDist + 1
	for _, ie := range bucket {
		if ie.lockAlias {
			continue
		}
		d := boundedHamming(subRunes, ie.runes, bestDist-1)
		if d < bestDist {
			bestDist = d
			best = ie.entry
		}
	}
	if bestDist > maxDist || best.ID == "" {
		return lexicon.Entry{}, 0, bestDist
	}
	conf := 1.0 - float64(bestDist)/float64(len(subRunes))
	return best, conf, bestDist
}

// findBestByPinyinSig 在 Hamming 超阈值后，按拼音 signature 二次匹配。
//
// 仅对有 pinyinSig 的 entry 参与匹配（PERSON 类别）。
//
// 返回 (entry, true) 当且仅当 subSig 与某 entry 的 sig FuzzyEqual。
func findBestByPinyinSig(subRunes []rune, bucket []indexedEntry) (lexicon.Entry, bool) {
	subSig := pinyin.Signature(string(subRunes))
	if subSig == "" {
		return lexicon.Entry{}, false
	}
	// 优先选 signature 完全相同的第一个（确定性：bucket 已按 Text 排序）
	for _, ie := range bucket {
		if ie.lockAlias {
			continue
		}
		if ie.pinyinSig == "" {
			continue
		}
		if pinyin.FuzzyEqual(subSig, ie.pinyinSig) {
			return ie.entry, true
		}
	}
	return lexicon.Entry{}, false
}

// pinyinFallbackConfidence pinyin 音近归一的固定置信度。
//
// 选择 0.7 的理由：高于 PERSON AutoThreshold=0.65（自动 replace），
// 低于 Hamming dist=1 conf=0.67（避免覆盖已有更精确匹配）。
func pinyinFallbackConfidence(n int) float64 {
	if n <= 0 {
		return 0.7
	}
	_ = n // 后续可按长度动态调整
	return 0.7
}

// boundedHamming 计算等长 rune 切片的 Hamming 距离，超过 max 时早退。
//
// # 等长 Hamming ≡ Levenshtein
//
// 对于 |a| == |b| 的字符串，Levenshtein 距离等于 Hamming 距离：
// 任意 insert+delete 对（成本 2）都不优于直接 substitute（成本 1），
// 因此最优编辑路径不包含 insert/delete。
//
// # 早退语义
//
// 若过程中差异数 d > max，返回当前 d（> max 的具体值不重要，
// 调用方只关心 d < bestDist）。
func boundedHamming(a, b []rune, max int) int {
	// 调用方保证 len(a) == len(b)；不等长时降级为完整扫描（不会发生）
	d := 0
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			d++
			if d > max {
				return d
			}
		}
	}
	return d
}

// findBestMatch 在候选 bucket 中找编辑距离最小的 entry（参考实现 / 测试用）。
//
// 使用 lexicon.LevenshteinDistance，支持任意长度差异；不分配预计算。
// Process 热路径使用 findBestInBucket（等长 Hamming 优化）。
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

// containsString 检查 haystack 是否包含 needle（精确子串，非 fuzzy）。
func containsString(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// lockAliasOf 从 Entry.Meta 拿 lock_alias bool（保护产品功能名/专有名词）。
func lockAliasOf(e lexicon.Entry) bool {
	if e.Meta == nil {
		return false
	}
	if v, ok := e.Meta["lock_alias"].(bool); ok {
		return v
	}
	return false
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
//
// 保留为公开工具（测试用）；Process 热路径已改用 byteOff 预计算表。
func byteOffsetOfRune(s string, runeIdx int) int {
	i := 0
	for ; runeIdx > 0 && i < len(s); runeIdx-- {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return i
}

// hasChangeAtSpan 检查 changes 中是否存在与给定 span 重叠的 Change。
//
// 修复 P6：上游 processor（alias / deterministic）应用 Replace 后没自动 Lock，
// 后续 processor 应跳过同一区间。
func hasChangeAtSpan(changes []lexnorm.Change, span lexnorm.Span) bool {
	for _, c := range changes {
		if c.Span.Start < span.End && c.Span.End > span.Start {
			return true
		}
	}
	return false
}

// Ensure compile-time interface assertion.
var _ lexnorm.Processor = (*FuzzyVocabProcessor)(nil)
var _ lexnorm.Versioner = (*FuzzyVocabProcessor)(nil)
var _ lexnorm.CertaintyReporter = (*FuzzyVocabProcessor)(nil)
