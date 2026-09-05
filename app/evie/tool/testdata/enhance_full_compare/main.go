// 独立验证：拿 evie/tool 真实 ASR rawText → 重新加载 system.json + 真实 qua 接口
// → 走完整 enhancement pipeline → 输出 enhancedText + changes
//
// 与 evie/tool HTTP 接口返回结果做对比，验证端到端 pipeline 行为可复现。
//
// 用法：
//   cd backend-service
//   go run -mod=mod ./app/evie/tool/testdata/enhance_full_compare/ \
//     -conf=./app/evie/tool/configs/config.yaml \
//     -token=<qua-token> \
//     -tenant=<tenant-id> \
//     -raw=/tmp/raw_text.txt
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/durationpb"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"
)

func main() {
	confFile := flag.String("conf", "", "config file path (required)")
	token := flag.String("token", "", "qua access token (required)")
	tenantID := flag.String("tenant", "", "tenant id (required)")
	rawPath := flag.String("raw", "", "rawText file path (required)")
	flag.Parse()
	if *confFile == "" || *token == "" || *tenantID == "" || *rawPath == "" {
		fmt.Fprintln(os.Stderr, "usage: enhance_full_compare -conf=<file> -token=<t> -tenant=<id> -raw=<file>")
		os.Exit(2)
	}

	rawTextBytes, err := os.ReadFile(*rawPath)
	check("read raw", err)
	rawText := string(rawTextBytes)
	fmt.Printf("=== 加载 rawText: %d 字符 ===\n\n", len(rawText))

	// 1. 加载 conf（kratos loader）
	c := config.New(config.WithSource(file.NewSource(*confFile)))
	defer c.Close()
	if err := c.Load(); err != nil {
		die("load conf", err)
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		die("scan conf", err)
	}
	fmt.Printf("=== conf 加载成功: base_url=%s system_dict=%s ===\n\n",
		bc.Qua.GetBaseUrl(), bc.SystemDict.GetPath())

	// 2. 注入 AuthInfo
	ctx := data.WithAuthInfo(context.Background(), &data.AuthInfo{
		TenantID:    *tenantID,
		AccessToken: *token,
		UserID:      "compare-bot",
	})

	// 3. 加载 system.json
	dictPath := bc.SystemDict.GetPath()
	if dictPath == "" {
		dictPath = "app/evie/tool/configs/dictionaries/system.json"
	}
	dictAbs, _ := filepath.Abs(dictPath)
	vb, err := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictAbs})
	check("NewVocabularyBuilder", err)
	sysSnap := vb.Build(ctx, *tenantID)
	fmt.Printf("=== system snapshot: %d entries, %d relations ===\n\n",
		sysSnap.EntryCount(), sysSnap.RelationCount())

	// 4. 真实 qua 接口
	fetcher, err := data.NewQuaClient(bc.Qua, nil)
	check("NewQuaClient", err)
	fmt.Println("✓ qua client constructed")

	// 5. 真实 sync
	vocabSource := data.NewQuaVocabularySource(fetcher)
	normalizer := biz.NewNormalizerFromConf(bc.VocabRules, log.DefaultLogger)
	registry := biz.NewTenantRegistry(bc.TenantRegistry)
	registry.Ensure(*tenantID)
	syncer := biz.NewVocabSyncer(registry, vb, normalizer, vocabSource,
		&conf.TenantVocab{SyncInterval: durationpb.New(60_000_000_000)},
		log.DefaultLogger,
	)
	if err := syncer.SyncTenant(ctx, *tenantID); err != nil {
		die("SyncTenant", err)
	}
	tenantSnap := vb.Build(ctx, *tenantID)
	fmt.Printf("=== tenant snapshot: %d entries, %d relations (含 system) ===\n\n",
		tenantSnap.EntryCount(), tenantSnap.RelationCount())

	// debug: 看 system relations 在 merged snapshot 中是否还存在
	fmt.Println("=== DEBUG: 金种子-related relations ===")
	for rt, rs := range tenantSnap.Relations {
		if rt == "金种子" || rt == "金种籽" || rt == "金种仔" {
			for _, r := range rs {
				fmt.Printf("  rel: %q → [%s] entry_id=%d target_id=%d\n",
					rt, r.RelationType, r.EntryID, r.TargetEntryID)
			}
		}
	}
	fmt.Println()

	// 6. BuildPipeline
	policy := biz.NewPolicyFromConf(bc.Enhancement)
	procReg := builtins.NewDefaultRegistry()
	pipeline, err := textenhance.BuildPipeline(procReg, policy)
	check("BuildPipeline", err)
	enhUC := biz.NewEnhancementUsecase(pipeline, vb, policy)

	// 7. 跑 EnhanceText
	resp, err := enhUC.EnhanceText(ctx, rawText, *tenantID)
	check("EnhanceText", err)

	fmt.Println("=== 独立验证结果 ===")
	fmt.Printf("  rawText : %d 字符\n", len(rawText))
	fmt.Printf("  outText : %d 字符\n", len(resp.EnhancedText))
	fmt.Printf("  changes : %d\n", len(resp.Changes))
	for i, c := range resp.Changes {
		fmt.Printf("  [%2d] [%s] %q → %q  type=%s src=%s conf=%.2f\n",
			i, c.Action, c.From, c.To, c.Type, c.Source, c.Confidence)
	}

	// 保存独立结果
	out, _ := json.MarshalIndent(map[string]any{
		"rawText":      rawText,
		"enhancedText": resp.EnhancedText,
		"changes":      resp.Changes,
	}, "", "  ")
	if err := os.WriteFile("/tmp/standalone_result.json", out, 0644); err != nil {
		die("write", err)
	}
	fmt.Println("\n✓ saved to /tmp/standalone_result.json")
}

func check(label string, err error) {
	if err != nil {
		die(label, err)
	}
}

func die(label string, err error) {
	fmt.Fprintf(os.Stderr, "✗ %s: %v\n", label, err)
	os.Exit(1)
}
