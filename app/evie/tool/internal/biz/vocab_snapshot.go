// Package biz · vocab_snapshot.go
// 词库快照类型（business-level，从 pkg/textenhance/processors 迁移而来）。
//
// M9.6 重构：原 pkg/textenhance/processors.VocabularySnapshot 等价物。
// 设计原因：
//   - 词库快照是 evie/tool 业务数据模型（per-tenant），不是通用文本增强概念
//   - 把它放在 biz 包，与 lexnorm 解耦（lexnorm 用自己的 Lexicon 类型）
//   - lexnorm_bridge.go 在 biz 内部把 snapshot → lexnorm.Lexicon
//
// v1.2 新增 TTL：快照带 ExpiresAt，VocabularyBuilder.HasFreshTenant 用 TTL 决定是否重拉。
// 设计理由：防止长期缓存导致数据陈旧；同时避免实时拉取（性能）。
package biz

import "time"

// VocabularyEntry 词汇条目。
type VocabularyEntry struct {
	ID            uint32
	StandardText  string
	Category      string
	EntryType     string
	Priority      int
	Pinyin        string
	PinyinInitial string
	// LockAlias=true 时，fuzzy/deterministic processor 不替换此 entry 的 standard_text
	//（业务产品功能名 / 已知专有名词保护，避免被相似词误改）。
	LockAlias bool
}

// VocabularyRelation 词汇关系（别名/纠错/同音等）。
type VocabularyRelation struct {
	EntryID       uint32
	RelationType  string // ALIAS / CORRECTION / HOMOPHONE / ...
	RelatedText   string
	TargetEntryID uint32
}

// VocabularySnapshot 不可变快照（per-request）。
//
// v1.2 新增 TTL：ExpiresAt 为零表示永不过期（如静态 source）。
type VocabularySnapshot struct {
	Version     string
	Entries     map[string]*VocabularyEntry      // standard_text → entry
	Relations   map[string][]*VocabularyRelation // related_text → relations
	lookupEntry map[string]*VocabularyEntry      // alias for O(1) query
	// ExpiresAt v1.2 快照过期时间；zero = 永不过期
	ExpiresAt time.Time
}

// NewVocabularySnapshot 构造永不过期的快照（兼容 v1.1 调用方）。
func NewVocabularySnapshot(entries []*VocabularyEntry, relations []*VocabularyRelation) *VocabularySnapshot {
	return NewVocabularySnapshotWithTTL(entries, relations, 0)
}

// NewVocabularySnapshotWithTTL v1.2 构造带 TTL 的快照。
//
// ttl <= 0 表示永不过期（适用静态 source / 单元测试）。
// ttl > 0 时 ExpiresAt = now + ttl。
func NewVocabularySnapshotWithTTL(entries []*VocabularyEntry, relations []*VocabularyRelation, ttl time.Duration) *VocabularySnapshot {
	es := make(map[string]*VocabularyEntry, len(entries))
	for _, e := range entries {
		if e == nil || e.StandardText == "" {
			continue
		}
		es[e.StandardText] = e
	}
	rs := make(map[string][]*VocabularyRelation, len(relations))
	for _, r := range relations {
		if r == nil || r.RelatedText == "" {
			continue
		}
		rs[r.RelatedText] = append(rs[r.RelatedText], r)
	}
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	return &VocabularySnapshot{
		Version:     "v1",
		Entries:     es,
		Relations:   rs,
		lookupEntry: es,
		ExpiresAt:   expiresAt,
	}
}

// EmptyVocabularySnapshot 返回空快照（合法状态）。
func EmptyVocabularySnapshot() *VocabularySnapshot {
	return &VocabularySnapshot{
		Version:     "v0-empty",
		Entries:     map[string]*VocabularyEntry{},
		Relations:   map[string][]*VocabularyRelation{},
		lookupEntry: map[string]*VocabularyEntry{},
	}
}

// IsExpired v1.2 判断快照是否过期（ExpiresAt 已设且 < now）。
//
// 零值 ExpiresAt 永不过期（适用静态 snapshot）。
func (s *VocabularySnapshot) IsExpired(now time.Time) bool {
	if s == nil {
		return true
	}
	if s.ExpiresAt.IsZero() {
		return false
	}
	return now.After(s.ExpiresAt)
}

// LookupEntry 精确匹配标准词。
func (s *VocabularySnapshot) LookupEntry(text string) (*VocabularyEntry, bool) {
	if s == nil {
		return nil, false
	}
	if s.lookupEntry != nil {
		e, ok := s.lookupEntry[text]
		return e, ok
	}
	e, ok := s.Entries[text]
	return e, ok
}

// LookupRelations 按关联表达查找关系（返回 slice 副本）。
func (s *VocabularySnapshot) LookupRelations(text string) []*VocabularyRelation {
	if s == nil {
		return nil
	}
	rs := s.Relations[text]
	if rs == nil {
		return nil
	}
	out := make([]*VocabularyRelation, len(rs))
	copy(out, rs)
	return out
}

// EntryCount 词条数。
func (s *VocabularySnapshot) EntryCount() int {
	if s == nil {
		return 0
	}
	return len(s.Entries)
}

// RelationCount 关系数。
func (s *VocabularySnapshot) RelationCount() int {
	if s == nil {
		return 0
	}
	return len(s.Relations)
}
