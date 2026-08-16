package authz

import (
	"context"
	"fmt"
	"sync"
)

// providerRegistry 授权提供者注册表。
// 各 provider 包（casbin 等）通过 init() 调用 RegisterProvider 注册自身。
var providerRegistry = struct {
	sync.RWMutex
	providers map[string]AuthzProvider
}{
	providers: make(map[string]AuthzProvider),
}

// RegisterProvider 注册授权提供者。重复注册同名提供者会 panic，以尽早暴露冲突。
func RegisterProvider(provider AuthzProvider) {
	if provider == nil {
		panic("authz: register nil provider")
	}
	providerRegistry.Lock()
	defer providerRegistry.Unlock()
	name := provider.Name()
	if _, exists := providerRegistry.providers[name]; exists {
		panic(fmt.Sprintf("authz: provider %q already registered", name))
	}
	providerRegistry.providers[name] = provider
}

// GetProvider 按名称获取已注册的授权提供者。
func GetProvider(name string) (AuthzProvider, bool) {
	providerRegistry.RLock()
	defer providerRegistry.RUnlock()
	p, ok := providerRegistry.providers[name]
	return p, ok
}

// NewAuthorizer 按提供者名称创建授权器实例。
//
// 使用前需 import 对应 provider 包以触发注册，例如：
//
//	import _ "backend-service/pkg/auth/authz/casbin"
//	authorizer, err := authz.NewAuthorizer("casbin", ctx, opts...)
func NewAuthorizer(name string, ctx context.Context, opts ...Option) (Authorizer, error) {
	provider, ok := GetProvider(name)
	if !ok {
		return nil, fmt.Errorf("authz: provider %q not registered", name)
	}
	return provider.NewAuthorizer(ctx, opts...)
}

// ProviderNames 返回所有已注册的提供者名称（诊断用）。
func ProviderNames() []string {
	providerRegistry.RLock()
	defer providerRegistry.RUnlock()
	names := make([]string, 0, len(providerRegistry.providers))
	for name := range providerRegistry.providers {
		names = append(names, name)
	}
	return names
}
