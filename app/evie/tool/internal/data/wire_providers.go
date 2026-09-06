package data

import (
	"fmt"

	"github.com/redis/go-redis/v9"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
)

// NewTokenCacheProvider 依据 conf.Credential 选 TokenLookup：
//
//   - provider == "static"：构造 StaticTokenCache（demo / 离线模式）。
//   - 否则：fallback 到 Redis TokenCache。
//
// Wire 注入此函数，返回 TokenLookup（接口），下游 NewTokenAuthMiddleware
// 无需感知具体实现。
func NewTokenCacheProvider(
	cred *conf.Credential,
	rdb *redis.Client,
	redisConf *conf.Data_Redis,
) (TokenLookup, error) {
	if cred != nil && cred.Provider == "static" && cred.Static != nil {
		return NewStaticTokenCache(cred.Static)
	}
	if rdb == nil {
		return nil, fmt.Errorf("data: redis client required when not using static credential")
	}
	return NewTokenCache(rdb, redisConf), nil
}

// NewVocabularySourceProvider 依据 conf.Vocabulary 选 biz.VocabularySource：
//
//   - source == "file"：从本地 JSON 加载（demo / 离线模式）。
//   - 否则：fallback 到 qua HTTP 源。
//
// quaFetcher 可为 nil（demo 模式下不需要 qua）。
func NewVocabularySourceProvider(
	vocab *conf.Vocabulary,
	quaFetcher QuaFetcher,
) (biz.VocabularySource, error) {
	if vocab != nil && vocab.Source == "file" && vocab.File != nil {
		return NewFileVocabularySource(vocab.File)
	}
	if quaFetcher == nil {
		return nil, fmt.Errorf("data: qua fetcher required when not using file vocabulary")
	}
	return NewQuaVocabularySource(quaFetcher), nil
}
