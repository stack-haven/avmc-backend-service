package asr

import "sync"

// ProviderRegistry 是 ASR 供应商注册中心。
//
// 各供应商实现应在服务启动时通过 Register 注册，运行时按名称路由，
// 避免业务层硬编码具体引擎。注册中心是并发安全的，可在启动阶段注册、
// 运行阶段并发读取。
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]ASRProvider
}

// NewProviderRegistry 创建空的注册中心。
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{providers: make(map[string]ASRProvider)}
}

// Register 注册一个供应商（同名覆盖）。nil 会被忽略。
func (r *ProviderRegistry) Register(p ASRProvider) {
	if p == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Get 按名称获取供应商；未注册时返回 ErrProviderNotFound。
func (r *ProviderRegistry) Get(name string) (ASRProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return p, nil
}

// List 返回所有已注册供应商的能力声明（含 Name）。
func (r *ProviderRegistry) List() []ProviderCapabilities {
	r.mu.RLock()
	defer r.mu.RUnlock()
	caps := make([]ProviderCapabilities, 0, len(r.providers))
	for _, p := range r.providers {
		caps = append(caps, p.Capabilities())
	}
	return caps
}

// Names 返回所有已注册供应商名称。
func (r *ProviderRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
