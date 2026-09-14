// Package biz 聚合 evie/tool 的所有业务用例 Provider。
//
// v1.6 减法：删除 NewTenantRegistry（tenants.json / TenantRegistry 完全剔除）。
// 所有租户数据通过请求 ctx.AuthContext 动态获取，依赖 lazy sync + TTL。
package biz

import (
	"context"

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
	NewEnhancementUsecaseWithConf,
	// M7
	NewASRUsecase,
	NewVocabSyncerWithAuth,
	// v1.7 减法后的保活：把 VocabSyncer.lazySync 暴露为函数，
	// 供 NewLexnormEngine 注入；wire 自动连接 syncer 依赖链。
	NewLazySyncFunc,
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
//
// 阈值设计（修复 P2）：lexnorm.Config 默认 AutoApplyThreshold=0.95 太严，
// 会拦截 biz 层 fuzzy_vocab 已经按 Category 判定好的 0.65 替换。
// v1.2 (P7/P8 修复后): 全局阈值与 fuzzy_vocab.CategoryAuto[PERSON]=0.85 对齐。
//
// 设计：
//   - AutoApplyThreshold=0.85：只接受字面 Hamming dist=1 (conf=0.95) 自动 Apply
//   - SuggestThreshold=0.50：pinyin 命中 (conf=0.55) 走 Suggest，不 Apply
//   - 默认 lexnorm.DefaultConfig()=0.95/0.65 太严，会导致 fuzzy_vocab
//     完全不 Apply；0.5/0.0 又太松，会导致 pinyin 命中破坏正常词。
func NewLexnormEngine(
	c *v1conf.Enhancement,
	builder *VocabularyBuilder,
	lazySync func(context.Context, string) error,
	logger log.Logger,
) (*lexnorm.Engine, error) {
	cfg := lexnorm.DefaultConfig()
	cfg.AutoApplyThreshold = 0.85 // v1.2: 与 fuzzy_vocab.CategoryAuto[PERSON] 对齐
	cfg.SuggestThreshold = 0.50   // v1.2: pinyin conf=0.55 走 Suggest

	// 构造 per-tenant ProfileResolver（注入 lazySync 以注册 cache miss 回调）
	resolver := NewTenantProfileResolver(builder, lazySync, cfg, logger)

	engine, err := lexnorm.New(
		lexnorm.WithProfileResolver(resolver),
		lexnorm.WithDefaultProfile(lexnorm.ProfileID("default")),
		lexnorm.WithConfig(cfg),
		// M9.6 hooks 留空：观测需求可后续加 lexnorm.Hook
	)
	if err != nil {
		return nil, err
	}
	return engine, nil
}
