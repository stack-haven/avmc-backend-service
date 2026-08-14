//go:build mock

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"backend-service/app/evie/service/internal/data"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/correctionrule"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryword"
	entviewer "backend-service/app/evie/service/internal/data/ent/viewer"
	"backend-service/app/evie/service/internal/runtimeconfig"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql"
)

var flagconf = "../../configs"

// mockWord 是字典标准词 + 别名的种子定义。
type mockWord struct {
	tenantID uint32
	word     string
	category string
	aliases  []string
}

func main() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path")
	flag.Parse()
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "evie mock failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx = entviewer.NewSystemContext(ctx)
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	client, err := data.NewEntClient(bc.Data, log.DefaultLogger)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := seed(ctx, client); err != nil {
		return err
	}
	fmt.Println("evie mock data ready: dictionary words seeded for tenant 1")
	return nil
}

func seed(ctx context.Context, client *gen.Client) error {
	words := []mockWord{
		// 人员类（企业员工）
		{tenantID: 1, word: "田华", category: "person", aliases: []string{"小田", "田经理", "田工"}},
		{tenantID: 1, word: "张伟", category: "person", aliases: []string{"小张", "张经理"}},
		{tenantID: 1, word: "李娜", category: "person", aliases: []string{"小李", "娜姐"}},
		// 组织类（部门）
		{tenantID: 1, word: "技术研发部", category: "org", aliases: []string{"技术部", "研发部", "技术研发"}},
		{tenantID: 1, word: "市场销售部", category: "org", aliases: []string{"市场部", "销售部"}},
		// 产品类（企业产品/术语）
		{tenantID: 1, word: "金种籽", category: "product", aliases: []string{"金种子", "金种籽酒"}},
		{tenantID: 1, word: "种籽奖励", category: "term", aliases: []string{"种子奖励", "种籽激励"}},
	}
	for _, w := range words {
		exists, err := client.DictionaryWord.Query().
			Where(dictionaryword.TenantIDEQ(w.tenantID), dictionaryword.WordEQ(w.word)).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("check word %s: %w", w.word, err)
		}
		if exists {
			continue // already seeded
		}
		row, err := client.DictionaryWord.Create().
			SetTenantID(w.tenantID).
			SetWord(w.word).
			SetLevel("tenant").
			SetCategory(w.category).
			SetSource("manual").
			Save(ctx)
		if err != nil {
			return fmt.Errorf("seed word %s: %w", w.word, err)
		}
		for _, a := range w.aliases {
			if _, err := client.DictionaryAlias.Create().
				SetTenantID(w.tenantID).
				SetWordID(row.ID).
				SetAlias(a).
				SetSource("manual").
				SetWeight(1.0).
				Save(ctx); err != nil {
				return fmt.Errorf("seed alias %s: %w", a, err)
			}
		}
	}
	return seedCorrectionRules(ctx, client)
}

// seedCorrectionRules 种子纠错规则（规则纠错器使用）。
func seedCorrectionRules(ctx context.Context, client *gen.Client) error {
	rules := []struct {
		source, target, typ string
	}{
		{source: "功课", target: "攻克", typ: "dictionary"},
		{source: "金种子", target: "金种籽", typ: "product"},
		{source: "种子奖励", target: "种籽奖励", typ: "dictionary"},
	}
	for _, r := range rules {
		exists, err := client.CorrectionRule.Query().
			Where(correctionrule.TenantIDEQ(1), correctionrule.SourceEQ(r.source)).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("check rule %s: %w", r.source, err)
		}
		if exists {
			continue
		}
		if _, err := client.CorrectionRule.Create().
			SetTenantID(1).
			SetSource(r.source).
			SetTarget(r.target).
			SetType(r.typ).
			SetPriority(100).
			Save(ctx); err != nil {
			return fmt.Errorf("seed rule %s: %w", r.source, err)
		}
	}
	return nil
}
