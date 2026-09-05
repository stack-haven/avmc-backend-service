// Package biz 聚合 evie/tool 的所有业务用例 Provider。
//
// M0~M6c 阶段 ProviderSet：
//   - NewNormalizerFromConf：conf.VocabRules → Normalizer（M3c）
//   - NewVocabularyBuilder：加载 system.json + Build 快照（M6c）
//   - NewPolicyFromConf：conf.Enhancement → textenhance.Policy（M6c）
//   - NewEnhancementPipeline：textenhance.Registry + Policy + Observers → Pipeline（M6c）
//   - NewEnhancementUsecase：Pipeline + VocabularyBuilder + Policy（M6c）
//
// 后续 M5 加：NewVocabSyncer
// 后续 M7 加：NewASRUsecase
package biz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"
	"backend-service/pkg/textenhance/observers"
	"backend-service/pkg/textenhance/processors"

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
	// M6c
	NewVocabularyBuilder,
	NewPolicyFromConf,
	NewEnhancementPipeline,
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

// NewEnhancementPipeline 从 conf + 默认 Registry 构造 textenhance.Pipeline。
//
// 默认带 LoggingObserver（写到 kratos log）+ CountingObserver（统计）。
func NewEnhancementPipeline(
	c *v1conf.Enhancement,
	logger log.Logger,
) (*textenhance.Pipeline, error) {
	reg := builtins.NewDefaultRegistry()
	policy := NewPolicyFromConf(c)

	// 默认 observer 列表
	obs := []processors.Observer{
		observers.NewLoggingObserver(kratosToObserversLogger{log: logger}),
		observers.NewCountingObserver(),
	}
	return textenhance.BuildPipeline(reg, policy,
		textenhance.WithObservers(obs...),
	)
}

// kratosToObserversLogger 适配 kratos log.Logger → observers.Logger。
type kratosToObserversLogger struct {
	log log.Logger
}

func (k kratosToObserversLogger) WithContext(ctx context.Context) observers.Logger { return k }

func (k kratosToObserversLogger) Debugf(format string, args ...any) {
	_ = k.log.Log(log.LevelDebug, "module", "textenhance", "msg", fmt.Sprintf(format, args...))
}

func (k kratosToObserversLogger) Infof(format string, args ...any) {
	_ = k.log.Log(log.LevelInfo, "module", "textenhance", "msg", fmt.Sprintf(format, args...))
}

func (k kratosToObserversLogger) Warnf(format string, args ...any) {
	_ = k.log.Log(log.LevelWarn, "module", "textenhance", "msg", fmt.Sprintf(format, args...))
}

func (k kratosToObserversLogger) Errorf(format string, args ...any) {
	_ = k.log.Log(log.LevelError, "module", "textenhance", "msg", fmt.Sprintf(format, args...))
}