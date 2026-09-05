// dump_vocab.go
//
// 独立查证工具：直接复现 "qua HTTP → Normalizer → VocabularyBuilder" 链路，
// 把每个节点的产物 dump 出来，让人能查证"qua users / depts 是否真进了词库"。
//
// 用法：
//   go run ./testdata/dump_vocab/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/data"
)

const (
	testToken  = "6dcd5e06b0284b3eb572322c5ac71e50"
	testTenant = "1889501240003497986"
)

func main() {
	hr := strings.Repeat("═", 70)
	fmt.Println(hr)
	fmt.Println(" evie/tool 词库链路查证（qua → Normalizer → VocabularyBuilder）")
	fmt.Println(hr)

	// ───────────────────────────────────────────────────────────
	// [1] qua HTTP 接口（独立 HTTP 调用）
	// ───────────────────────────────────────────────────────────
	fmt.Println("\n[1] qua HTTP 接口（独立 HTTP 调用）")
	users := quaHTTP("/admin-api/qua/member-extended/page?selectAll=true")
	depts := quaHTTP("/admin-api/system/dept/list")
	fmt.Printf("    users returned: %d\n", len(users))
	fmt.Printf("    depts returned: %d\n", len(depts))

	// ───────────────────────────────────────────────────────────
	// [2] qua raw → Normalizer（NormalizeBatch）
	// ───────────────────────────────────────────────────────────
	fmt.Println("\n[2] Normalizer.NormalizeBatch")
	bc := mkBootstrap()
	normalizer := biz.NewNormalizerFromConf(bc.VocabRules, log.DefaultLogger)
	quaSource := data.NewQuaVocabularySource(&mockQua{users: users, depts: depts})
	raws, _ := quaSource.Fetch(context.Background())
	fmt.Printf("    raw entities: %d (users + depts)\n", len(raws))
	normalized, nerr := normalizer.NormalizeBatch(raws)
	fmt.Printf("    normalized: %d entries / err=%v\n", len(normalized), nerr)
	dumpNormalizedSample(normalized)

	// ───────────────────────────────────────────────────────────
	// [3] VocabularyBuilder 接收 sync 写入
	// ───────────────────────────────────────────────────────────
	fmt.Println("\n[3] VocabularyBuilder.UpdateTenant")
	builder, err := biz.NewVocabularyBuilder(bc.SystemDict)
	if err != nil {
		die("NewVocabularyBuilder: %v", err)
	}
	registry := biz.NewTenantRegistry(bc.TenantRegistry)
	syncer := biz.NewVocabSyncer(registry, builder, normalizer, quaSource, bc.TenantVocab, log.DefaultLogger)
	if err := syncer.SyncTenant(context.Background(), testTenant); err != nil {
		die("sync: %v", err)
	}

	// ───────────────────────────────────────────────────────────
	// [4] VocabularyBuilder.Build 拿 snapshot
	// ───────────────────────────────────────────────────────────
	fmt.Println("\n[4] VocabularySnapshot")
	snap := builder.Build(context.Background(), testTenant)
	fmt.Printf("    entries   = %d\n", snap.EntryCount())
	fmt.Printf("    relations = %d\n", snap.RelationCount())

	// ───────────────────────────────────────────────────────────
	// [5] entries by category
	// ───────────────────────────────────────────────────────────
	fmt.Println("\n[5] entries by category")
	dumpByCategory(snap)

	// ───────────────────────────────────────────────────────────
	// [6] 关键人名验证
	// ───────────────────────────────────────────────────────────
	fmt.Println("\n[6] key name verify (✓=在词库 / ✗=不在)")
	verifyNames(snap, "佘丽群", "周丽群", "测试1", "测试播",
		"陈欣静", "伍西辉", "陈科航", "田华", "田清", "田花",
		"金种籽", "金种子")

	// ───────────────────────────────────────────────────────────
	// [7] SystemInfo / TenantInfo
	// ───────────────────────────────────────────────────────────
	fmt.Println("\n[7] builder 信息")
	si := builder.SystemInfo()
	fmt.Printf("    SystemInfo: entries=%d relations=%d loaded=%v\n",
		si.SystemEntries, si.SystemRelations, si.LoadedAt.Format("15:04:05"))
	ti := builder.GetTenantInfo(testTenant)
	fmt.Printf("    TenantInfo: entries=%d relations=%d lastSync=%v\n",
		ti.EntryCount, ti.RelationCount, ti.LastSyncAt.Format("15:04:05"))

	fmt.Println("\n" + hr)
	fmt.Println(" ✓ 查证完成")
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

func dumpNormalizedSample(ns []*biz.NormalizedEntry) {
	// 抽 5 条 user + 5 条 dept + 1 条 PHRASE
	users := []*biz.NormalizedEntry{}
	depts := []*biz.NormalizedEntry{}
	for _, n := range ns {
		switch n.Category {
		case "PERSON":
			users = append(users, n)
		case "ORGANIZATION":
			depts = append(depts, n)
		}
	}
	fmt.Printf("    sample PERSON (5/%d):\n", len(users))
	for i, e := range users {
		if i >= 5 {
			break
		}
		fmt.Printf("      [%d] %s  (alias=%v  pinyin=%s  src=%s id=%s prio=%d)\n",
			i+1, e.StandardText, e.Aliases, e.PinyinHint, e.Source, e.SourceID, e.Priority)
	}
	fmt.Printf("    sample ORGANIZATION (5/%d):\n", len(depts))
	for i, e := range depts {
		if i >= 5 {
			break
		}
		fmt.Printf("      [%d] %s  (alias=%v  src=%s id=%s prio=%d)\n",
			i+1, e.StandardText, e.Aliases, e.Source, e.SourceID, e.Priority)
	}
}

func dumpByCategory(snap *biz.VocabularySnapshot) {
	byCat := map[string]int{}
	for _, e := range snap.Entries {
		if e == nil {
			continue
		}
		cat := e.Category
		if cat == "" {
			cat = "(none)"
		}
		byCat[cat]++
	}
	keys := []string{}
	for k := range byCat {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("    %-15s = %d\n", k, byCat[k])
	}
}

func verifyNames(snap *biz.VocabularySnapshot, names ...string) {
	idx := map[string]biz.VocabularyEntry{}
	for _, e := range snap.Entries {
		if e == nil {
			continue
		}
		idx[e.StandardText] = *e
	}
	for _, n := range names {
		if e, ok := idx[n]; ok {
			fmt.Printf("    ✓ %s  (id=%d, cat=%s, prio=%d)\n",
				n, e.ID, e.Category, e.Priority)
		} else {
			fmt.Printf("    ✗ %s  (不在词库)\n", n)
		}
	}
}

func quaHTTP(path string) []map[string]any {
	req, _ := http.NewRequest("GET", "http://api.bdksim-pro.test.bedoke.com"+path, nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("tenant-id", testTenant)
	req.Header.Set("zone", "Asia/Shanghai")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  qua HTTP %s error: %v\n", path, err)
		return nil
	}
	defer resp.Body.Close()
	var base map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&base); err != nil {
		fmt.Printf("  qua HTTP %s decode: %v\n", path, err)
		return nil
	}
	data := base["data"]
	if strings.Contains(path, "dept/list") {
		arr := data.([]any)
		out := make([]map[string]any, 0, len(arr))
		for _, x := range arr {
			out = append(out, x.(map[string]any))
		}
		return out
	}
	m := data.(map[string]any)
	arr := m["list"].([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, x := range arr {
		out = append(out, x.(map[string]any))
	}
	return out
}

func mkBootstrap() *conf.Bootstrap {
	return &conf.Bootstrap{
		Qua: &conf.Qua{BaseUrl: "http://stub"},
		SystemDict: &conf.SystemDict{
			Path: "/Users/jayden/Development/Code/Object/stack-haven/avmc/backend-service/app/evie/tool/configs/dictionaries/system.json", HotReload: false,
		},
		TenantRegistry: &conf.TenantRegistry{Path: "/Users/jayden/Development/Code/Object/stack-haven/avmc/backend-service/app/evie/tool/configs/tenants.json"},
		VocabRules: &conf.VocabRules{
			Sources: map[string]*conf.VocabRules_SourceRules{
				"qua": {
					EntityMappings: []*conf.VocabRules_EntityMapping{
						{
							Match: &conf.VocabRules_EntityMapping_Match{EntityType: "user"},
							Emit: &conf.VocabRules_EntityMapping_Emit{
								StandardText: "name", Category: "PERSON",
								Aliases: []string{"mobile"}, Priority: "50",
								IncludeWhen: "status==1",
							},
						},
						{
							Match: &conf.VocabRules_EntityMapping_Match{EntityType: "department"},
							Emit: &conf.VocabRules_EntityMapping_Emit{
								StandardText: "name", Category: "ORGANIZATION",
								Priority: "30",
							},
						},
					},
				},
			},
		},
	}
}

type mockQua struct {
	users []map[string]any
	depts []map[string]any
}

func (m *mockQua) FetchUsersRaw(_ context.Context) ([]map[string]any, error) {
	return m.users, nil
}
func (m *mockQua) FetchDeptsRaw(_ context.Context) ([]map[string]any, error) {
	return m.depts, nil
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}