// Package biz · rule_loader.go
// RuleSet 从 conf.VocabRules（proto 类型）加载，纯函数无副作用。
//
// 设计要点：
//   1. conf.VocabRules（proto）→ biz.RuleSet（领域）单一转换函数
//   2. 不做规则校验（warn 由 Normalizer 处理；C 决定）
//   3. 不做缓存；调用方（data.NewRuleLoaderFromConf）按需加载
package biz

import (
	v1conf "backend-service/app/evie/tool/internal/conf"
)

// LoadRuleSet 从 conf.VocabRules 构造 RuleSet。
//
// 字段映射规则：
//   conf.VocabRules.sources[<source>]      → RuleSet.Sources[<source>]
//   conf.VocabRules.SourceRules.entity_mappings → SourceRules.EntityMappings
//   conf.VocabRules.EntityMapping.emit.*    → EntityMapping.Emit.*
func LoadRuleSet(cfg *v1conf.VocabRules) *RuleSet {
	rs := &RuleSet{Sources: make(map[string]*SourceRules)}
	if cfg == nil {
		return rs
	}
	for source, sr := range cfg.GetSources() {
		if sr == nil {
			continue
		}
		bizSR := &SourceRules{Source: source}
		for _, em := range sr.GetEntityMappings() {
			if em == nil {
				continue
			}
			bizSR.EntityMappings = append(bizSR.EntityMappings, EntityMapping{
				Match: MatchCondition{
					EntityType: em.GetMatch().GetEntityType(),
				},
				Emit: EmitSpec{
					StandardText: em.GetEmit().GetStandardText(),
					Category:     em.GetEmit().GetCategory(),
					Aliases:      em.GetEmit().GetAliases(),
					PinyinHint:   em.GetEmit().GetPinyinHint(),
					Priority:     em.GetEmit().GetPriority(),
					IncludeWhen:  em.GetEmit().GetIncludeWhen(),
				},
			})
		}
		rs.Sources[source] = bizSR
	}
	return rs
}

// MergeRuleSet 合并两个 RuleSet（后者覆盖前者；用于 YAML 热加载叠加）。
//
// 设计：v1 简单实现；M5/M9 阶段再考虑并发安全。
func MergeRuleSet(base, override *RuleSet) *RuleSet {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}
	out := &RuleSet{Sources: make(map[string]*SourceRules, len(base.Sources)+len(override.Sources))}
	for k, v := range base.Sources {
		out.Sources[k] = v
	}
	for k, v := range override.Sources {
		out.Sources[k] = v
	}
	return out
}