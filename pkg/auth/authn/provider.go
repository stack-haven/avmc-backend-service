package authn

import (
	"context"
	"fmt"
	"sync"
)

// providerRegistry 认证提供者注册表。
// 各 provider 包（jwt/oidc 等）通过 init() 调用 RegisterProvider 注册自身，
// 使用者只需 import 对应 provider 包（blank import）并按名称创建。
var providerRegistry = struct {
	sync.RWMutex
	providers map[string]AuthProvider
}{
	providers: make(map[string]AuthProvider),
}

// RegisterProvider 注册认证提供者。重复注册同名提供者会 panic，以尽早暴露冲突。
func RegisterProvider(provider AuthProvider) {
	if provider == nil {
		panic("authn: register nil provider")
	}
	providerRegistry.Lock()
	defer providerRegistry.Unlock()
	name := provider.Name()
	if _, exists := providerRegistry.providers[name]; exists {
		panic(fmt.Sprintf("authn: provider %q already registered", name))
	}
	providerRegistry.providers[name] = provider
}

// GetProvider 按名称获取已注册的认证提供者。
func GetProvider(name string) (AuthProvider, bool) {
	providerRegistry.RLock()
	defer providerRegistry.RUnlock()
	p, ok := providerRegistry.providers[name]
	return p, ok
}

// NewAuthenticator 按提供者名称创建认证器实例。
//
// 使用前需 import 对应 provider 包以触发注册，例如：
//
//	import _ "backend-service/pkg/auth/authn/jwt"
//	authenticator, err := authn.NewAuthenticator("jwt", ctx, opts...)
func NewAuthenticator(name string, ctx context.Context, opts ...Option) (Authenticator, error) {
	provider, ok := GetProvider(name)
	if !ok {
		return nil, fmt.Errorf("authn: provider %q not registered", name)
	}
	return provider.NewAuthenticator(ctx, opts...)
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
