// Package biz · lexnorm_bridge.go
// 把现有 VocabularySnapshot（qua + system.json）适配成 lexnorm 的 Lexicon。
//
// 设计动机：
//   - 业务层已有 VocabularyBuilder（管理 system entries + tenant entries + relations）
//   - lexnorm 的 Lexicon 是 zero-dep 标准接口（Entry + Relation + Variant）
//   - 不能让 lexnorm 反向依赖 business，因此用 adapter 模式单向转换
//
// 转换规则：
//   - VocabularyEntry{StandardText} → lexnorm.Entry.Text
//   - VocabularyEntry{ID} → lexnorm.Entry.ID (string of uint32)
//   - VocabularyEntry{Category, Priority, Pinyin} → Entry.Meta（保留供自定义 processor 使用）
//   - VocabularyRelation{ALIAS} → VariantAlias
//   - VocabularyRelation{CORRECTION} → VariantCorrection
//   - VocabularyRelation{HOMOPHONE} → VariantHomophone
//   - system.json phrase_rules → 单独的 Entry + VariantCorrection
package biz

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/stack-haven/lexnorm/lexicon"
)

// BuildLexiconFromSnapshotExported 导出 buildLexiconFromSnapshot 给 testdata/debug 程序用。
//
// 注意：仅供 debug / 一次性测试程序；正式业务代码应通过 TenantProfileResolver。
func BuildLexiconFromSnapshotExported(snap *VocabularySnapshot, systemDict *systemDictFile, version string) (lexicon.Lexicon, error) {
	return buildLexiconFromSnapshot(snap, systemDict, version)
}

// buildLexiconFromSnapshot 把 per-tenant VocabularySnapshot 转 lexnorm.Lexicon。
//
// 重要：Lexicon 是不可变的，每次 Build 都重新构造；调用方缓存到 per-tenant 内存里。
// 包含 system phrase rules 作为额外 Entry（Category="PHRASE"）。
func buildLexiconFromSnapshot(snap *VocabularySnapshot, systemDict *systemDictFile, version string) (lexicon.Lexicon, error) {
	if snap == nil {
		return nil, fmt.Errorf("lexnorm_bridge: snapshot is nil")
	}


	entries := make([]lexicon.Entry, 0, len(snap.Entries))
	idSet := make(map[string]bool, len(snap.Entries))

	// 1. tenant + system 合并 entries（tenant 优先；与 VocabularyBuilder.mergeWithSystem 一致）
	for _, e := range snap.Entries {
		if e == nil || e.StandardText == "" {
			continue
		}
		idStr := strconv.FormatUint(uint64(e.ID), 10)
		if idSet[idStr] {
			continue
		}
		idSet[idStr] = true

		entry := lexicon.Entry{
			ID:   lexicon.EntryID(idStr),
			Text: e.StandardText,
			Meta: map[string]any{
				"category": e.Category,
				"priority": e.Priority,
				"pinyin":   e.Pinyin,
			},
		}

		// 收集该 entry 的所有 variants（按 related_text 反查 relations）
		var variants []lexicon.Variant
		for _, rs := range snap.Relations {
			for _, r := range rs {
				if r == nil {
					continue
				}
				if r.EntryID != e.ID {
					continue
				}
				if r.RelatedText == "" || r.RelatedText == e.StandardText {
					continue
				}
				kind, ok := variantKindFromRelationType(r.RelationType)
				if !ok {
					continue // unknown kind → skip
				}
				variants = append(variants, lexicon.Variant{
					Text:        r.RelatedText,
					Kind:        kind,
					Confidence:  variantConfidenceFromKind(kind),
					Source:      relationSource(r.RelationType),
				})
			}
		}

		// 系统 phrase_rules 也作为该 entry 的 VariantCorrection
		if systemDict != nil {
			for _, pr := range systemDict.PhraseRules {
				if pr.To == e.StandardText && pr.From != "" {
					variants = append(variants, lexicon.Variant{
						Text:        pr.From,
						Kind:        lexicon.VariantCorrection,
						Confidence:  1.0,
						Source:      "system_phrase",
					})
				}
			}
		}

		// 排序 variants（确保确定性；Aho-Corasick 行为依赖输入顺序）
		sortVariants(variants)
		entry.Variants = variants

		entries = append(entries, entry)
	}

	// 2. system phrase_rules 里 To 不在 entries 中 → 加一个 PHRASE category entry
	if systemDict != nil {
		coveredTo := make(map[string]bool, len(systemDict.PhraseRules))
		for _, e := range entries {
			coveredTo[e.Text] = true
		}
		phraseIdx := uint32(1 << 30) // 1G+ 避免与 qua ID 冲突
		for _, pr := range systemDict.PhraseRules {
			if coveredTo[pr.To] || pr.From == "" || pr.To == "" {
				continue
			}
			idStr := strconv.FormatUint(uint64(phraseIdx), 10)
			phraseIdx++
			entries = append(entries, lexicon.Entry{
				ID:   lexicon.EntryID(idStr),
				Text: pr.To,
				Meta: map[string]any{"category": "PHRASE", "priority": 50},
				Variants: []lexicon.Variant{
					{Text: pr.From, Kind: lexicon.VariantCorrection, Confidence: 1.0, Source: "system_phrase"},
				},
			})
			coveredTo[pr.To] = true // 避免同 to 多条 rule 重复加 entry
		}
	}

	// 3. Relations（保留跨 entry 的引用，alias / correction 链）
	var relations []lexicon.Relation
	for _, rs := range snap.Relations {
		for _, r := range rs {
			if r == nil {
				continue
			}
			kind, ok := variantKindFromRelationType(r.RelationType)
			if !ok {
				continue
			}
			fromID := strconv.FormatUint(uint64(r.EntryID), 10)
			toID := strconv.FormatUint(uint64(r.TargetEntryID), 10)
			if !idSet[fromID] || !idSet[toID] {
				continue // dangling relation
			}
			relations = append(relations, lexicon.Relation{
				From:   lexicon.EntryID(fromID),
				To:     lexicon.EntryID(toID),
				Kind:   kind,
				Weight: 1.0,
			})
		}
	}

	// 4. 排序 entries（确定性）
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Text < entries[j].Text
	})

	if version == "" {
		version = "v1"
	}
	return lexicon.NewBuilderWithVersion(version).
		Add(entries...).
		AddRelation(relations...).
		Build()
}

// variantKindFromRelationType 把 textenhance 的 RelationType 字符串映射成 lexnorm 的 VariantKind。
//
// 返回 (kind, ok)。ok=false 表示 unknown，应跳过。
func variantKindFromRelationType(rt string) (lexicon.VariantKind, bool) {
	switch rt {
	case "ALIAS":
		return lexicon.VariantAlias, true
	case "CORRECTION":
		return lexicon.VariantCorrection, true
	case "HOMOPHONE":
		return lexicon.VariantHomophone, true
	}
	return 0, false
}

// variantConfidenceFromKind 返回 lexnorm processor 默认期望的 confidence。
//
// 实际替换行为由 Config.AutoApplyThreshold/SuggestThreshold 决定，
// 这里设 Variant 自身 confidence=1.0 表示「这个变体本身就正确」，让
// fuzzy/alias/deterministic 处理器按阈值统一决策。
func variantConfidenceFromKind(k lexicon.VariantKind) float64 {
	return 1.0
}

// relationSource 返回 relation 的 source 标识（用于 audit）。
func relationSource(rt string) string {
	switch rt {
	case "ALIAS":
		return "alias"
	case "CORRECTION":
		return "correction"
	case "HOMOPHONE":
		return "homophone"
	}
	return rt
}

// sortVariants 按 kind 升序 + text 字典序排序（确定性）。
func sortVariants(vs []lexicon.Variant) {
	sort.Slice(vs, func(i, j int) bool {
		if vs[i].Kind != vs[j].Kind {
			return vs[i].Kind < vs[j].Kind
		}
		return vs[i].Text < vs[j].Text
	})
}
