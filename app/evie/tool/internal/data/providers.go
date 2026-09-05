// Package data · providers.go
// ASR Provider 装配：按 conf.Asr 配置按名启用 Provider 注册到 pkg/asr.ProviderRegistry。
//
// 路由约定（业务层使用）：
//   batch  → conf.Asr.DefaultBatchProvider
//   stream → conf.Asr.DefaultStreamProvider
// 若首选 Provider 未 enabled，自动降级到 Providers 中第一个 enabled。
package data

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/pkg/asr"
	"backend-service/pkg/asr/funasr"
	"backend-service/pkg/asr/xunfei"
)

// NewASRRegistry 根据 conf.Asr 创建并填充 ProviderRegistry。
//
// 规则：
//   - providers.funasr.enabled = true  → 注册 funasr（addr + sample_rate + language）
//   - providers.xunfei.enabled  = true  → 注册 xunfei（host/app_id/api_key/api_secret/uri）
//   - providers.whisper.enabled = true  → 注册 whisper（预留，需后续 pkg/asr/whisper 实现）
//   - providers.aliyun.enabled  = true  → 注册 aliyun（预留）
//
// 即使所有 provider 都 enabled = false，registry 仍会返回；调用方拿到 ErrProviderNotFound。
func NewASRRegistry(c *conf.Asr, logger log.Logger) (*asr.ProviderRegistry, error) {
	if c == nil {
		return nil, fmt.Errorf("asr config is required")
	}
	reg := asr.NewProviderRegistry()
	logHelper := log.NewHelper(log.With(logger, "module", "asr/registry"))

	// 1. FunASR（整段批量，本地部署）
	if c.Providers.Funasr.GetEnabled() {
		addr := c.Providers.Funasr.GetAddr()
		if addr == "" {
			return nil, fmt.Errorf("asr.providers.funasr.enabled=true but addr is empty")
		}
		// funasr.Config 用 JSON 配置，简化此处直接构造
		provider, err := funasr.New(funasr.Config{
			Addr:       addr,
			StreamAddr: c.Providers.Funasr.GetStreamAddr(),
			SampleRate: int(c.Providers.Funasr.GetSampleRate()),
			Language:   c.Providers.Funasr.GetLanguage(),
		})
		if err != nil {
			return nil, fmt.Errorf("funasr provider: %w", err)
		}
		reg.Register(provider)
		logHelper.Infof("registered funasr provider addr=%s stream=%q", addr, c.Providers.Funasr.GetStreamAddr())
	}

	// 2. 讯飞 IAT（流式，云 API）
	if c.Providers.Xunfei.GetEnabled() {
		host := c.Providers.Xunfei.GetHost()
		appID := c.Providers.Xunfei.GetAppId()
		apiKey := c.Providers.Xunfei.GetApiKey()
		apiSecret := c.Providers.Xunfei.GetApiSecret()
		if host == "" || appID == "" || apiKey == "" || apiSecret == "" {
			return nil, fmt.Errorf("asr.providers.xunfei requires host/app_id/api_key/api_secret")
		}
		provider, err := xunfei.New(xunfei.Config{
			Host:      host,
			AppID:     appID,
			APIKey:    apiKey,
			APISecret: apiSecret,
			URI:       c.Providers.Xunfei.GetUri(),
		})
		if err != nil {
			return nil, fmt.Errorf("xunfei provider: %w", err)
		}
		reg.Register(provider)
		logHelper.Infof("registered xunfei provider host=%s uri=%s", host, c.Providers.Xunfei.GetUri())
	}

	// 3. whisper / aliyun 预留：等待 pkg/asr/whisper 与 pkg/asr/aliyun 实现完成
	if c.Providers.Whisper.GetEnabled() {
		logHelper.Warnf("whisper provider enabled but pkg/asr/whisper not implemented yet")
	}
	if c.Providers.Aliyun.GetEnabled() {
		logHelper.Warnf("aliyun provider enabled but pkg/asr/aliyun not implemented yet")
	}

	return reg, nil
}

// ResolveProvider 从 registry 取指定 provider；未注册时尝试按 enabled 顺序取第一个。
func ResolveProvider(reg *asr.ProviderRegistry, name string, enabledNames []string) (asr.ASRProvider, error) {
	if reg == nil {
		return nil, fmt.Errorf("asr registry is nil")
	}
	if name != "" {
		if p, err := reg.Get(name); err == nil {
			return p, nil
		}
	}
	// 降级：按 enabled 顺序取第一个
	for _, n := range enabledNames {
		if p, err := reg.Get(n); err == nil {
			return p, nil
		}
	}
	return nil, asr.ErrProviderNotFound
}

// EnabledProviderNames 返回按配置顺序的已启用 provider 名（用于降级路由）。
func EnabledProviderNames(c *conf.Asr) []string {
	if c == nil {
		return nil
	}
	var names []string
	if c.Providers.Funasr.GetEnabled() {
		names = append(names, "funasr")
	}
	if c.Providers.Xunfei.GetEnabled() {
		names = append(names, "xunfei")
	}
	if c.Providers.Whisper.GetEnabled() {
		names = append(names, "whisper")
	}
	if c.Providers.Aliyun.GetEnabled() {
		names = append(names, "aliyun")
	}
	return names
}

// NewASRProviders 按 conf.Asr 同时解析 Batch + Stream provider 并返回 *biz.ASRProviders。
//
// 至少一个成功；都失败时 wire 不会 panic，由 NewASRUsecase 校验。
func NewASRProviders(reg *asr.ProviderRegistry, c *conf.Asr) *biz.ASRProviders {
	out := &biz.ASRProviders{}
	if reg == nil || c == nil {
		return out
	}
	if p, _ := ResolveProvider(reg, c.GetDefaultBatchProvider(), EnabledProviderNames(c)); p != nil {
		out.Batch = p
	}
	if p, _ := ResolveProvider(reg, c.GetDefaultStreamProvider(), EnabledProviderNames(c)); p != nil {
		out.Stream = p
	}
	return out
}