// Package data 聚合 evie/tool 的所有基础设施 Provider（Redis client、HTTP client、文件加载器）。
package data

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	"backend-service/app/evie/tool/internal/conf"
)

// ProviderSet data providers（按依赖方向注入）。
//
// 顺序约束（Wire 决定）：
//   conf.Data          → RedisClient → TokenCache
//   conf.Qua        → QuaFetcher → QuaVocabularySource
//   conf.Asr        → ASRRegistry
var ProviderSet = wire.NewSet(
	NewRedisClient,
	NewRedisConf,
	NewTokenCache,
	NewASRRegistry,
	NewASRProviders,
	NewQuaClient,
	NewQuaVocabularySource,
	NewVocabularySourceRegistry,
	NewQuaClientOptions, // 空 slice（测试 / 配置化在 M9 阶段接）
	NewHealthChecker, // M9: 健康检查（返回接口，由 wire 推断）
	// M4: NewSystemDictLoader
)

// HealthProviderSet 预留（健康检查已合并到 ProviderSet）。
var HealthProviderSet = wire.NewSet()

// NewQuaClientOptions 返回空的 QuaClientOption 列表（Wire 要求显式 provider）。
func NewQuaClientOptions() []QuaClientOption {
	return nil
}

// NewRedisConf 从 *conf.Data 抽出 *conf.Data_Redis（Wire field extractor）。
func NewRedisConf(d *conf.Data) *conf.Data_Redis {
	if d == nil {
		return nil
	}
	return d.Redis
}

// Data 聚合本工具运行时所有基础设施句柄。
type Data struct {
	conf *conf.Bootstrap
}

// NewData 创建 Data 聚合实例（M0 阶段仅占位；M7+ 阶段会用到）。
func NewData(c *conf.Bootstrap, logger log.Logger) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("evie/tool: closing data resources")
	}
	return &Data{conf: c}, cleanup, nil
}