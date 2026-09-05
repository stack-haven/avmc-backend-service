// Package data · source_registry.go
// VocabularySourceRegistry：多 source 聚合。
//
// 设计要点（Q13）：
//   1. 未来加新 source（如飞书/LDAP）只调 Register，无需修改 Normalizer
//   2. Registry 是并发安全的 map wrapper
//   3. VocabSyncer（M5）通过 All() 拿到全部 source 同步
package data

import (
	"sync"

	"backend-service/app/evie/tool/internal/biz"
)

// VocabularySourceRegistry 多 source 注册表。
type VocabularySourceRegistry struct {
	mu      sync.RWMutex
	sources map[string]biz.VocabularySource
}

// NewVocabularySourceRegistry 构造空 Registry。
func NewVocabularySourceRegistry() *VocabularySourceRegistry {
	return &VocabularySourceRegistry{sources: make(map[string]biz.VocabularySource)}
}

// Register 注册一个 VocabularySource（同名覆盖）。
func (r *VocabularySourceRegistry) Register(s biz.VocabularySource) {
	if s == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[s.Name()] = s
}

// Get 按名称获取 source；未注册返回 nil。
func (r *VocabularySourceRegistry) Get(name string) biz.VocabularySource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sources[name]
}

// All 返回全部已注册 source 的快照（map copy，避免并发修改）。
func (r *VocabularySourceRegistry) All() []biz.VocabularySource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]biz.VocabularySource, 0, len(r.sources))
	for _, s := range r.sources {
		out = append(out, s)
	}
	return out
}

// Names 返回已注册 source 名称（用于运维接口）。
func (r *VocabularySourceRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.sources))
	for name := range r.sources {
		out = append(out, name)
	}
	return out
}

// 编译期断言
var _ biz.VocabularySource = (*quaVocabularySource)(nil)