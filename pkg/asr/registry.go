package asr

import "fmt"

// ProviderRegistry ASR 供应商注册中心。
// 所有 Provider 在服务启动时注册，运行时按租户配置路由。
type ProviderRegistry struct {
	providers map[string]ASRProvider
}

// NewProviderRegistry 创建空的注册中心。
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{providers: make(map[string]ASRProvider)}
}

// Register 注册一个供应商（同名覆盖）。
func (r *ProviderRegistry) Register(p ASRProvider) {
	if p == nil {
		return
	}
	r.providers[p.Name()] = p
}

// Get 按名称获取供应商。
func (r *ProviderRegistry) Get(name string) (ASRProvider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown asr provider: %s", name)
	}
	return p, nil
}

// List 返回所有已注册供应商的能力集。
func (r *ProviderRegistry) List() []ProviderCapabilities {
	caps := make([]ProviderCapabilities, 0, len(r.providers))
	for _, p := range r.providers {
		caps = append(caps, p.Capabilities())
	}
	return caps
}

// Names 返回所有已注册供应商名称。
func (r *ProviderRegistry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
