package data

import (
	"fmt"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
)

// NewTokenCacheFromConf 根据 conf.Credential 选 static TokenLookup。
//
// conf.Credential 为空或 provider != "static" 时返回错误（由调用方 fallback 到 Redis 路径）。
func NewTokenCacheFromConf(cred *conf.Credential) (TokenLookup, error) {
	if cred == nil || cred.Provider != "static" || cred.Static == nil {
		return nil, fmt.Errorf("data: static credential not configured")
	}
	return NewStaticTokenCache(cred.Static)
}

// NewVocabularySourceFromConf 根据 conf.Vocabulary 选择 biz.VocabularySource。
//
// conf.Vocabulary 为空或 source != "file" 时返回错误（由调用方 fallback 到 qua 路径）。
func NewVocabularySourceFromConf(vocab *conf.Vocabulary) (biz.VocabularySource, error) {
	if vocab == nil || vocab.Source != "file" || vocab.File == nil {
		return nil, fmt.Errorf("data: file vocabulary not configured")
	}
	return NewFileVocabularySource(vocab.File)
}
