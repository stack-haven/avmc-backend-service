// Package biz · vocab_sync.go
// VocabSyncer：纯 lazy-load + TTL cache（v1.6 — 减法后）。
//
// 设计（用户需求）：
//  1. 第一次带 token 访问时 → 同步调 qua 拉 PERSON/DEPT → 写入 VocabularyBuilder
//  2. 后续高频访问 → 直接读 VocabularyBuilder 快照（O(1)）
//  3. TTL 过期（默认 5min）→ 下次访问时重新拉取
//
// 完全剔除（v1.6）：
//   - 后台 Warmup / Run ticker
//   - admin_token 配置
//   - tenants.json 加载
//   - TenantRegistry 抽象
//   - sync_token 过期管理
//
// 依赖：ctx 中的 AuthContext（middleware 注入）+ qua.VocabularySource（HTTP 拉数据）
package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pkgpinyin "backend-service/pkg/pinyin"
)

// VocabSyncer 把 qua raw entities 转换为 vocab entries 并写入 VocabularyBuilder。
//
// 不再持有 TenantRegistry / adminToken / interval：所有租户数据通过 ctx.AuthContext
// 动态获取，TTL 过期由 VocabularyBuilder 内部判断。
type VocabSyncer struct {
	vocab      *VocabularyBuilder
	normalizer *Normalizer
	quaSource  VocabularySource
	log        *log.Helper
}

// NewVocabSyncer 构造 syncer。
func NewVocabSyncer(
	vocab *VocabularyBuilder,
	normalizer *Normalizer,
	quaSource VocabularySource,
	logger log.Logger,
) *VocabSyncer {
	return &VocabSyncer{
		vocab:      vocab,
		normalizer: normalizer,
		quaSource:  quaSource,
		log:        log.NewHelper(log.With(logger, "module", "biz/vocab_sync")),
	}
}

// NewVocabSyncerWithAuth wire 注入入口。
//
// 设计：ctx.AuthContext 提供 tenant-id + access-token，请求路径走 lazy sync。
// 无 token 的 ctx 直接调 qua 会 401，由 qua.VocabularySource 内部处理（依赖
// token + tenant-id 头透传到上游 qua 系统）。
func NewVocabSyncerWithAuth(
	vocab *VocabularyBuilder,
	normalizer *Normalizer,
	quaSource VocabularySource,
	logger log.Logger,
) *VocabSyncer {
	s := NewVocabSyncer(vocab, normalizer, quaSource, logger)
	// 装上 cache miss 回调（不阻塞当前请求，goroutine 异步同步）。
	s.AttachLazySync(vocab)
	return s
}

// SyncTenant 同步单 tenant（qua raw → Normalizer → entries/relations → UpdateTenant）。
//
// 触发场景：cache miss / TTL 过期 / 显式预热（无 Warmup 函数了）。
//
// ctx 必须包含 AuthContext（middleware 注入），否则 qua 调用会因缺 token 失败。
//
// 设计说明（M5 → M9 修正）：quaSource 拉到的所有 RawEntity 都属于同一租户（qua 端
// 按 tenant-id 头隔离数据），所以无需再过滤；所有 raws 直接进入 Normalizer。
func (s *VocabSyncer) SyncTenant(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return nil
	}

	// 校验 ctx 是否有 AuthContext（无则 qua 必 401）
	if _, hasAuth := AuthFrom(ctx); !hasAuth {
		return fmt.Errorf("sync tenant %s: missing AuthContext in ctx", tenantID)
	}

	// 拉 raw（partial-failure 容忍：user/dept 任一失败仍用已拿到的数据）
	allRaws, err := s.quaSource.Fetch(ctx)
	if err != nil {
		s.log.Warnf("fetch partial error（接受已拉到的实体）: %v", err)
	}
	if len(allRaws) == 0 {
		return fmt.Errorf("fetch: no data (and error: %w)", err)
	}

	// Normalizer 转换（所有 raws 属于同一 tenant，无需再过滤）
	entries, rels, err := s.convertRawToVocab(allRaws)
	if err != nil {
		return fmt.Errorf("normalize: %w", err)
	}

	// 更新 builder（同时设置 ExpiresAt = now + TTL）
	s.vocab.UpdateTenant(tenantID, entries, rels)
	s.log.Infof("synced tenant %s: %d entries, %d relations", tenantID, len(entries), len(rels))
	return nil
}

// EnsureTenant 保证某 tenant 的 vocab snapshot 是新鲜的。
//
// 触发条件：
//  1. cache miss（该 tenant 从未同步过）→ 同步
//  2. cache stale（snapshot.ExpiresAt 过期）→ 同步
//  3. 已有且未过期 → 跳过
//
// 调用 ctx 用于调 qua，需包含 AuthContext。
// 同步在调用 goroutine 中执行（与请求同步），保证下次访问是新鲜数据。
func (s *VocabSyncer) EnsureTenant(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return nil
	}
	if s.vocab.HasFreshTenant(tenantID) {
		return nil // 未过期，跳过
	}
	if err := s.SyncTenant(ctx, tenantID); err != nil {
		s.log.Warnf("lazy sync tenant %s: %v (continue serving stale snapshot)", tenantID, err)
		return err
	}
	s.log.Infof("lazy synced tenant %s", tenantID)
	return nil
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

// convertRawToVocab 把 qua raw entities 转换为 vocab entries + relations。
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
