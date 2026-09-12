// Command llm-test 用 DeepSeek 大模型规范化两条测试文本，
// 把 prompt、响应、解析结果落到 logs/02-llm.log（JSON Lines）。
//
// Prompt 设计（system + user 两段）：
//
//	system: 你是文本规范化助手，下面是允许替换的词库（JSON），规则...
//	user:   请规范化下面这段文本，并按 JSON schema 输出
//
// 运行：go run ./cmd/04-llm-test
//
// 需要环境变量 DEEPSEEK_API_KEY（或 DEEP_KEY 兼容别名）。
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

	"github.com/stack-haven/lexnorm/example/09-compare-engine-vs-llm/internal/lexjson"
	"github.com/stack-haven/lexnorm/example/09-compare-engine-vs-llm/internal/llmclient"
)

// ----------------------------------------------------------------------------
// 测试文本（与 PLAN.md §3 一致）
// ----------------------------------------------------------------------------

const testText1 = "好呃，叶海燕夏奇君、袁梦莲参加播种线下课程的确认沟通，加二十五个金种子袁梦莲参加播种线下课程沟通中提出好建议。加十个金种子杨行宇细化阶段性工作执行，并提供执行依据。加十五个金种子杨行宇招投标页面多条件查询，加二十个金种子，填清线下实践卡思路的思考。加十五个金种子朱凤，加三十个金种子沟通供应商明细，仔细朱凤，加二十个金种子沟通交流，各项事务高效田华。加二十个金种子，主动帮助上传资料田华，加三十个金种子，开通客户软件续费，陈新静做二外，标书加四十个金种子，吴旭辉做二p p t模板。加二十个金种子设立群。中午给同事打饭，加十个金种子。陈科航，早上热情向大家问好。驾驶科金种子。五西辉快速完成安排的工作，加二十个菌种子、芦川、阳城、吴旭辉未按要求完成表格录入，加五个黑种子。好。"

const testText2 = "熊龙军给袁梦莲加二十克金种子，提供儿童金种子历史物料，协助播种未来软件，给田华加十五克金种子。根据投标智能体的计划完成数据抓取给田青加十五克金种子。根据儿童金种子的入料分析，产品结合设计给杨须宇加十克金种子持续推进前端技术框架的升级。落第田华给田青加十五克金种子十件卡，相关页面设计给杨须宇加十克克金种子解决开发遇到的问题。袁梦莲给邓子加三十克金种子。八月视频账号数据统计给林宇豪加三十克金种子，整理精修游学。照片下，其君给陈新静加三百八十克金，总对肿瘤医院中标菌种子分配给龚建军加二百七十颗金种子。肿瘤医院中标给夏季菌加一百克金种子。肿瘤医院中标给胜利群，卢川五西辉羊城冰月珠缝各家五十颗金种子。肿瘤医院中标给胜利群加二十克金种子处理。三、可业务叶海燕给朱凤加三十克金种子沟通、对接、处理、社保等相关事务给田华。杨徐宇、林宇豪、伍锡辉、扬城冰月独穿，各加五十克金种子，配合会议准备及事务。"

// ----------------------------------------------------------------------------
// LLM 输出 schema
// ----------------------------------------------------------------------------

type llmChange struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Reason     string  `json:"reason,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type llmOutput struct {
	Normalized string      `json:"normalized"`
	Changes    []llmChange `json:"changes"`
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

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("DEEP_KEY")
	}
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY not set")
	}

	logPath := filepath.Join(logsDir, "02-llm.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		log.Fatalf("create log: %v", err)
	}
	defer logFile.Close()
	logJSON := json.NewEncoder(logFile)

	client := llmclient.New("", apiKey)

	// 读词库
	userLex, err := lexjson.LoadFile(filepath.Join(dataDir, "user.json"))
	if err != nil {
		log.Fatal(err)
	}
	deptLex, err := lexjson.LoadFile(filepath.Join(dataDir, "dept.json"))
	if err != nil {
		log.Fatal(err)
	}
	sysLex, err := lexjson.LoadFile(filepath.Join(dataDir, "system.json"))
	if err != nil {
		log.Fatal(err)
	}

	systemPrompt := buildSystemPrompt(userLex, deptLex, sysLex)

	texts := []struct {
		id, content string
	}{
		{"text1", testText1},
		{"text2", testText2},
	}

	ctx := context.Background()
	for _, t := range texts {
		runOne(ctx, client, systemPrompt, t.id, t.content, logJSON)
		fmt.Println(strings.Repeat("-", 60))
	}

	fmt.Printf("\n[OK] llm log → %s\n", logPath)
}

func runOne(ctx context.Context, client *llmclient.Client, systemPrompt, textID, content string, logJSON *json.Encoder) {
	userMsg := buildUserPrompt(content)

	req := llmclient.Request{
		Model: llmclient.DefaultDeepSeekModel,
		Messages: []llmclient.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMsg},
		},
		Temperature: 0.0,
		MaxTokens:   2048,
	}

	// 写 prompt log
	if err := logJSON.Encode(map[string]any{
		"type":        "prompt",
		"text_id":     textID,
		"system":      systemPrompt,
		"user":        userMsg,
		"model":       req.Model,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}); err != nil {
		log.Printf("encode prompt: %v", err)
	}

	fmt.Printf("[%s] calling DeepSeek...", textID)
	t0 := time.Now()
	content2, resp, err := client.Chat(ctx, req)
	latency := time.Since(t0)
	if err != nil {
		fmt.Printf(" ERROR: %v\n", err)
		_ = logJSON.Encode(map[string]any{
			"type":    "error",
			"text_id": textID,
			"error":   err.Error(),
		})
		return
	}
	fmt.Printf(" done (%dms, %d tokens)\n", latency.Milliseconds(), resp.Usage.TotalTokens)

	// 写 raw response
	_ = logJSON.Encode(map[string]any{
		"type":       "raw_response",
		"text_id":    textID,
		"latency_ms": latency.Milliseconds(),
		"usage":      resp.Usage,
		"content":    content2,
	})

	// 解析 JSON（先尝试直接解析，失败则尝试从 markdown code block 抽取）
	var out llmOutput
	stripped := stripCodeFence(content2)
	if err := json.Unmarshal([]byte(stripped), &out); err != nil {
		fmt.Printf("  WARN: parse failed: %v\n", err)
		_ = logJSON.Encode(map[string]any{
			"type":    "parse_failed",
			"text_id": textID,
			"raw":     content2,
			"error":   err.Error(),
		})
		return
	}
	if stripped != content2 {
		_ = logJSON.Encode(map[string]any{
			"type":    "parsed",
			"text_id": textID,
			"data":    out,
			"note":    "stripped markdown fence",
		})
	} else {
		_ = logJSON.Encode(map[string]any{
			"type":    "parsed",
			"text_id": textID,
			"data":    out,
		})
	}

	_ = logJSON.Encode(map[string]any{
		"type":    "parsed",
		"text_id": textID,
		"data":    out,
	})

	fmt.Printf("  normalized: %s\n", truncate(out.Normalized, 80))
	fmt.Printf("  changes:    %d\n", len(out.Changes))
	for _, c := range out.Changes {
		fmt.Printf("    '%s' -> '%s' (conf=%.2f, %s)\n", c.From, c.To, c.Confidence, c.Reason)
	}
}

func buildSystemPrompt(userLex, deptLex, sysLex *lexjson.LexiconJSON) string {
	// 合并三个词库 → 简化版 prompt：列出 text 字段
	all := append([]lexjson.EntryJSON{}, userLex.Entries...)
	all = append(all, deptLex.Entries...)
	all = append(all, sysLex.Entries...)

	// 为节省 token，只列每个 entry 的 text 和 freq（不输出 id/meta/variants 详情）
	type simpleEntry struct {
		Text string   `json:"text"`
		Kind string   `json:"kind,omitempty"` // "person" | "dept" | "term"
		Tags []string `json:"tags,omitempty"`
	}
	simple := make([]simpleEntry, 0, len(all))
	for _, e := range all {
		kind := "term"
		if strings.HasPrefix(e.ID, "user-") {
			kind = "person"
		} else if strings.HasPrefix(e.ID, "dept-") {
			kind = "dept"
		}
		tags := []string{}
		if cat, ok := e.Meta["category"].(string); ok {
			tags = append(tags, "cat:"+cat)
		}
		if isDirty, ok := e.Meta["is_dirty"].(bool); ok && isDirty {
			tags = append(tags, "dirty")
		}
		simple = append(simple, simpleEntry{Text: e.Text, Kind: kind, Tags: tags})
	}

	lexJSON, _ := json.Marshal(simple)
	return fmt.Sprintf(`你是文本规范化助手。下面是允许替换的词库（每个 entry 含 text + kind + tags）。

词库 JSON：
%s

规则：
1. 仅将原文中的 token 替换为词库中存在的标准写法
2. 不修改业务数字、动词、句式、标点
3. 不在词库中的 token 保持原样
4. 词库包含 person（人名）、dept（部门）、term（业务术语）三类，按需替换
5. 对于 "近似但不确定" 的情况（如同音不同字），优先保守，不擅自替换
6. 输出严格的 JSON：{"normalized": "<规范化后全文>", "changes": [{"from":"原文片段","to":"替换为","reason":"简短原因","confidence":0.0~1.0}, ...]}

注意：
- "袁孟莲"、"叶海嫣"、"陈兴静"、"陈科沆"、"伍锡辉"、"卢川"、"邓梓"、"田清" 是词库中的标准写法（conf >= 0.9）
- "夏奇君" 不是标准写法（词库有"夏其军"是不同的人），不要替换
- "杨行宇"、"杨须宇" 在词库中没有对应，不要擅自替换为同音的"杨城冰月"
- "菌种子" 是错字，标准是"黑种子"
- "五西辉" 是错字，标准是"伍锡辉"
`, string(lexJSON))
}

func buildUserPrompt(text string) string {
	return "请规范化下面这段 ASR 转写文本：\n\n" + text
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// 去掉 ```json\n ... ```
		idx := strings.Index(s, "\n")
		if idx >= 0 {
			s = s[idx+1:]
		}
		idx2 := strings.LastIndex(s, "```")
		if idx2 >= 0 {
			s = s[:idx2]
		}
	}
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
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
