// Package biz · vocab_sync.go
// VocabSyncer：后台 worker，定期从 qua 拉取用户/部门并 Normalizer 转换后
// 更新 VocabularyBuilder 的 per-tenant 快照。
//
// 设计：
//  1. 启动时 Warmup() 全量预热已发现的 tenant（从 tenant_registry）
//  2. 后台 ticker 周期同步（默认 5min）
//  3. ctx 取消时优雅退出
//  4. HA：sync 失败 warn 不阻断；旧快照继续服务
//  5. tenant 列表来源：tenant_registry（持久化）
package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/evie/tool/internal/conf"
	"backend-service/pkg/textenhance/processors"
)

// TenantRegistry 简化版：持久化的 tenant 列表。
//
// 真实实现从 tenant_registry.json 读 + qua 实时发现；本期用简单 map。
type TenantRegistry struct {
	mu      sync.RWMutex
	tenants map[string]bool
}

// NewTenantRegistry 从 conf 构造（启动时读 tenant_registry.path 文件）。
//
// M5 阶段：先从文件读；文件缺失用空 map；qua 同步时新增 tenant 自动加入。
func NewTenantRegistry(c *conf.TenantRegistry) *TenantRegistry {
	r := &TenantRegistry{tenants: make(map[string]bool)}
	if c == nil || c.Path == "" {
		return r
	}
	// 简化：不读文件（M5 阶段后续补）；qua 同步时动态发现
	return r
}

// Ensure 注册新 tenant（qua 同步时调用）。
func (r *TenantRegistry) Ensure(tenantID string) bool {
	if tenantID == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tenants[tenantID] {
		return false // 已存在
	}
	r.tenants[tenantID] = true
	return true // 新增
}

// List 返回已知 tenant 列表。
func (r *TenantRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.tenants))
	for k := range r.tenants {
		out = append(out, k)
	}
	return out
}

// VocabSyncer 后台同步 worker。
type VocabSyncer struct {
	registry    *TenantRegistry
	vocab       *VocabularyBuilder
	normalizer  *Normalizer
	quaSource   VocabularySource
	interval    time.Duration
	concurrency int
	log         *log.Helper

	// canQuaFetch 判断 ctx 是否可调 qua（避免启动期无 token 报 401）。
	// 由 wire 注入，默认返回 true。
	canQuaFetch func(ctx context.Context) bool
}

// SyncerOption 配置函数（Functional Options 模式）。
type SyncerOption func(*VocabSyncer)

// WithCanQuaFetch 注入 ctx AuthInfo 检测函数。
func WithCanQuaFetch(fn func(ctx context.Context) bool) SyncerOption {
	return func(s *VocabSyncer) {
		if fn != nil {
			s.canQuaFetch = fn
		}
	}
}

// NewVocabSyncer 构造 syncer。
//
// quaSource 使用 biz.VocabularySource 接口（data.NewQuaVocabularySource 已实现）。
func NewVocabSyncer(
	registry *TenantRegistry,
	vocab *VocabularyBuilder,
	normalizer *Normalizer,
	quaSource VocabularySource,
	c *conf.TenantVocab,
	logger log.Logger,
	opts ...SyncerOption,
) *VocabSyncer {
	interval := 5 * time.Minute
	if c != nil && c.SyncInterval != nil {
		interval = c.SyncInterval.AsDuration()
	}
	concurrency := 4
	if c != nil && c.Concurrency > 0 {
		concurrency = int(c.Concurrency)
	}
	s := &VocabSyncer{
		registry:    registry,
		vocab:       vocab,
		normalizer:  normalizer,
		quaSource:   quaSource,
		interval:    interval,
		concurrency: concurrency,
		log:         log.NewHelper(log.With(logger, "module", "biz/vocab_sync")),
		canQuaFetch: func(ctx context.Context) bool { return true }, // 默认不限制
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// NewVocabSyncerWithAuth 是 wire 专用的注入器：在 NewVocabSyncer 上
// 额外注入 ctx AuthInfo 检测（避免启动期 warmup 调 qua 时报 401）。
//
// 同时装上 lazy sync 回调（cache miss 时按需同步）。
func NewVocabSyncerWithAuth(
	registry *TenantRegistry,
	vocab *VocabularyBuilder,
	normalizer *Normalizer,
	quaSource VocabularySource,
	c *conf.TenantVocab,
	logger log.Logger,
	canQuaFetch func(ctx context.Context) bool,
) *VocabSyncer {
	s := NewVocabSyncer(registry, vocab, normalizer, quaSource, c, logger,
		WithCanQuaFetch(canQuaFetch),
	)
	// 装上 cache miss 回调（不阻塞当前请求，goroutine 异步同步）。
	s.AttachLazySync(vocab)
	return s
}

// Warmup 启动期全量预热（拉一次 qua + 对已注册 tenant 同步）。
//
// 不阻塞主流程太久：设 30s timeout。
//
// 注意：qua 调用需要 ctx 里携带 AuthInfo（用户 token），启动期如果没有
// 共享 service account，会跳过全量预热、改为请求级按需同步。
func (s *VocabSyncer) Warmup(ctx context.Context) {
	// 优雅跳过：启动期无 AuthInfo 的场景（最常见）。
	if s.canQuaFetch != nil && !s.canQuaFetch(ctx) {
		s.log.Info("vocab warmup skipped: no AuthInfo in ctx (will sync per-request)")
		return
	}

	warmupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 拉一次 qua，把所有发现的 tenant 注册到 registry
	raws, err := s.quaSource.Fetch(warmupCtx)
	if err != nil {
		s.log.Warnf("warmup qua fetch failed: %v", err)
		return
	}

	// 注册发现的 tenant
	discoveredTenants := make(map[string]bool)
	for _, r := range raws {
		if r.Source != "" {
			discoveredTenants[r.Source] = true
		}
	}
	for t := range discoveredTenants {
		s.registry.Ensure(t)
	}

	// 同步已注册的所有 tenant
	tenants := s.registry.List()
	s.log.Infof("vocab warmup: %d tenants to sync", len(tenants))
	for _, t := range tenants {
		if err := s.SyncTenant(warmupCtx, t); err != nil {
			s.log.Warnf("warmup tenant %s: %v", t, err)
		}
	}
}

// Run 后台 ticker 循环；ctx 取消时退出。
func (s *VocabSyncer) Run(ctx context.Context) {
	s.log.Infof("vocab sync started, interval=%v", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 立即跑一次（不等第一个 tick）
	s.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			s.log.Info("vocab sync stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// runOnce 单次全量同步（拉 qua + 同步所有 tenant）。
func (s *VocabSyncer) runOnce(ctx context.Context) {
	// 无 AuthInfo 跳过：避免启动期 ticker 第一个 tick 就报 401。
	if s.canQuaFetch != nil && !s.canQuaFetch(ctx) {
		s.log.Debug("vocab runOnce skipped: no AuthInfo in ctx")
		return
	}
	raws, err := s.quaSource.Fetch(ctx)
	if err != nil {
		s.log.Warnf("sync qua fetch failed: %v", err)
		return
	}

	// 按 tenant 分组
	byTenant := make(map[string][]RawEntity)
	for _, r := range raws {
		byTenant[r.Source] = append(byTenant[r.Source], r)
	}

	// 注册新发现的 tenant
	for t := range byTenant {
		s.registry.Ensure(t)
	}

	// 同步每个 tenant
	tenants := s.registry.List()
	for _, t := range tenants {
		if err := s.SyncTenant(ctx, t); err != nil {
			s.log.Warnf("sync tenant %s: %v", t, err)
		}
	}
}

// SyncTenant 同步单 tenant（qua raw → Normalizer → entries/relations → UpdateTenant）。
//
// 返回 error 仅用于 logging；调用方不需处理。
//
// 设计说明（M5 → M9 修正）：quaSource 拉到的所有 RawEntity 都属于同一租户（qua 端
// 按 tenant-id 头隔离数据），所以无需再过滤；所有 raws 直接进入 Normalizer。
func (s *VocabSyncer) SyncTenant(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return nil
	}

	// 拉 raw（partial-failure 容忍：user/dept 任一失败仍用已拿到的数据）
	allRaws, err := s.quaSource.Fetch(ctx)
	if err != nil {
		s.log.Warnf("fetch partial error（接受已拉到的实体）: %v", err)
	}
	if len(allRaws) == 0 {
		return fmt.Errorf("fetch: no data (and error: %w)", err)
	}

	// Normalizer 转换（当前实现下，所有 raws 属于同一 tenant，无需再过滤）
	entries, rels, err := s.convertRawToVocab(allRaws)
	if err != nil {
		return fmt.Errorf("normalize: %w", err)
	}

	// 更新 builder
	s.vocab.UpdateTenant(tenantID, entries, rels)
	s.log.Infof("synced tenant %s: %d entries, %d relations", tenantID, len(entries), len(rels))
	return nil
}

// EnsureTenant 保证某 tenant 的 vocab snapshot 已存在。如果 tenant 未注册过，
// 主动调 qua 同步一次；同步失败不阻塞调用方（返回 error 仅用于日志）。
//
// 设计意图：请求路径上首次访问某 tenant 时调用，避免 5min ticker 期间的真空期。
// 单 tenant 内部串行（避免重复同步同 tenant）。
func (s *VocabSyncer) EnsureTenant(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return nil
	}
	if _, ok := s.vocab.HasTenant(tenantID); ok {
		return nil // 已有 snapshot，跳过
	}
	s.registry.Ensure(tenantID)
	return s.SyncTenant(ctx, tenantID)
}
func (s *VocabSyncer) convertRawToVocab(raws []RawEntity) ([]*processors.VocabularyEntry, []*processors.VocabularyRelation, error) {
	normalized, err := s.normalizer.NormalizeBatch(raws)
	if err != nil {
		return nil, nil, err
	}

	entries := make([]*processors.VocabularyEntry, 0, len(normalized))
	relations := make([]*processors.VocabularyRelation, 0)
	var nextID uint32 = 1
	entryIDByText := make(map[string]uint32, len(normalized))

	for _, n := range normalized {
		e := &processors.VocabularyEntry{
			ID:            nextID,
			StandardText:  n.StandardText,
			Category:      n.Category,
			EntryType:     "WORD",
			Pinyin:        derivePinyin(n.StandardText),
			PinyinInitial: derivePinyinInitial(n.StandardText),
		}
		entries = append(entries, e)
		entryIDByText[n.StandardText] = nextID
		nextID++

		// ALIAS 关系
		for _, a := range n.Aliases {
			relations = append(relations, &processors.VocabularyRelation{
				EntryID:       e.ID,
				RelationType:  "ALIAS",
				RelatedText:   a,
				TargetEntryID: e.ID,
			})
		}
	}

	return entries, relations, nil
}

// derivePinyin 简化版：实际生产应调 pkg/pinyin。
// 这里先占位（M5 后续接）。
func derivePinyin(s string) string { return "" }

// derivePinyinInitial 简化版。
func derivePinyinInitial(s string) string { return "" }

// AttachLazySync 把 VocabularyBuilder 的 cache miss 回调装为自己。
//
// wireApp 中只需调用一次：vocabSyncer.AttachLazySync(vocabBuilder)。
//
// 设计动机（M9.5）：避免在 wireApp 手写 lambda 包装 ctx。
// lazySync 是 VocabSyncer 方法，内部从 biz.AuthFrom 复制 auth 到
// 独立 timeout ctx，再调 EnsureTenant（不被 ASR 请求 cancel 打断）。
func (s *VocabSyncer) AttachLazySync(b *VocabularyBuilder) {
	b.WithLazySyncOnMiss(s.lazySync)
}

// lazySync VocabularyBuilder.Build cache miss 时调用的回调（goroutine 内运行）。
//
// 入参 reqCtx 带 AuthContext（中间件注入），但 8s 后被 ASR 请求 cancel。
// 复制 AuthContext 到 background ctx 后丢弃原 ctx，让 qua 调用有 token 且不被 cancel。
func (s *VocabSyncer) lazySync(reqCtx context.Context, tenantID string) error {
	syncCtx, cancel := context.WithTimeout(CopyAuthContext(context.Background(), reqCtx), 15*time.Second)
	defer cancel()
	return s.EnsureTenant(syncCtx, tenantID)
}
