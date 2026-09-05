// Package biz · vocabulary.go
// VocabularyBuilder：evie/tool 的词库快照构建器（per-tenant，HA）。
//
// 设计（M5）：
//   1. 启动时加载 system.json（系统静态词条；PLATFORM / SYSTEM scope）
//   2. 租户快照通过 UpdateTenant 由 VocabSyncer 异步刷新
//   3. Build(ctx, tenantID) 走 cache-aside：返回最后一份好快照
//   4. HA：sync 失败不影响主流程；空快照合法
//   5. 线程安全：Build（高频读）+ UpdateTenant（低频写）通过 RWMutex 保护
package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backend-service/pkg/textenhance/processors"

	"backend-service/app/evie/tool/internal/conf"
)

// VocabularyBuilder 词库快照构建器。
type VocabularyBuilder struct {
	conf       *conf.SystemDict
	systemDict *systemDictFile
	// systemDictPath M9.5 加载后的绝对路径（诊断 / hot reload 用）。
	systemDictPath string

	// 租户快照缓存（key = tenantID）
	tenantMu    sync.RWMutex
	tenantSnaps map[string]*processors.VocabularySnapshot

	// 全局 fallback（最后一次任意租户成功构建的快照；HA 兜底）
	fallbackMu sync.RWMutex
	fallback   *processors.VocabularySnapshot

	// lazySyncOnMiss 可选回调：Build 在 tenant 缓存 miss 时触发。
	// 用于“按需同步”：请求路径上首次访问某 tenant 时同步一次。
	// 返回 true 表示同步已发起；后续 Build 还会走 system/fallback 路径。
	// 业务侧注册 VocabSyncer.EnsureTenant。
	lazySyncOnMiss func(ctx context.Context, tenantID string) error
}

// systemDictFile system.json 文件结构。
type systemDictFile struct {
	Version     string             `json:"version"`
	Entries     []systemEntry      `json:"entries"`
	PhraseRules []systemPhraseRule `json:"phrase_rules"`
}

type systemEntry struct {
	StandardText string   `json:"standard_text"`
	Category     string   `json:"category"`
	Priority     int      `json:"priority"`
	Aliases      []string `json:"aliases"`
	Corrections  []string `json:"corrections"`
	Homophones   []string `json:"homophones"`
}

type systemPhraseRule struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// NewVocabularyBuilder 构造 builder 并加载系统词库。
func NewVocabularyBuilder(c *conf.SystemDict) (*VocabularyBuilder, error) {
	if c == nil {
		return nil, fmt.Errorf("biz: system_dict config is nil")
	}
	if c.Path == "" {
		return nil, fmt.Errorf("biz: system_dict.path is empty")
	}
	b := &VocabularyBuilder{
		conf:        c,
		tenantSnaps: make(map[string]*processors.VocabularySnapshot),
	}
	if err := b.loadSystemDict(); err != nil {
		// 启动期加载失败：记录 warn + 用空快照（不阻断启动）
		fmt.Fprintf(os.Stderr, "[vocabulary] load system dict failed: %v (using empty)\n", err)
		b.systemDict = &systemDictFile{}
	}
	return b, nil
}

// WithLazySyncOnMiss 设置 cache miss 时的懒同步回调。
func (b *VocabularyBuilder) WithLazySyncOnMiss(fn func(ctx context.Context, tenantID string) error) {
	b.lazySyncOnMiss = fn
}

// loadSystemDict 从 system.json 加载（启动期一次性）。
//
// 路径解析策略（多候选、绝对化）：服务启动 cwd 不固定（可能是 backend-service/ 或 evie/tool/），
// 按以下顺序尝试，任一成功即返回：
//  1. 原 config path（绝对路径直接用）
//  2. 原 config path（相对路径 → filepath.Abs）
//  3. 同路径加上 evie/tool 前缀（兼容 cwd=backend-service/）
//  4. 同路径去除 evie/tool 前缀（兼容 cwd=evie/tool/）
//  5. conf.Path 加 .yaml 变 .json 同名文件（误配 fallback）
//
// 失败的候选不会阻断启动期，使用空快照（fail-open）；启动后 hot reload 重新尝试。
func (b *VocabularyBuilder) loadSystemDict() error {
	candidates := b.resolveSystemDictCandidates()
	var lastErr error
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			lastErr = err
			continue
		}
		if _, statErr := os.Stat(abs); statErr != nil {
			lastErr = fmt.Errorf("stat %s: %w", abs, statErr)
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			lastErr = fmt.Errorf("read %s: %w", abs, err)
			continue
		}
		var f systemDictFile
		if err := json.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("unmarshal %s: %w", abs, err)
		}
		b.systemDict = &f
		b.systemDictPath = abs
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate paths configured")
	}
	return lastErr
}

// resolveSystemDictCandidates 从 b.conf.Path 产生多个候选绝对路径（不验证存在）。
func (b *VocabularyBuilder) resolveSystemDictCandidates() []string {
	p := b.conf.Path
	if p == "" {
		return nil
	}
	out := []string{p}

	// 补 / 去 "app/evie/tool/" 前缀以适配不同 cwd
	const prefix = "app/evie/tool/"
	if strings.HasPrefix(p, "./"+prefix) {
		out = append(out, "./"+strings.TrimPrefix(p, "./"+prefix))
		out = append(out, p)
	} else if strings.HasPrefix(p, prefix) {
		out = append(out, "./"+strings.TrimPrefix(p, prefix))
		out = append(out, "./"+p)
	} else if strings.HasPrefix(p, "./") {
		// ./configs/dictionaries/system.json 这种 → 补 / 去 app/evie/tool 前缀
		out = append(out, "./"+prefix+strings.TrimPrefix(p, "./"))
	}

	return out
}

// Build 为指定 tenant 构建 VocabularySnapshot。
//
// 查找顺序（cache-aside）：
//   1. tenant 缓存（sync worker 最近一次成功构建）
//   2. system 静态快照
//   3. fallback（最后一次任意租户成功；HA 兜底）
//   4. EmptyVocabularySnapshot（启动期 / 全失败）
//
// 返回值永不为 nil；调用方拿不到 error（HA 简化）。
func (b *VocabularyBuilder) Build(ctx context.Context, tenantID string) *processors.VocabularySnapshot {
	if tenantID != "" {
		b.tenantMu.RLock()
		snap := b.tenantSnaps[tenantID]
		b.tenantMu.RUnlock()
		if snap != nil {
			return snap
		}
		// cache miss：发起一次懒同步（不阻塞当前请求，下次 Build 会拿到）
		// 把 reqCtx 传给 callback（带 AuthInfo），由 callback 内部复制 auth 到独立 ctx。
		if b.lazySyncOnMiss != nil {
			go func(reqCtx context.Context, t string) {
				_ = b.lazySyncOnMiss(reqCtx, t)
			}(ctx, tenantID)
		}
	}

	// fallback: 尝试 system snapshot
	if sysSnap := b.buildSystemSnapshot(); sysSnap.EntryCount() > 0 || sysSnap.RelationCount() > 0 {
		return sysSnap
	}

	// 全局 fallback
	b.fallbackMu.RLock()
	fb := b.fallback
	b.fallbackMu.RUnlock()
	if fb != nil {
		return fb
	}

	// 启动期 / 全失败
	return processors.EmptyVocabularySnapshot()
}

// HasTenant 判断 tenant 是否已有非空 snapshot（用于 VocabSyncer.EnsureTenant 优化）。
func (b *VocabularyBuilder) HasTenant(tenantID string) (*processors.VocabularySnapshot, bool) {
	if tenantID == "" {
		return nil, false
	}
	b.tenantMu.RLock()
	snap, ok := b.tenantSnaps[tenantID]
	b.tenantMu.RUnlock()
	return snap, ok
}

// UpdateTenant 刷新某 tenant 的快照（vocab_sync 调用）。
//
// entries / relations 来自 qua API → Normalizer 转换后的通用词条。
// HA 行为：写入失败不抛；原快照保留。
func (b *VocabularyBuilder) UpdateTenant(tenantID string, entries []*processors.VocabularyEntry, relations []*processors.VocabularyRelation) {
	if tenantID == "" {
		return
	}
	// 合并：system entries + tenant entries（tenant 优先）
	mergedEntries, mergedRelations := b.mergeWithSystem(entries, relations)
	snap := processors.NewVocabularySnapshot(mergedEntries, mergedRelations)

	b.tenantMu.Lock()
	b.tenantSnaps[tenantID] = snap
	b.tenantMu.Unlock()

	// 同步更新全局 fallback（任意 tenant 成功都更新）
	b.fallbackMu.Lock()
	b.fallback = snap
	b.fallbackMu.Unlock()
}

// DeleteTenant 清理 tenant 快照（tenant 失效时调用；fallback 不删）。
func (b *VocabularyBuilder) DeleteTenant(tenantID string) {
	b.tenantMu.Lock()
	delete(b.tenantSnaps, tenantID)
	b.tenantMu.Unlock()
}

// ListTenants 返回已知 tenant 列表。
func (b *VocabularyBuilder) ListTenants() []string {
	b.tenantMu.RLock()
	defer b.tenantMu.RUnlock()
	out := make([]string, 0, len(b.tenantSnaps))
	for k := range b.tenantSnaps {
		out = append(out, k)
	}
	return out
}

// mergeWithSystem 合并 system 词条 + tenant 词条（tenant 优先）。
//
// 合并策略：
//   - system entries + tenant entries
//   - 同 standard_text：tenant 优先
//   - relations：全部合并（system + tenant）
//
// 重要（M9 修复）：重新分配 entry ID + 修复 relation.TargetEntryID，
// 避免 system 与 tenant ID 冲突（如 system "金种籽"=ID1 与 tenant "万康盛鼎集团"=ID1 同 ID，
// 导致 resolveTarget 找错 entry）。
func (b *VocabularyBuilder) mergeWithSystem(
	tenantEntries []*processors.VocabularyEntry,
	tenantRelations []*processors.VocabularyRelation,
) ([]*processors.VocabularyEntry, []*processors.VocabularyRelation) {
	// 1. system entries → base
	sysSnap := b.buildSystemSnapshot()

	// 2. tenant 优先，重新分配 ID
	var nextID uint32 = 1
	entries := make([]*processors.VocabularyEntry, 0, len(sysSnap.Entries)+len(tenantEntries))
	seen := make(map[string]bool, len(tenantEntries))
	oldIDToNew := make(map[uint32]uint32, len(tenantEntries))

	// 2a. tenant entries 先（用原 ID 但记下映射）
	for _, e := range tenantEntries {
		if e == nil || e.StandardText == "" {
			continue
		}
		newEntry := *e // copy
		newEntry.ID = nextID
		oldIDToNew[e.ID] = nextID
		nextID++
		entries = append(entries, &newEntry)
		seen[e.StandardText] = true
	}

	// 2b. system 补全（无重名时）
	for k, e := range sysSnap.Entries {
		if !seen[k] {
			newEntry := *e
			newEntry.ID = nextID
			oldIDToNew[e.ID] = nextID
			nextID++
			entries = append(entries, &newEntry)
		}
	}

	// 3. relations 合并 + 修复 TargetEntryID
	relations := make([]*processors.VocabularyRelation, 0, len(sysSnap.Relations)+len(tenantRelations))
	// 3a. system relations 先
	for _, rs := range sysSnap.Relations {
		for _, r := range rs {
			nr := *r
			if newID, ok := oldIDToNew[r.TargetEntryID]; ok {
				nr.TargetEntryID = newID
				nr.EntryID = newID
			}
			relations = append(relations, &nr)
		}
	}
	// 3b. tenant relations
	for _, r := range tenantRelations {
		nr := *r
		if newID, ok := oldIDToNew[r.TargetEntryID]; ok {
			nr.TargetEntryID = newID
			nr.EntryID = newID
		}
		relations = append(relations, &nr)
	}

	return entries, relations
}

// buildSystemSnapshot 从 systemDictFile 构造 VocabularySnapshot。
func (b *VocabularyBuilder) buildSystemSnapshot() *processors.VocabularySnapshot {
	if b.systemDict == nil {
		return processors.EmptyVocabularySnapshot()
	}

	entries := make([]*processors.VocabularyEntry, 0, len(b.systemDict.Entries))
	relations := make([]*processors.VocabularyRelation, 0)
	var nextID uint32 = 1

	for _, e := range b.systemDict.Entries {
		if e.StandardText == "" {
			continue
		}
		ent := &processors.VocabularyEntry{
			ID:            nextID,
			StandardText:  e.StandardText,
			Category:      e.Category,
			EntryType:     "WORD",
			Priority:      e.Priority,
		}
		entries = append(entries, ent)
		nextID++

		for _, alias := range e.Aliases {
			if alias == "" {
				continue
			}
			relations = append(relations, &processors.VocabularyRelation{
				EntryID:       ent.ID,
				RelationType:  "ALIAS",
				RelatedText:   alias,
				TargetEntryID: ent.ID,
			})
		}
		for _, corr := range e.Corrections {
			if corr == "" {
				continue
			}
			relations = append(relations, &processors.VocabularyRelation{
				EntryID:       ent.ID,
				RelationType:  "CORRECTION",
				RelatedText:   corr,
				TargetEntryID: ent.ID,
			})
		}
		for _, homo := range e.Homophones {
			if homo == "" {
				continue
			}
			relations = append(relations, &processors.VocabularyRelation{
				EntryID:       ent.ID,
				RelationType:  "HOMOPHONE",
				RelatedText:   homo,
				TargetEntryID: ent.ID,
			})
		}
	}

	return processors.NewVocabularySnapshot(entries, relations)
}

// ReloadSystemDict 热重载 system.json。
func (b *VocabularyBuilder) ReloadSystemDict() error {
	return b.loadSystemDict()
}

// TenantSnapshotInfo 单 tenant 快照信息（运维）。
type TenantSnapshotInfo struct {
	TenantID     string
	EntryCount   int
	RelationCount int
	LastSyncAt   time.Time
}

// GetTenantInfo 返回某 tenant 的快照信息。
func (b *VocabularyBuilder) GetTenantInfo(tenantID string) TenantSnapshotInfo {
	b.tenantMu.RLock()
	snap, ok := b.tenantSnaps[tenantID]
	b.tenantMu.RUnlock()
	if !ok {
		return TenantSnapshotInfo{TenantID: tenantID}
	}
	return TenantSnapshotInfo{
		TenantID:      tenantID,
		EntryCount:    snap.EntryCount(),
		RelationCount: snap.RelationCount(),
		LastSyncAt:    time.Now(),
	}
}

// ListTenantInfo 返回所有 tenant 的快照信息。
func (b *VocabularyBuilder) ListTenantInfo() []TenantSnapshotInfo {
	tenants := b.ListTenants()
	out := make([]TenantSnapshotInfo, 0, len(tenants))
	for _, t := range tenants {
		out = append(out, b.GetTenantInfo(t))
	}
	return out
}

// SystemInfo 系统词库信息。
type SystemInfo struct {
	SystemEntries   int
	SystemRelations int
	LoadedAt        time.Time
}

// SystemInfo 返回系统词库信息。
func (b *VocabularyBuilder) SystemInfo() SystemInfo {
	if b.systemDict == nil {
		return SystemInfo{}
	}
	return SystemInfo{
		SystemEntries:   len(b.systemDict.Entries),
		SystemRelations: b.countRelations(),
		LoadedAt:        time.Now(),
	}
}

func (b *VocabularyBuilder) countRelations() int {
	if b.systemDict == nil {
		return 0
	}
	n := 0
	for _, e := range b.systemDict.Entries {
		n += len(e.Aliases) + len(e.Corrections) + len(e.Homophones)
	}
	return n
}