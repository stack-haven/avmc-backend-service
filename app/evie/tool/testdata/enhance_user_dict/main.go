// 单元级验证：用 qua 真实人名（4 个）+ 1 个组织 跑 enhance pipeline
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/durationpb"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"
)

const testTenantID = "1889501240003497986"

func main() {
	ctx := context.Background()
	dictPath, _ := filepath.Abs("app/evie/tool/configs/dictionaries/system.json")
	vb, err := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictPath})
	check("vb", err)

	// 模拟 qua 真实数据：4 个用户 + 1 个组织（人名 + 组织名）
	raws := []biz.RawEntity{
		{SourceID: "u1", EntityType: "user", Source: "qua",
			Data: map[string]any{"id": "1", "name": "熊龙军", "status": 1}},
		{SourceID: "u2", EntityType: "user", Source: "qua",
			Data: map[string]any{"id": "2", "name": "田华", "status": 1}},
		{SourceID: "u3", EntityType: "user", Source: "qua",
			Data: map[string]any{"id": "3", "name": "于云海", "status": 1}},
		{SourceID: "u4", EntityType: "user", Source: "qua",
			Data: map[string]any{"id": "4", "name": "夏其军", "status": 1}},
		{SourceID: "d1", EntityType: "department", Source: "qua",
			Data: map[string]any{"id": "1904450303484194818", "name": "倍多客科技"}},
	}

	normalizer := biz.NewNormalizer(&biz.RuleSet{
		Sources: map[string]*biz.SourceRules{
			"qua": {
				Source: "qua",
				EntityMappings: []biz.EntityMapping{
					{
						Match: biz.MatchCondition{EntityType: "user"},
						Emit: biz.EmitSpec{
							StandardText: "name", Category: "PERSON", Priority: "50",
							IncludeWhen: "status==1",
						},
					},
					{
						Match: biz.MatchCondition{EntityType: "department"},
						Emit: biz.EmitSpec{
							StandardText: "name", Category: "ORGANIZATION", Priority: "30",
						},
					},
				},
			},
		},
	})

	// 走完整 sync 流程（与 SyncTenant 一致）
	registry := biz.NewTenantRegistry(&conf.TenantRegistry{Path: ""})
	registry.Ensure(testTenantID)
	syncer := biz.NewVocabSyncer(registry, vb, normalizer,
		mockSource{raws: raws},
		&conf.TenantVocab{SyncInterval: durationpb.New(time.Minute)},
		log.DefaultLogger,
	)
	if err := syncer.SyncTenant(ctx, testTenantID); err != nil {
		check("sync", err)
	}

	snap := vb.Build(ctx, testTenantID)
	fmt.Printf("=== snapshot: %d entries, %d relations ===\n", snap.EntryCount(), snap.RelationCount())
	for k, e := range snap.Entries {
		fmt.Printf("    [%s] %s (id=%d)\n", e.Category, k, e.ID)
	}

	// build pipeline（8 层全开，让 user 替换走 alias_resolution / vocab_matching）
	policy := biz.NewPolicyFromConf(&conf.Enhancement{
		Pipeline: []string{
			"cleaning", "filler", "vocab_matching", "alias_resolution",
			"deterministic_replacement", "phrase_standardization",
			"pinyin_correction", "fuzzy_matching", "context_correction",
		},
	})
	procReg := builtins.NewDefaultRegistry()
	pipeline, err := textenhance.BuildPipeline(procReg, policy)
	check("BuildPipeline", err)

	enhUC := biz.NewEnhancementUsecase(pipeline, vb, policy)

	tests := []struct{ in, desc string }{
		{"于云海加了五个种子", "qua 人名匹配（标准词）"},
		{"熊龙君提交了报告", "qua 人名 fuzzy 匹配（熊龙君→熊龙军）"},
		{"于运海提交了报告", "qua 人名同音匹配（于运海→于云海）"},
		{"金种子情况", "system 别名匹配（金种子→金种籽）"},
	}
	fmt.Println()
	fmt.Println("=== Enhance 验证 ===")
	for _, tc := range tests {
		resp, err := enhUC.EnhanceText(ctx, tc.in, testTenantID)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", tc.desc, err)
			continue
		}
		tag := "  "
		if resp.EnhancedText != tc.in {
			tag = "✓ "
		}
		fmt.Printf("  %s%s\n    in : %s\n    out: %s\n", tag, tc.desc, tc.in, resp.EnhancedText)
		for _, c := range resp.Changes {
			fmt.Printf("      [%s] %s → %s\n", c.Type, c.From, c.To)
		}
	}
}

type mockSource struct{ raws []biz.RawEntity }

func (m mockSource) Name() string                                       { return "mock" }
func (m mockSource) Fetch(_ context.Context) ([]biz.RawEntity, error) { return m.raws, nil }

func check(l string, e error) {
	if e != nil {
		fmt.Printf("✗ %s: %v\n", l, e)
		os.Exit(1)
	}
}
