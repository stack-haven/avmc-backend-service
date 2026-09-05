// Package biz 聚合 evie/tool 的所有业务用例 Provider。
//
// M9.6 重构：从 pkg/textenhance 迁移到 pkg/lexnorm。
//
// ProviderSet 变更：
//   - 删除：NewPolicyFromConf / NewEnhancementPipeline
//   - 新增：NewLexnormEngine / NewTenantProfileResolver
//   - 保留：NewNormalizerFromConf / NewVocabularyBuilder / NewEnhancementUsecase /
//     NewTenantRegistry / NewASRUsecase / NewVocabSyncerWithAuth
package biz

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/stack-haven/lexnorm"

	v1conf "backend-service/app/evie/tool/internal/conf"
)

// ProviderSet biz providers。
//
// Wire interface binding：data.QuaVocabularySource → biz.bizVocabularySource。
// 在 data.ProviderSet 中 NewQuaVocabularySource 返回 *quaVocabularySource（实现 biz.VocabularySource）。
// 这里显式 wire.Bind 让 wire 知道如何满足 NewVocabSyncer 对 bizVocabularySource 的依赖。
var ProviderSet = wire.NewSet(
	// M3c
	NewNormalizerFromConf,
	// M6c / M9.6
	NewVocabularyBuilder,
	NewTenantProfileResolver,
	NewLexnormEngine,
	NewEnhancementUsecase,
	// M5
	NewTenantRegistry,
	// M7
	NewASRUsecase,
)

// NewNormalizerFromConf 从 conf.VocabRules 构造 Normalizer（带 warn logger）。
func NewNormalizerFromConf(rules *v1conf.VocabRules, logger log.Logger) *Normalizer {
	rs := LoadRuleSet(rules)
	return NewNormalizerWithLogger(rs, log.NewHelper(log.With(logger, "module", "vocab/normalizer")))
}

// NewLexnormEngine 构造 lexnorm.Engine（注入 ProfileResolver + 默认 Profile）。
//
// engine 是并发安全的；Engine.Normalize 是高频调用入口。
// ProfileResolver 内部 lazy 调 VocabularyBuilder.Build(ctx, tenantID)，
// 自动触发按需 qua sync。
func NewLexnormEngine(
	c *v1conf.Enhancement,
	builder *VocabularyBuilder,
	logger log.Logger,
) (*lexnorm.Engine, error) {
	// 构造 per-tenant ProfileResolver
	resolver := NewTenantProfileResolver(builder, lexnorm.DefaultConfig(), logger)

	engine, err := lexnorm.New(
		lexnorm.WithProfileResolver(resolver),
		lexnorm.WithDefaultProfile(lexnorm.ProfileID("default")),
		// M9.6 hooks 留空：观测需求可后续加 lexnorm.Hook
	)
	if err != nil {
		return nil, err
	}
	return engine, nil
}
