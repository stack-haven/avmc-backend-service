// Package processors · snapshot.go
// VocabularySnapshot：不可变词汇快照（per-request 安全共享）。
//
// 设计要点：
//   1. 构造后不可变；Process 只读，不写
//   2. 由各服务的 VocabularyBuilder.Build(ctx, tenantID) 返回
//   3. 跨租户不可共享（每个租户一份独立快照）
//   4. 空快照合法（首次启动 / 外部源故障时使用）
package processors

// VocabularyEntry 词汇条目（语言概念）。
type VocabularyEntry struct {
	ID            uint32
	StandardText  string
	Category      string
	EntryType     string
	Priority      int
	Pinyin        string
	PinyinInitial string
}

// VocabularyRelation 词汇关系（别名/纠错/同音等）。
type VocabularyRelation struct {
	EntryID       uint32
	RelationType  string // ALIAS / CORRECTION / HOMOPHONE / ...
	RelatedText   string
	TargetEntryID uint32
}

// VocabularySnapshot 不可变快照（per-request）。
type VocabularySnapshot struct {
	Version      string
	Entries      map[string]*VocabularyEntry      // standard_text → entry
	Relations    map[string][]*VocabularyRelation // related_text → relations
	lookupEntry  map[string]*VocabularyEntry      // alias for O(1) query
}

// NewVocabularySnapshot 构造快照（自动建索引）。
func NewVocabularySnapshot(entries []*VocabularyEntry, relations []*VocabularyRelation) *VocabularySnapshot {
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
	return &VocabularySnapshot{
		Version:     "v1",
		Entries:     es,
		Relations:   rs,
		lookupEntry: es,
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