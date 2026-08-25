package biz

import (
	"context"
	"sort"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
)

// VocabularyEntry 上下文中的词条（语言概念）。
type VocabularyEntry struct {
	ID            uint32
	StandardText  string
	Category      string
	EntryType     string
	Priority      int
	Pinyin        string
	PinyinInitial string
}

// VocabularyRelation 上下文中的词条关系。
type VocabularyRelation struct {
	EntryID       uint32
	RelationType  string // ALIAS/CORRECTION/HOMOPHONE/...
	RelatedText   string
	TargetEntryID uint32
}

// VocabularyRelationData 关系 + 所属作用域（供冲突检测）。
type VocabularyRelationData struct {
	EntryID       uint32
	RelationType  string
	RelatedText   string
	TargetEntryID uint32
	Scope         string // PLATFORM/SYSTEM/TENANT
}

// VocabularyContext 运行时词库上下文（通用语言上下文）。
// 由 VocabularyBuilder 从 Platform/System/Tenant 词库构建，服务文本增强引擎与
// 未来的 ASR Hotword Builder / NER（演进预留 4.1）。
type VocabularyContext struct {
	Version   string
	entries   map[string]*VocabularyEntry      // standard_text → entry
	relations map[string][]*VocabularyRelation // related_text → relations
}

// LookupEntry 精确匹配标准词。
func (c *VocabularyContext) LookupEntry(text string) (*VocabularyEntry, bool) {
	e, ok := c.entries[text]
	return e, ok
}

// LookupRelations 按关联表达查找关系（别名/纠错/同音等）。
func (c *VocabularyContext) LookupRelations(text string) []*VocabularyRelation {
	return c.relations[text]
}

// EntryCount 返回词条数量。
func (c *VocabularyContext) EntryCount() int { return len(c.entries) }

// RelationCount 返回关系数量。
func (c *VocabularyContext) RelationCount() int { return len(c.relations) }

// DictionaryConflictRecorder 词库冲突记录接口。
type DictionaryConflictRecorder interface {
	RecordConflict(ctx context.Context, conflict *DictionaryConflict) error
	ListConflicts(ctx context.Context) ([]*DictionaryConflict, error)
}

// DictionaryConflict 词库冲突（业务模型）。
type DictionaryConflict struct {
	Input             string
	Candidate         string
	SourceScope       string
	SourceDictionary  string
	Priority          int32
	ResolvedCandidate string
}

// scopePriority 作用域优先级：TENANT > SYSTEM > PLATFORM。
func scopePriority(scope string) int {
	switch scope {
	case "TENANT":
		return 3
	case "SYSTEM":
		return 2
	case "PLATFORM":
		return 1
	default:
		return 0
	}
}

// VocabularyBuilder 构建 VocabularyContext，按 TENANT > SYSTEM > PLATFORM 优先级合并。
type VocabularyBuilder struct {
	repo    DictionaryRepo
	confRec DictionaryConflictRecorder
	cache   *sync.Map // key: tenantID → *VocabularyContext
	log     *log.Helper
}

// NewVocabularyBuilder 创建词库上下文构建器。
func NewVocabularyBuilder(repo DictionaryRepo, confRec DictionaryConflictRecorder, logger log.Logger) *VocabularyBuilder {
	return &VocabularyBuilder{repo: repo, confRec: confRec, cache: &sync.Map{}, log: log.NewHelper(logger)}
}

// Build 构建租户的词汇上下文（Platform + System + Tenant 合并），结果按租户缓存。
// 词库发布新版本后调用 Invalidate 失效，保证单次请求内一致性。
func (b *VocabularyBuilder) Build(ctx context.Context, tenantID uint32) (*VocabularyContext, error) {
	if v, ok := b.cache.Load(tenantID); ok {
		return v.(*VocabularyContext), nil
	}
	vc, err := b.build(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	b.cache.Store(tenantID, vc)
	return vc, nil
}

// Invalidate 失效指定租户的上下文缓存（词库发布新版本后调用）。
func (b *VocabularyBuilder) Invalidate(tenantID uint32) {
	b.cache.Delete(tenantID)
}

// build 从三个作用域加载词条+关系并合并，冲突按优先级解析并记录。
func (b *VocabularyBuilder) build(ctx context.Context, tenantID uint32) (*VocabularyContext, error) {
	entries, relations, err := b.repo.LoadVocabularyEntries(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	vc := &VocabularyContext{
		Version:   "v1",
		entries:   make(map[string]*VocabularyEntry, len(entries)),
		relations: make(map[string][]*VocabularyRelation, len(relations)),
	}

	// 词条索引（同一 standard_text 按 priority 取最高者）
	for _, e := range entries {
		existing, ok := vc.entries[e.GetStandardText()]
		if !ok || int(e.GetPriority()) > existing.Priority {
			vc.entries[e.GetStandardText()] = &VocabularyEntry{
				ID:            e.GetId(),
				StandardText:  e.GetStandardText(),
				Category:      e.GetCategory(),
				EntryType:     e.GetEntryType(),
				Priority:      int(e.GetPriority()),
				Pinyin:        e.GetPinyin(),
				PinyinInitial: e.GetPinyinInitial(),
			}
		}
	}

	// 冲突检测 + 关系索引（同一 related_text 多 scope 冲突按优先级解析）
	conflicts := b.detectConflicts(relations)
	for _, c := range conflicts {
		if b.confRec != nil {
			_ = b.confRec.RecordConflict(ctx, c)
		}
	}
	for _, rel := range relations {
		vc.relations[rel.RelatedText] = append(vc.relations[rel.RelatedText], &VocabularyRelation{
			EntryID:       rel.EntryID,
			RelationType:  rel.RelationType,
			RelatedText:   rel.RelatedText,
			TargetEntryID: rel.TargetEntryID,
		})
	}

	return vc, nil
}

// detectConflicts 检测同一 related_text 在不同作用域的候选冲突。
// 同一 related_text 出现在多个 scope 且 target 不同即视为冲突；按 scope 优先级解析
// （TENANT > SYSTEM > PLATFORM），低优先级的候选记入冲突记录。
func (b *VocabularyBuilder) detectConflicts(relations []VocabularyRelationData) []*DictionaryConflict {
	// 按 related_text 分组
	grouped := make(map[string][]VocabularyRelationData)
	for _, r := range relations {
		grouped[r.RelatedText] = append(grouped[r.RelatedText], r)
	}

	var conflicts []*DictionaryConflict
	for text, rs := range grouped {
		if len(rs) < 2 {
			continue
		}
		// 排序：高优先级在前
		sort.SliceStable(rs, func(i, j int) bool {
			return scopePriority(rs[i].Scope) > scopePriority(rs[j].Scope)
		})
		resolved := rs[0]
		for _, r := range rs[1:] {
			// 低优先级候选与解析结果不同 → 冲突
			if r.TargetEntryID != resolved.TargetEntryID || r.EntryID != resolved.EntryID {
				conflicts = append(conflicts, &DictionaryConflict{
					Input:             text,
					Candidate:         candidateOf(r),
					SourceScope:       r.Scope,
					SourceDictionary:  r.Scope,
					Priority:          int32(scopePriority(r.Scope)),
					ResolvedCandidate: candidateOf(resolved),
				})
			}
		}
	}
	return conflicts
}

// candidateOf 冲突候选展示（target entry id 或 relation 归属）。
func candidateOf(r VocabularyRelationData) string {
	if r.TargetEntryID != 0 {
		return r.RelatedText
	}
	return r.RelatedText
}
