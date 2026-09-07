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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pkgpinyin "backend-service/pkg/pinyin"

	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/metrics"
)

// TenantRegistry 简化版：持久化的租户列表。
//
// JSON 格式（`tenant_registry.path` 指向的文件）：
//
//	[
//	  {"id": "158", "sync_token": "..."},                           // 可选，配置后台同步使用的 Bearer
//	  {"id": "1889501240003497986",
//	   "sync_token": "eyJ...",                                       // qua service token
//	   "sync_token_expires_at": "2026-12-31T23:59:59+08:00",        // RFC3339；可选，缺省 = 不过期
//	   "last_refresh_at": "2026-09-07T10:00:00+08:00"}              // 自动写入；运行时可读
//	]
//
// 行为：
//   - 启动时加载文件，填入 tenants + syncTokens + expires。
//   - 缺失字段时仅记录 ID；sync_token 为空表示该租户仅在请求路径按需同步。
//   - qua 同步时如发现新租户也会调用 Ensure 注册。
//   - sync_token 过期 / 即将过期由 TokenRefresher 负责（见 token_refresher.go）。
type TenantRegistry struct {
	mu         sync.RWMutex
	tenants    map[string]bool
	syncTokens map[string]string
	// expiresAt[tenantID] = zero 表示不过期（缺省行为，兼容旧配置）
	expiresAt map[string]time.Time
	// lastRefreshAt[tenantID] 由 TokenRefresher 自动写入
	lastRefreshAt map[string]time.Time
}

// tenantRegistryEntry tenant_registry.json 中的单条记录。
type tenantRegistryEntry struct {
	ID                string    `json:"id"`
	SyncToken         string    `json:"sync_token,omitempty"`
	SyncTokenExpires  string    `json:"sync_token_expires_at,omitempty"` // RFC3339
	LastRefreshAt     string    `json:"last_refresh_at,omitempty"`        // RFC3339（只读，refresher 写入）
}

// NewTenantRegistry 从 conf 构造（启动时读 tenant_registry.path 文件）。
func NewTenantRegistry(c *conf.TenantRegistry) *TenantRegistry {
	r := &TenantRegistry{
		tenants:       make(map[string]bool),
		syncTokens:    make(map[string]string),
		expiresAt:     make(map[string]time.Time),
		lastRefreshAt: make(map[string]time.Time),
	}
	if c == nil || c.Path == "" {
		return r
	}
	if err := r.loadFromFile(c.Path); err != nil {
		// 加载失败不阻断启动，仅记入 stderr，后续请求按需同步。
		fmt.Fprintf(os.Stderr, "[tenant_registry] load %s: %v (fallback to empty)\n", c.Path, err)
	}
	return r
}

// loadFromFile 读取 tenant_registry.json，填入注册表。
//
// 文件不存在或格式异常返回错误，调用方决定是否告警。
func (r *TenantRegistry) loadFromFile(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	var entries []tenantRegistryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range entries {
		if e.ID == "" {
			continue
		}
		r.tenants[e.ID] = true
		if e.SyncToken != "" {
			r.syncTokens[e.ID] = e.SyncToken
		}
		if e.SyncTokenExpires != "" {
			if t, err := time.Parse(time.RFC3339, e.SyncTokenExpires); err == nil {
				r.expiresAt[e.ID] = t
			} else {
				fmt.Fprintf(os.Stderr, "[tenant_registry] %s: invalid expires_at %q: %v\n",
					e.ID, e.SyncTokenExpires, err)
			}
		}
		if e.LastRefreshAt != "" {
			if t, err := time.Parse(time.RFC3339, e.LastRefreshAt); err == nil {
				r.lastRefreshAt[e.ID] = t
			}
		}
	}
	return nil
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

// GetSyncToken 返回指定租户的后台同步 Bearer（过期仍返回，由调用方决定）。
func (r *TenantRegistry) GetSyncToken(tenantID string) string {
	if tenantID == "" {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.syncTokens[tenantID]
}

// SyncTokenExpiresAt 返回过期时间；zero 表示未配置 / 不过期。
func (r *TenantRegistry) SyncTokenExpiresAt(tenantID string) time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.expiresAt[tenantID]
}

// HasSyncTokens 是否存在至少一个配置了 sync_token 的租户。
func (r *TenantRegistry) HasSyncTokens() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.syncTokens) > 0
}

// SetSyncToken 动态设置（用于测试 / 运行时注入）。
func (r *TenantRegistry) SetSyncToken(tenantID, token string) {
	if tenantID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[tenantID] = true
	if token == "" {
		delete(r.syncTokens, tenantID)
		delete(r.expiresAt, tenantID)
		delete(r.lastRefreshAt, tenantID)
		return
	}
	r.syncTokens[tenantID] = token
}

// SetSyncTokenWithExpiry 设置 token + 过期时间（refresher 调用）。
func (r *TenantRegistry) SetSyncTokenWithExpiry(tenantID, token string, expiresAt time.Time) {
	if tenantID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[tenantID] = true
	r.syncTokens[tenantID] = token
	r.expiresAt[tenantID] = expiresAt
	r.lastRefreshAt[tenantID] = time.Now()
}

// LastRefreshAt 返回该租户最近一次 refresh 时间（zero = 从未刷新）。
func (r *TenantRegistry) LastRefreshAt(tenantID string) time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastRefreshAt[tenantID]
}

// ExpiringTenants 返回 expiresAt - now < threshold 的 tenantID 列表（即将过期）。
func (r *TenantRegistry) ExpiringTenants(threshold time.Duration) []string {
	now := time.Now()
	var out []string
	r.mu.RLock()
	defer r.mu.RUnlock()
	for tid, exp := range r.expiresAt {
		if exp.IsZero() {
			continue // 未配置过期 = 不过期
		}
		if exp.Sub(now) < threshold {
			out = append(out, tid)
		}
	}
	return out
}

// ExpiredTenants 返回已过期（exp < now）的 tenantID 列表。
func (r *TenantRegistry) ExpiredTenants() []string {
	now := time.Now()
	var out []string
	r.mu.RLock()
	defer r.mu.RUnlock()
	for tid, exp := range r.expiresAt {
		if exp.IsZero() {
			continue
		}
		if exp.Before(now) {
			out = append(out, tid)
		}
	}
	return out
}

// Snapshot 返回不可变快照（供健康检查 / refresher 读）。
type TenantSnapshot struct {
	ID                string
	HasToken          bool
	ExpiresAt         time.Time
	LastRefreshAt     time.Time
	IsExpired         bool
	IsExpiringSoon    bool // 1h 内
}

// SnapshotAll 返回所有租户状态快照。
func (r *TenantRegistry) SnapshotAll() []TenantSnapshot {
	now := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TenantSnapshot, 0, len(r.tenants))
	for tid := range r.tenants {
		exp, hasExp := r.expiresAt[tid]
		snap := TenantSnapshot{
			ID:            tid,
			HasToken:      r.syncTokens[tid] != "",
			ExpiresAt:     exp,
			LastRefreshAt: r.lastRefreshAt[tid],
		}
		if hasExp && !exp.IsZero() {
			snap.IsExpired = exp.Before(now)
			snap.IsExpiringSoon = !snap.IsExpired && exp.Sub(now) < time.Hour
		}
		out = append(out, snap)
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

	// healthChecker 可选：用于上报同步模式（lazy_only / background）。
	healthChecker HealthNotifier
}

// HealthNotifier 抽象 health 同步模式上报（避免 biz 直接依赖 pkg/health 接口类型）。
type HealthNotifier interface {
	SetSyncMode(mode string)
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

// WithHealthNotifier 注入 health checker 用于报告 sync 模式。
func WithHealthNotifier(n HealthNotifier) SyncerOption {
	return func(s *VocabSyncer) {
		if n != nil {
			s.healthChecker = n
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
	health HealthNotifier,
) *VocabSyncer {
	s := NewVocabSyncer(registry, vocab, normalizer, quaSource, c, logger,
		WithCanQuaFetch(canQuaFetch),
		WithHealthNotifier(health),
	)
	// 装上 cache miss 回调（不阻塞当前请求，goroutine 异步同步）。
	s.AttachLazySync(vocab)
	// 初次上报同步模式。
	if health != nil {
		health.SetSyncMode(s.SyncMode())
	}
	return s
}

// Warmup 启动期全量预热（拉一次 qua + 对已注册 tenant 同步）。
//
// 不阻塞主流程太久：设 30s timeout。
//
// 注意：qua 调用需要 ctx 里携带 AuthInfo（用户 token），启动期如果没有
// 共享 service account，会跳过全量预热、改为请求级按需同步。
// Warmup 启动期预热：
//
//  1. 优先从 tenant_registry 读取预配置租户（含可选 sync_token）；
//  2. 对有 sync_token 的租户立即同步一次（用其 token 作为认证上下文）。
//  3. 未配置 sync_token 的租户只能靠请求路径 lazy 同步，warmup 不会拉。
func (s *VocabSyncer) Warmup(ctx context.Context) {
	tenants := s.registry.List()
	if len(tenants) == 0 {
		s.log.Info("vocab warmup skipped: empty tenant registry (will sync per-request)")
		return
	}

	warmupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	synced := 0
	for _, t := range tenants {
		token := s.registry.GetSyncToken(t)
		if token == "" {
			s.log.Debugf("warmup skip tenant %s: no sync_token (lazy only)", t)
			continue
		}
		syncCtx := s.syncCtxFor(warmupCtx, t, token)
		if err := s.SyncTenant(syncCtx, t); err != nil {
			s.log.Warnf("warmup tenant %s: %v", t, err)
			continue
		}
		synced++
	}
	s.log.Infof("vocab warmup: %d/%d tenants synced (with sync_token)", synced, len(tenants))
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

// runOnce 单次后台同步：遍历 registry，仅同步配置了 sync_token 的租户。
//
// 语义说明：
//   - 每个租户独立构造带有其 token 的 ctx，避免 token 透传错误；
//   - 同步失败仅记 warn，不中断其他租户；
//   - 每次同步向 metrics.VocabSyncTotal 上报 mode + status。
func (s *VocabSyncer) runOnce(ctx context.Context) {
	tenants := s.registry.List()
	if len(tenants) == 0 {
		return
	}
	mode := s.SyncMode()
	for _, t := range tenants {
		token := s.registry.GetSyncToken(t)
		if token == "" {
			// 无后台 token：仅靠请求路径同步。
			continue
		}
		syncCtx := s.syncCtxFor(ctx, t, token)
		if err := s.SyncTenant(syncCtx, t); err != nil {
			metrics.VocabSyncTotal.Inc(mode, "error")
			s.log.Warnf("sync tenant %s: %v", t, err)
			continue
		}
		metrics.VocabSyncTotal.Inc(mode, "ok")
	}
}

// SyncMode 返回当前同步模式：
//   - "background"：存在至少一个 sync_token，后台周期同步生效；
//   - "lazy_only"：未配置 sync_token，仅在请求路径同步。
func (s *VocabSyncer) SyncMode() string {
	if s.registry.HasSyncTokens() {
		return "background"
	}
	return "lazy_only"
}

// SyncTenant 同步单 tenant（qua raw → Normalizer → entries/relations → UpdateTenant）。
//
// 返回 error 仅用于 logging；调用方不需处理。
//
// 设计说明（M5 → M9 修正）：quaSource 拉到的所有 RawEntity 都属于同一租户（qua 端
// 按 tenant-id 头隔离数据），所以无需再过滤；所有 raws 直接进入 Normalizer。
//
// Token 来源（优先级）：
//   1. ctx.AuthContext.AccessToken（请求路径，前端调用方 token）
//   2. registry.GetSyncToken(tenantID)（后台路径，配置在 tenants.json 的 service token）
//   3. 都没有 → 跳过后台同步（仅依赖 lazy 请求路径）
func (s *VocabSyncer) SyncTenant(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return nil
	}

	// 若 ctx 没有 auth，注入 tenants.json 里的 sync_token（后台同步路径）
	if _, hasAuth := AuthFrom(ctx); !hasAuth {
		if token := s.registry.GetSyncToken(tenantID); token != "" {
			ctx = s.syncCtxFor(ctx, tenantID, token)
		}
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

// syncAuth 为后台同步提供 per-tenant 认证上下文。
//
// 实现 biz.AuthContext 接口；quaSource 的 TokenProvider / TenantIDProvider 从
// ctx 中提取 token / tenant-id 才能路由到正确的上游租户。
type syncAuth struct {
	token    string
	tenantID string
}

// GetAccessToken 实现 biz.AuthContext。
func (a *syncAuth) GetAccessToken() string { return a.token }

// GetTenantID 实现 biz.AuthContext。
func (a *syncAuth) GetTenantID() string { return a.tenantID }

// syncCtxFor 为指定租户构造带有 sync token 的 ctx。
//
// 避免多个 goroutine 共享同一 ctx，每个租户独立 clone。
func (s *VocabSyncer) syncCtxFor(parent context.Context, tenantID, token string) context.Context {
	if token == "" {
		return parent
	}
	return WithAuth(parent, &syncAuth{token: token, tenantID: tenantID})
}
func (s *VocabSyncer) convertRawToVocab(raws []RawEntity) ([]*VocabularyEntry, []*VocabularyRelation, error) {
	normalized, err := s.normalizer.NormalizeBatch(raws)
	if err != nil {
		return nil, nil, err
	}

	entries := make([]*VocabularyEntry, 0, len(normalized))
	relations := make([]*VocabularyRelation, 0)
	var nextID uint32 = 1
	entryIDByText := make(map[string]uint32, len(normalized))

	for _, n := range normalized {
		e := &VocabularyEntry{
			ID:            nextID,
			StandardText:  n.StandardText,
			Category:      n.Category,
			EntryType:     "WORD",
			Priority:      int(n.Priority), // 修复 P5：Normalizer 输出的 Priority 必须传到 VocabularyEntry
			Pinyin:        derivePinyin(n.StandardText),
			PinyinInitial: derivePinyinInitial(n.StandardText),
		}
		entries = append(entries, e)
		entryIDByText[n.StandardText] = nextID
		nextID++

		// ALIAS 关系
		for _, a := range n.Aliases {
			relations = append(relations, &VocabularyRelation{
				EntryID:       e.ID,
				RelationType:  "ALIAS",
				RelatedText:   a,
				TargetEntryID: e.ID,
			})
		}
	}

	return entries, relations, nil
}

// 全局拼音转换器（无状态、可并发使用）。
var pinyinConv = pkgpinyin.NewConverter()

// derivePinyin 派生标准词的全拼（空格分隔每个汉字）。
// 输入为空或转换失败时返回空串。
func derivePinyin(s string) string {
	if s == "" {
		return ""
	}
	res, err := pinyinConv.Convert(s, false)
	if err != nil || res == nil {
		return ""
	}
	return res.Pinyin
}

// derivePinyinInitial 派生标准词的首字母串（无分隔）。
// 输入为空或转换失败时返回空串。
func derivePinyinInitial(s string) string {
	if s == "" {
		return ""
	}
	res, err := pinyinConv.Convert(s, true)
	if err != nil || res == nil {
		return ""
	}
	return res.PinyinInitial
}

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
