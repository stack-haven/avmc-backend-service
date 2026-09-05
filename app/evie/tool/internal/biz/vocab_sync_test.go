package biz_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/durationpb"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
)

// mockSource 模拟 VocabularySource（用于测试）。
type mockSource struct {
	entities []biz.RawEntity
	err      error
	count    int32
}

func (m *mockSource) Name() string { return "mock" }
func (m *mockSource) Fetch(ctx context.Context) ([]biz.RawEntity, error) {
	atomic.AddInt32(&m.count, 1)
	return m.entities, m.err
}

func writeSyncTestDict(t *testing.T) string {
	dir := t.TempDir()
	path := filepath.Join(dir, "system.json")
	content := `{
  "version": "test",
  "entries": [
    {"standard_text": "系统词", "category": "SYSTEM"}
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// TestVocabBuilder_UpdateTenant 测试 per-tenant 快照更新 + 合并。
func TestVocabBuilder_UpdateTenant(t *testing.T) {
	dictPath := writeSyncTestDict(t)
	vb, _ := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictPath})

	// 1. 初始无 tenant
	snap := vb.Build(context.Background(), "158")
	if snap.EntryCount() != 1 {
		t.Errorf("initial EntryCount = %d, want 1 (system only)", snap.EntryCount())
	}

	// 2. 更新 tenant 158
	tenantEntries := []*biz.VocabularyEntry{
		{ID: 100, StandardText: "田华", Category: "PERSON", Priority: 90},
		{ID: 101, StandardText: "技术研发部", Category: "ORGANIZATION", Priority: 80},
	}
	tenantRelations := []*biz.VocabularyRelation{
		{EntryID: 100, RelationType: "ALIAS", RelatedText: "小田", TargetEntryID: 100},
	}
	vb.UpdateTenant("158", tenantEntries, tenantRelations)

	// 3. 验证 tenant 158 看到新 entries
	snap2 := vb.Build(context.Background(), "158")
	if snap2.EntryCount() != 3 {
		t.Errorf("after update EntryCount = %d, want 3 (system + 2 tenant)", snap2.EntryCount())
	}
	// 检查 alias 关系
	rs := snap2.LookupRelations("小田")
	if len(rs) != 1 || rs[0].RelationType != "ALIAS" {
		t.Errorf("LookupRelations(小田) = %+v, want 1 ALIAS", rs)
	}

	// 4. 验证其他 tenant 不受影响
	snap3 := vb.Build(context.Background(), "999")
	if snap3.EntryCount() != 1 {
		t.Errorf("other tenant EntryCount = %d, want 1 (system only)", snap3.EntryCount())
	}
}

// TestVocabBuilder_PerTenantIsolation 测试 per-tenant 隔离。
func TestVocabBuilder_PerTenantIsolation(t *testing.T) {
	dictPath := writeSyncTestDict(t)
	vb, _ := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictPath})

	vb.UpdateTenant("158", []*biz.VocabularyEntry{
		{ID: 1, StandardText: "田华", Category: "PERSON"},
	}, nil)
	vb.UpdateTenant("159", []*biz.VocabularyEntry{
		{ID: 2, StandardText: "张三", Category: "PERSON"},
	}, nil)

	snap158 := vb.Build(context.Background(), "158")
	snap159 := vb.Build(context.Background(), "159")

	if _, ok := snap158.LookupEntry("田华"); !ok {
		t.Error("158 should have 田华")
	}
	if _, ok := snap158.LookupEntry("张三"); ok {
		t.Error("158 should NOT have 张三 (isolation failed)")
	}
	if _, ok := snap159.LookupEntry("张三"); !ok {
		t.Error("159 should have 张三")
	}
}

// TestVocabBuilder_ConcurrentUpdate 100 goroutine 并发写。
func TestVocabBuilder_ConcurrentUpdate(t *testing.T) {
	dictPath := writeSyncTestDict(t)
	vb, _ := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictPath})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tenantID := "tenant-" + string(rune('A'+(idx%10)))
			vb.UpdateTenant(tenantID, []*biz.VocabularyEntry{
				{ID: uint32(idx), StandardText: "name-" + string(rune('A'+(idx%10))), Category: "TEST"},
			}, nil)
		}(i)
	}
	wg.Wait()

	tenants := vb.ListTenants()
	if len(tenants) == 0 {
		t.Error("expected non-empty tenants after concurrent update")
	}
}

// TestVocabSyncer_SyncTenant 模拟 qua 拉取 → Normalizer → UpdateTenant。
func TestVocabSyncer_SyncTenant(t *testing.T) {
	dictPath := writeSyncTestDict(t)
	vb, _ := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictPath})
	registry := biz.NewTenantRegistry(&conf.TenantRegistry{Path: "tenants.json"})

	// mock qua 返回的 raw entities
	mockRaws := []biz.RawEntity{
		{
			SourceID: "u1", EntityType: "user", Source: "158",
			Data: map[string]any{
				"realName": "田华", "nickname": "小田", "status": float64(1),
			},
		},
	}
	src := &mockSource{entities: mockRaws}

	// 构造 Normalizer：rule 接受 "realName" 作为 standard_text
	normalizer := biz.NewNormalizerFromConf(&conf.VocabRules{
		Sources: map[string]*conf.VocabRules_SourceRules{
			"158": {
				Source: "158",
				EntityMappings: []*conf.VocabRules_EntityMapping{
					{
						Match: &conf.VocabRules_EntityMapping_Match{EntityType: "user"},
						Emit: &conf.VocabRules_EntityMapping_Emit{
							StandardText: "realName",
							Category:     "PERSON",
							Aliases:      []string{"nickname"},
							IncludeWhen:  "status==1",
						},
					},
				},
			},
		},
	}, log.DefaultLogger)

	// 构造 syncer（interval 1s；M5 占位）
	syncer := biz.NewVocabSyncer(
		registry, vb, normalizer, src,
		&conf.TenantVocab{SyncInterval: durationpb.New(1 * time.Second), Concurrency: 1},
		log.DefaultLogger,
	)

	// 触发 sync
	err := syncer.SyncTenant(context.Background(), "158")
	if err != nil {
		t.Fatalf("SyncTenant: %v", err)
	}

	// 验证 tenant 158 有了 user 词条
	snap := vb.Build(context.Background(), "158")
	if _, ok := snap.LookupEntry("田华"); !ok {
		t.Error("expected tenant 158 to have 田华 after sync")
	}
	// 验证 alias
	rs := snap.LookupRelations("小田")
	if len(rs) != 1 {
		t.Errorf("expected 1 alias relation for 小田, got %d", len(rs))
	}
}

// sync.Mutex 防止编译警告
var _ sync.Mutex
// TestVocabularyBuilder_MergeNoIDConflict 回归测试：merge system + tenant 时 entry ID 必须唯一
// 否则 alias_resolution.resolveTarget 会找错 entry（M9 bug 修复）。
func TestVocabularyBuilder_MergeNoIDConflict(t *testing.T) {
	// 使用真实的 system.json（含 "金种籽"/"金种子"）
	dictPath, err := filepath.Abs("../../configs/dictionaries/system.json")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(dictPath); err != nil {
		t.Skipf("system.json not found at %s: %v", dictPath, err)
	}
	vb, _ := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictPath})

	// 构造 tenant entries（ID 从 1 开始，与 system 冲突场景）
	tenantEntries := []*biz.VocabularyEntry{
		{ID: 1, StandardText: "万康盛鼎集团", Category: "ORGANIZATION", Priority: 50},
		{ID: 2, StandardText: "技术部", Category: "ORGANIZATION", Priority: 80},
	}
	tenantRelations := []*biz.VocabularyRelation{
		{EntryID: 2, RelationType: "ALIAS", RelatedText: "研发部", TargetEntryID: 2},
	}

	vb.UpdateTenant("t-1", tenantEntries, tenantRelations)

	snap := vb.Build(context.Background(), "t-1")
	if snap == nil {
		t.Fatal("nil snapshot")
	}

	// 1. 所有 entry ID 必须唯一
	seenIDs := make(map[uint32]string)
	for stdText, e := range snap.Entries {
		if prev, ok := seenIDs[e.ID]; ok {
			t.Errorf("ID conflict: entry '%s' and '%s' both have ID=%d", prev, stdText, e.ID)
		}
		seenIDs[e.ID] = stdText
	}

	// 2. 所有 relation.TargetEntryID 必须指向真实存在的 entry
	for relatedText, rels := range snap.Relations {
		for _, rel := range rels {
			if _, ok := snap.Entries[relatedText]; ok {
				// relatedText 是标准词自身（罕见但合法）
				continue
			}
			// 解析 target
			found := false
			for _, e := range snap.Entries {
				if e.ID == rel.TargetEntryID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("relation '%s' (type=%s) points to missing entry ID=%d", relatedText, rel.RelationType, rel.TargetEntryID)
			}
		}
	}

	// 3. "金种籽" 必须存在且指向 system 的别名（"金种子"）
	jinZhongZi, ok := snap.LookupEntry("金种籽")
	if !ok {
		t.Error("'金种籽' entry missing")
	}
	if jinZhongZi != nil {
		// "金种子" 是 "金种籽" 的 alias
		rels := snap.LookupRelations("金种子")
		if len(rels) == 0 {
			t.Error("'金种子' relation missing")
		} else if rels[0].TargetEntryID != jinZhongZi.ID {
			t.Errorf("'金种子' TargetEntryID = %d, want %d", rels[0].TargetEntryID, jinZhongZi.ID)
		}
	}
}

// TestVocabSyncer_SyncTenant_PartialFailure 回归：quaSource.Fetch 返回 partial data（user 失败/dept 成功）
// 时，SyncTenant 仍能写入 tenant snapshot，不能全部丢掉。
func TestVocabSyncer_SyncTenant_PartialFailure(t *testing.T) {
	dictPath := writeSyncTestDict(t)
	vb, _ := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictPath})
	registry := biz.NewTenantRegistry(&conf.TenantRegistry{Path: ""})
	registry.Ensure("t-partial")

	// mock qua source：Fetch 返回 depts + error（user 失败场景）
	mock := &partialFailureSource{
		err:    errors.New("users: mock 501"),
		depts:  []biz.RawEntity{rawDept("1904", "万康盛鼎集团")},
	}
	normalizer := biz.NewNormalizer(buildTestRules())
	syncer := biz.NewVocabSyncer(registry, vb, normalizer, mock, &conf.TenantVocab{
		SyncInterval: durationpb.New(time.Minute),
		Concurrency:  1,
	}, log.DefaultLogger)

	// sync 不应报错（partial data OK）
	if err := syncer.SyncTenant(context.Background(), "t-partial"); err != nil {
		t.Fatalf("SyncTenant with partial data: %v", err)
	}

	snap := vb.Build(context.Background(), "t-partial")
	if snap.EntryCount() < 2 {
		t.Errorf("entry count = %d, want >= 2 (system + 1 dept)", snap.EntryCount())
	}
	_, ok := snap.LookupEntry("万康盛鼎集团")
	if !ok {
		t.Error("'万康盛鼎集团' missing after partial-failure sync")
	}
}

// === helpers ===

func rawDept(id, name string) biz.RawEntity {
	return biz.RawEntity{
		SourceID:   id,
		EntityType: "department",
		Source:     "qua",
		Data:       map[string]any{"id": id, "name": name, "status": 0},
	}
}

type partialFailureSource struct {
	err   error
	depts []biz.RawEntity
}

func (p *partialFailureSource) Name() string { return "mock-partial" }
func (p *partialFailureSource) Fetch(ctx context.Context) ([]biz.RawEntity, error) {
	return p.depts, p.err
}

func buildTestRules() *biz.RuleSet {
	return &biz.RuleSet{
		Sources: map[string]*biz.SourceRules{
			"qua": {
				Source: "qua",
				EntityMappings: []biz.EntityMapping{
					{
						Match: biz.MatchCondition{EntityType: "department"},
						Emit: biz.EmitSpec{
							StandardText: "name",
							Category:     "ORGANIZATION",
						},
					},
				},
			},
		},
	}
}
