// Command engine-test 用 ark-lexnorm 引擎规范化两条测试文本，
// 把每次 Change 落到 logs/01-engine.log（JSON Lines）。
//
// Pipeline：
//
//	normalize → disfluency → alias → pinyin → fuzzy
//
// # Lexicon：Compose(user, dept, system)，开启 pinyin 索引
//
// 运行：go run ./cmd/03-engine-test
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/example/09-compare-engine-vs-llm/internal/lexjson"
	"github.com/stack-haven/lexnorm/example/09-compare-engine-vs-llm/internal/pinyinlite"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/fuzzy"
	"github.com/stack-haven/lexnorm/processor/normalize"
	"github.com/stack-haven/lexnorm/processor/pinyin"
)

// ----------------------------------------------------------------------------
// 测试文本（与 PLAN.md §3 一致）
// ----------------------------------------------------------------------------

const testText1 = "好呃，叶海燕夏奇君、袁梦莲参加播种线下课程的确认沟通，加二十五个金种子袁梦莲参加播种线下课程沟通中提出好建议。加十个金种子杨行宇细化阶段性工作执行，并提供执行依据。加十五个金种子杨行宇招投标页面多条件查询，加二十个金种子，填清线下实践卡思路的思考。加十五个金种子朱凤，加三十个金种子沟通供应商明细，仔细朱凤，加二十个金种子沟通交流，各项事务高效田华。加二十个金种子，主动帮助上传资料田华，加三十个金种子，开通客户软件续费，陈新静做二外，标书加四十个金种子，吴旭辉做二p p t模板。加二十个金种子设立群。中午给同事打饭，加十个金种子。陈科航，早上热情向大家问好。驾驶科金种子。五西辉快速完成安排的工作，加二十个菌种子、芦川、阳城、吴旭辉未按要求完成表格录入，加五个黑种子。好。"

const testText2 = "熊龙军给袁梦莲加二十克金种子，提供儿童金种子历史物料，协助播种未来软件，给田华加十五克金种子。根据投标智能体的计划完成数据抓取给田青加十五克金种子。根据儿童金种子的入料分析，产品结合设计给杨须宇加十克金种子持续推进前端技术框架的升级。落第田华给田青加十五克金种子十件卡，相关页面设计给杨须宇加十克金种子解决开发遇到的问题。袁梦莲给邓子加三十克金种子。八月视频账号数据统计给林宇豪加三十克金种子，整理精修游学。照片下，其君给陈新静加三百八十克金，总对肿瘤医院中标菌种子分配给龚建军加二百七十颗金种子。肿瘤医院中标给夏季菌加一百克金种子。肿瘤医院中标给胜利群，卢川五西辉羊城冰月珠缝各家五十颗金种子。肿瘤医院中标给胜利群加二十克金种子处理。三、可业务叶海燕给朱凤给三十克金种子沟通、对接、处理、社保等相关事务给田华。杨徐宇、林宇豪、伍锡辉、扬城冰月独穿，各加五十克金种子，配合会议准备及事务。"

// testText3 是专门为展示 system.json 能力构造的 demo 文本，
// 包含多个 system.json 里登记的 ASR 错读变体。原始真实文本里这些错读不一定同时出现。
// 运行后会看到 fuzzy processor 命中如：金种子→金种籽、金钟子→金种籽、重标→中标 等。
const testText3 = "本次审计中，发现以下 ASR 错读被 engine 识别：金种子应当记录为金种籽，金钟子同金种仔也是错读，另外记录金种资也给紧种子作为可参考样本。关于项目状态：某项目重标后还重标，但重标过程需严格审查。供应商名单上共应商名字写错，需联系供应伤核实。物料清单出现无料、悟料字样，按规范统一为物料。阶段划分采用节段以避免与阶段混淆。执行阶段则是执行。接种接种。"

// ----------------------------------------------------------------------------
// 日志条目 schema
// ----------------------------------------------------------------------------

type changeRecord struct {
	TextID     string  `json:"text_id"`
	Input      string  `json:"input"`
	Output     string  `json:"output"`
	MatchType  string  `json:"match_type"` // exact|alias|pinyin|fuzzy|homophone|disfluency|normalize
	Source     string  `json:"source"`     // entry id
	EntryText  string  `json:"entry_text,omitempty"`
	Kind       string  `json:"kind,omitempty"`
	Step       string  `json:"step"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Confidence float64 `json:"confidence"`
}

type summaryRecord struct {
	TextID        string `json:"text_id"`
	Original      string `json:"original"`
	Normalized    string `json:"normalized"`
	OriginalLen   int    `json:"original_len"`
	NormalizedLen int    `json:"normalized_len"`
	ChangeCount   int    `json:"change_count"`
	LatencyMS     int64  `json:"latency_ms"`
}

// ----------------------------------------------------------------------------

func main() {
	dataDir, logsDir, err := resolveDirs()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		log.Fatal(err)
	}

	logPath := filepath.Join(logsDir, "01-engine.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		log.Fatalf("create log: %v", err)
	}
	defer logFile.Close()
	logJSON := json.NewEncoder(logFile)

	// ----- 加载词库 -----
	lex, err := buildLexicon(dataDir)
	if err != nil {
		log.Fatalf("build lexicon: %v", err)
	}
	fmt.Printf("lexicon: entries=%d version=%q\n\n", lex.Len(), lex.Version())

	// ----- 配置 pipeline -----
	pipeline := lexnorm.NewPipeline(
		normalize.New(),
		disfluency.New(),
		alias.New(lex),
		pinyin.New(lex, pinyinlite.Converter{}),
		fuzzy.New(lex),
	)

	engine, err := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(pipeline),
		// P0-1: 降低 AutoApplyThreshold 0.95 → 0.70
		// 让 conf=0.70 的"菌种子→黑种籽"等 也能 Apply，不再仅 Suggest
		lexnorm.WithConfig(lexnorm.Config{
			AutoApplyThreshold: 0.70,
			SuggestThreshold:   0.50,
			DefaultErrorPolicy: lexnorm.ContinueOnError,
		}),
	)
	if err != nil {
		log.Fatalf("new engine: %v", err)
	}

	// ----- 跑两条文本 -----
	texts := []struct {
		id, content string
	}{
		{"text1", testText1},
		{"text2", testText2},
		{"text3-system-demo", testText3},
	}

	ctx := context.Background()
	for _, t := range texts {
		runOne(ctx, engine, t.id, t.content, lex, logJSON)
		fmt.Println(strings.Repeat("-", 60))
	}

	fmt.Printf("\n[OK] engine log → %s\n", logPath)
}

func runOne(ctx context.Context, engine *lexnorm.Engine, textID, content string, lex lexicon.Lexicon, logJSON *json.Encoder) {
	t0 := time.Now()
	result, err := engine.Normalize(ctx, content)
	latency := time.Since(t0)

	if err != nil {
		fmt.Printf("[%s] ERROR: %v\n", textID, err)
		return
	}

	// 写 summary
	summary := summaryRecord{
		TextID:        textID,
		Original:      result.Original,
		Normalized:    result.Text,
		OriginalLen:   len([]rune(result.Original)),
		NormalizedLen: len([]rune(result.Text)),
		ChangeCount:   len(result.Changes),
		LatencyMS:     latency.Milliseconds(),
	}
	if err := logJSON.Encode(map[string]any{
		"type": "summary",
		"data": summary,
	}); err != nil {
		log.Printf("encode summary: %v", err)
	}

	// 写每条 change
	for _, ch := range result.Changes {
		rec := changeRecord{
			TextID:     textID,
			Input:      ch.From,
			Output:     ch.To,
			MatchType:  guessMatchType(ch),
			Source:     ch.EntryID,
			Step:       ch.Processor,
			Start:      ch.Span.Start,
			End:        ch.Span.End,
			Confidence: ch.Confidence,
		}
		if err := logJSON.Encode(map[string]any{
			"type": "change",
			"data": rec,
		}); err != nil {
			log.Printf("encode change: %v", err)
		}
	}

	fmt.Printf("[%s] changes=%d latency=%dms\n", textID, len(result.Changes), latency.Milliseconds())
	fmt.Printf("  IN : %s\n", truncate(result.Original, 80))
	fmt.Printf("  OUT: %s\n", truncate(result.Text, 80))
}

// guessMatchType 推断 MatchType（基于 ch.Kind 和 ch.Processor）
func guessMatchType(ch lexnorm.Change) string {
	switch ch.Processor {
	case "normalize":
		return "normalize"
	case "disfluency":
		return "disfluency"
	case "alias":
		return "alias"
	case "pinyin":
		return "homophone"
	case "fuzzy":
		return "fuzzy"
	}
	return "unknown"
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func buildLexicon(dataDir string) (lexicon.Lexicon, error) {
	userLex, err := lexjson.LoadFile(filepath.Join(dataDir, "user.json"))
	if err != nil {
		return nil, fmt.Errorf("user.json: %w", err)
	}
	deptLex, err := lexjson.LoadFile(filepath.Join(dataDir, "dept.json"))
	if err != nil {
		return nil, fmt.Errorf("dept.json: %w", err)
	}
	sysLex, err := lexjson.LoadFile(filepath.Join(dataDir, "system.json"))
	if err != nil {
		return nil, fmt.Errorf("system.json: %w", err)
	}

	// 手动收集所有 entries → 走 Builder.WithPinyin() 启用拼音索引。
	allEntries := append([]lexicon.Entry{}, collectAll(userLex)...)
	allEntries = append(allEntries, collectAll(deptLex)...)
	allEntries = append(allEntries, collectAll(sysLex)...)

	builder := lexicon.NewBuilderWithVersion("user+dept+system-2025-09-08").
		Add(allEntries...).
		WithPinyin(pinyinlite.Converter{})
	built, err := builder.Build()
	if err != nil {
		return nil, err
	}
	return built, nil
}

func collectAll(lex *lexjson.LexiconJSON) []lexicon.Entry {
	out := make([]lexicon.Entry, 0, len(lex.Entries))
	for _, e := range lex.Entries {
		le, err := lexjson.ToEntry(e)
		if err != nil {
			log.Printf("warn: skip entry %q: %v", e.ID, err)
			continue
		}
		out = append(out, le)
	}
	return out
}

func resolveDirs() (dataDir, logsDir string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	candidates := []struct{ base string }{
		{base: cwd},
		{base: filepath.Join(cwd, "example", "09-compare-engine-vs-llm")},
	}
	for _, c := range candidates {
		dataC := filepath.Join(c.base, "data")
		logsC := filepath.Join(c.base, "logs")
		if st, err := os.Stat(dataC); err == nil && st.IsDir() {
			return dataC, logsC, nil
		}
	}
	return "", "", fmt.Errorf("data dir not found (cwd=%s)", cwd)
}
