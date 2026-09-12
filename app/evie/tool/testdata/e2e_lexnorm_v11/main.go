// e2e_lexnorm_v11 端到端验证 evie/tool + lexnorm v1.1 完整文本规范化流程。
//
// 用途：
//   - 不依赖 ASR（funasr/xunfei）
//   - 不依赖外部 qua / Redis
//   - 覆盖 8 层 pipeline 的每层典型场景
//   - 验证 lexnorm v1.1 升级后行为不退化
//
// 运行：
//
//	cd backend-service/app/evie/tool
//	go run ./testdata/e2e_lexnorm_v11
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"backend-service/app/evie/tool/internal/biz/processor"
	pkgpinyin "backend-service/pkg/pinyin"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/ctxproc"
	"github.com/stack-haven/lexnorm/processor/deterministic"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/normalize"
	lexpinyin "github.com/stack-haven/lexnorm/processor/pinyin"
)

func main() {
	// 1. 构造 Lexicon（demo + system 合并）
	lex := buildLexicon()
	fmt.Printf("[setup] lexicon built\n")

	// 2. 构造 Pipeline（evie/tool 标准 8 层 + 业务 fuzzy_vocab）
	pipe := lexnorm.NewPipeline(
		normalize.New(),
		disfluency.New(),
		alias.New(lex),
		deterministic.New(lex),
		lexpinyin.New(lex, &pinyinConverter{}),
		processor.NewFuzzyVocabProcessor(lex, processor.DefaultFuzzyVocabConfig()),
		ctxproc.New(),
	)

	// 3. 构造 Engine
	engine, err := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(pipe),
		lexnorm.WithConfig(lexnorm.Config{
			AutoApplyThreshold: 0.95,
			SuggestThreshold:   0.65,
		}),
	)
	if err != nil {
		fmt.Printf("[FAIL] engine.New: %v\n", err)
		os.Exit(1)
	}

	// 4. 端到端 case
	cases := []struct {
		name  string
		input string
	}{
		{"cleaning: 全角标点", "你好，世界。"},
		{"cleaning: 多余空白", "  你好  世界  "},
		{"disfluency: 独立叹词", "呃 我想去"},
		{"disfluency: 多字歧义（独立）", "那个 我想去"},
		{"disfluency: 多字歧义（非独立保留）", "呃 那个 我想去"},
		{"alias: 完全匹配（佘丽群）", "佘丽群 报告"},
		{"fuzzy: 周丽群→佘丽群（dist=1）", "周丽群 报告"},
		{"fuzzy: 佘莉群（dist=1）", "佘莉群"},
		{"fuzzy: lock_alias 保护", "金种籽 报告"},
		{"综合: 8 层联调", "呃，周丽群明天的报告，那个黑种籽计划"},
	}

	results := make([]map[string]any, 0, len(cases))
	for _, tc := range cases {
		res, err := engine.Normalize(context.Background(), tc.input)
		if err != nil {
			fmt.Printf("[ERR ] %s: %v\n", tc.name, err)
			continue
		}
		row := map[string]any{
			"case":     tc.name,
			"input":    tc.input,
			"output":   res.Text,
			"status":   res.Status.String(),
			"duration": res.Duration.Microseconds(),
			"changes":  len(res.Changes),
			"errors":   len(res.Errors),
			"changes_detail": func() []map[string]string {
				out := make([]map[string]string, 0, len(res.Changes))
				for _, c := range res.Changes {
					out = append(out, map[string]string{
						"from":   c.From,
						"to":     c.To,
						"action": c.Action.String(),
						"source": c.Source,
					})
				}
				return out
			}(),
			"steps": func() []map[string]any {
				out := make([]map[string]any, 0, len(res.Steps))
				for _, s := range res.Steps {
					out = append(out, map[string]any{
						"name":          s.Processor,
						"version":       s.ProcessorVersion,
						"duration_us":   s.Duration.Microseconds(),
						"change_count":  s.ChangeCount,
						"category":      string(s.Category),
						"deterministic": s.Deterministic,
					})
				}
				return out
			}(),
			"runtime": map[string]any{
				"lex_version": res.Runtime.LexiconVersion,
				"pipeline":    res.Runtime.PipelineVersion,
				"profile":     res.Runtime.ProfileID,
			},
		}
		results = append(results, row)
		fmt.Printf("[OK  ] %-45s | in=%-30s → out=%-30s chg=%d dur=%dµs\n",
			tc.name, tc.input, res.Text, len(res.Changes), res.Duration.Microseconds())
	}

	// 5. 输出 JSON
	outFile := "/tmp/e2e_lexnorm_v11_result.json"
	b, _ := json.MarshalIndent(results, "", "  ")
	if err := os.WriteFile(outFile, b, 0644); err != nil {
		fmt.Printf("[WARN] write %s: %v\n", outFile, err)
	} else {
		fmt.Printf("[OK  ] wrote %s\n", outFile)
	}

	// 6. 打印详细步骤对比（验证 8 层都跑了）
	fmt.Println("\n=== 详细步骤分析（run " + cases[len(cases)-1].name + "）===")
	final := results[len(results)-1]
	for _, st := range final["steps"].([]map[string]any) {
		fmt.Printf("  %-15s v=%-3s dur=%5vµs chg=%d cat=%s det=%v\n",
			st["name"], st["version"], st["duration_us"], st["change_count"],
			st["category"], st["deterministic"])
	}

	fmt.Println("\n=== Changes 详情（run " + cases[len(cases)-1].name + "）===")
	for _, c := range final["changes_detail"].([]map[string]string) {
		fmt.Printf("  [%s] %-12s \"%s\" → \"%s\"\n", c["source"], c["action"], c["from"], c["to"])
	}
}

// buildLexicon 构造测试用 Lexicon。
func buildLexicon() lexicon.Lexicon {
	entries := []lexicon.Entry{
		// PERSON 类别（fuzzy 重点）
		{ID: "p1", Text: "佘丽群", Meta: map[string]any{"category": "PERSON", "priority": 100}},
		{ID: "p2", Text: "测试一", Meta: map[string]any{"category": "PERSON", "priority": 50}},
		{ID: "p3", Text: "田华", Meta: map[string]any{"category": "PERSON", "priority": 50}},
		{ID: "p4", Text: "田花", Meta: map[string]any{"category": "PERSON", "priority": 50}},
		// PRODUCT 类别 + lock_alias 保护
		{ID: "prod1", Text: "金种籽", Meta: map[string]any{"category": "PRODUCT", "priority": 100, "lock_alias": true}},
		{ID: "prod2", Text: "黑种籽", Meta: map[string]any{"category": "PRODUCT", "priority": 100, "lock_alias": true}},
		// ORGANIZATION 类别
		{ID: "o1", Text: "工程部", Meta: map[string]any{"category": "ORGANIZATION", "priority": 30}},
		{ID: "o2", Text: "测试组", Meta: map[string]any{"category": "ORGANIZATION", "priority": 30}},
	}

	// 关键变体：让 deterministic processor 能命中
	for i := range entries {
		switch entries[i].Text {
		case "佘丽群":
			entries[i].Variants = []lexicon.Variant{
				{Text: "周丽群", Kind: lexicon.VariantHomophone, Confidence: 1.0, Source: "system"},
			}
		case "黑种籽":
			entries[i].Variants = []lexicon.Variant{
				{Text: "黑中子", Kind: lexicon.VariantHomophone, Confidence: 1.0, Source: "system"},
				{Text: "黑种仔", Kind: lexicon.VariantCorrection, Confidence: 1.0, Source: "system"},
			}
		}
	}

	// 排序（确定性）
	sort.Slice(entries, func(i, j int) bool { return entries[i].Text < entries[j].Text })

	lex, err := lexicon.NewBuilderWithVersion("demo-v1").
		Add(entries...).
		Build()
	if err != nil {
		panic(fmt.Sprintf("Build Lexicon: %v", err))
	}
	return lex
}

// pinyinConverter 实现 lexnorm.PinyinConverter：基于 backend-service/pkg/pinyin。
type pinyinConverter struct{}

func (c *pinyinConverter) ToPinyin(text string) []string {
	if text == "" {
		return nil
	}
	// 简化：直接返回所有可能的同音字（实际由 pkg/pinyin 转换）
	// 这里用 backend-service/pkg/pinyin 提供真实实现
	r, err := pkgpinyin.Convert(text, true)
	if err != nil || r == nil {
		return []string{text}
	}
	parts := strings.Fields(r.Pinyin)
	if len(parts) == 0 {
		return []string{text}
	}
	return parts
}
