// Package textenhance · registry.go
// Registry：processor 名称 → 工厂的注册表。
package textenhance

import (
	"fmt"
	"sync"

	"backend-service/pkg/textenhance/processors"
)

// Option 是跨 processor 的通用 Option 类型（type-erased）。
type Option = any

// ProcessorFactory 是 Processor 的工厂函数签名。
type ProcessorFactory func(opts ...Option) (processors.TextProcessor, error)

// Registry processor 注册表（并发安全）。
type Registry struct {
	mu        sync.RWMutex
	factories map[string]ProcessorFactory
}

// NewRegistry 构造空 Registry。
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]ProcessorFactory)}
}

// Register 注册一个 processor 工厂。
func (r *Registry) Register(name string, factory ProcessorFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Build 按 name 构造 processor 实例。
func (r *Registry) Build(name string, opts ...Option) (processors.TextProcessor, error) {
	r.mu.RLock()
	factory, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrProcessorNotFound(name)
	}
	return factory(opts...)
}

// Has 检查 processor 是否已注册。
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[name]
	return ok
}

// Names 返回已注册 processor 名称（顺序不固定）。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.factories))
	for n := range r.factories {
		out = append(out, n)
	}
	return out
}

// ToOptions 泛型辅助：把 []Option 转换为 []T。
func ToOptions[T any](opts []Option) ([]T, error) {
	out := make([]T, len(opts))
	for i, o := range opts {
		c, ok := o.(T)
		if !ok {
			var zero T
			return nil, fmt.Errorf("textenhance: expected %T, got %T", zero, o)
		}
		out[i] = c
	}
	return out, nil
}