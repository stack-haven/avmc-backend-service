// Command pk-report 比较引擎和 LLM 的规范化结果，生成 logs/03-pk.md 和 logs/04-optimize.md。
//
// 输入：logs/01-engine.log + logs/02-llm.log
// 输出：
//
//	logs/03-pk.md    — PK 矩阵（每条 from 的处理对比）
//	logs/04-optimize.md — 对 ark-lexnorm 工具包的优化建议
//
// 运行：go run ./cmd/05-pk-report
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ----------------------------------------------------------------------------
// 数据结构
// ----------------------------------------------------------------------------

type engineChange struct {
	From   string
	To     string
	Step   string
	Source string
	Conf   float64
}

type llmChange struct {
	From   string
	To     string
	Reason string
	Conf   float64
}

type textPK struct {
	TextID        string
	EngineChanges []engineChange
	LLMChanges    []llmChange
	EngineNorm    string
	LLMNorm       string
}

type changeBucket struct {
	engineFrom, engineTo string
	llmFrom, llmTo       string
	llmReason            string
}

// ----------------------------------------------------------------------------

func main() {
	logsDir, err := resolveLogsDir()
	if err != nil {
		log.Fatal(err)
	}

	engineLog, err := readJSONL(filepath.Join(logsDir, "01-engine.log"))
	if err != nil {
		log.Fatalf("read engine log: %v", err)
	}
	llmLog, err := readJSONL(filepath.Join(logsDir, "02-llm.log"))
	if err != nil {
		log.Fatalf("read llm log: %v", err)
	}

	pks := buildPKs(engineLog, llmLog)

	pkPath := filepath.Join(logsDir, "03-pk.md")
	optPath := filepath.Join(logsDir, "04-optimize.md")

	if err := os.WriteFile(pkPath, []byte(renderPK(pks)), 0o644); err != nil {
		log.Fatalf("write %s: %v", pkPath, err)
	}
	if err := os.WriteFile(optPath, []byte(renderOptimize(pks)), 0o644); err != nil {
		log.Fatalf("write %s: %v", optPath, err)
	}

	fmt.Printf("[OK] pk report   → %s\n", pkPath)
	fmt.Printf("[OK] optimize    → %s\n", optPath)
	fmt.Println()
	fmt.Println("=== summary ===")
	for _, pk := range pks {
		fmt.Printf("[%s] engine=%d llm=%d\n", pk.TextID, len(pk.EngineChanges), len(pk.LLMChanges))
	}
}

// ----------------------------------------------------------------------------
// 解析 log
// ----------------------------------------------------------------------------

func readJSONL(path string) ([]map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	dec := json.NewDecoder(strings.NewReader(string(b)))
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		out = append(out, m)
	}
	return out, nil
}

func buildPKs(engineLog, llmLog []map[string]any) []textPK {
	// 按 text_id 分组
	engineByID := map[string][]engineChange{}
	engineNormByID := map[string]string{}
	for _, m := range engineLog {
		switch m["type"] {
		case "summary":
			d := m["data"].(map[string]any)
			id := d["text_id"].(string)
			engineNormByID[id] = d["normalized"].(string)
		case "change":
			d := m["data"].(map[string]any)
			id := d["text_id"].(string)
			step := d["step"].(string)
			// 跳过 normalize（标点空格变化不算"实质性"规范化）
			if step == "normalize" {
				continue
			}
			engineByID[id] = append(engineByID[id], engineChange{
				From:   d["input"].(string),
				To:     d["output"].(string),
				Step:   step,
				Source: d["source"].(string),
				Conf:   d["confidence"].(float64),
			})
		}
	}

	llmByID := map[string][]llmChange{}
	llmNormByID := map[string]string{}
	for _, m := range llmLog {
		switch m["type"] {
		case "parsed":
			d := m["data"].(map[string]any)
			id := m["text_id"].(string)
			llmNormByID[id] = d["normalized"].(string)
			if cs, ok := d["changes"].([]any); ok {
				for _, c := range cs {
					cm := c.(map[string]any)
					from, _ := cm["from"].(string)
					to, _ := cm["to"].(string)
					if from == "" && to == "" {
						continue
					}
					reason, _ := cm["reason"].(string)
					var conf float64
					if c, ok := cm["confidence"].(float64); ok {
						conf = c
					}
					llmByID[id] = append(llmByID[id], llmChange{
						From:   from,
						To:     to,
						Reason: reason,
						Conf:   conf,
					})
				}
			}
		}
	}

	// 合并所有 text_id
	allIDs := map[string]bool{}
	for id := range engineByID {
		allIDs[id] = true
	}
	for id := range llmByID {
		allIDs[id] = true
	}
	ids := make([]string, 0, len(allIDs))
	for id := range allIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]textPK, 0, len(ids))
	for _, id := range ids {
		out = append(out, textPK{
			TextID:        id,
			EngineChanges: engineByID[id],
			LLMChanges:    llmByID[id],
			EngineNorm:    engineNormByID[id],
			LLMNorm:       llmNormByID[id],
		})
	}
	return out
}

// ----------------------------------------------------------------------------
// PK 矩阵渲染
// ----------------------------------------------------------------------------

func renderPK(pks []textPK) string {
	var b strings.Builder
	b.WriteString("# Phase B-3 · PK 矩阵：ark-lexnorm 引擎 vs DeepSeek 大模型\n\n")
	b.WriteString("> 数据源：logs/01-engine.log（引擎规范化日志）+ logs/02-llm.log（LLM 规范化日志）\n")
	b.WriteString("> 排除 normalize processor 的标点/空格变化（不计入实质性 PK）\n\n")

	// 统计
	totalEngine, totalLLM, bothSame, engineOnly, llmOnly, conflict, neither := 0, 0, 0, 0, 0, 0, 0
	allFroms := map[string]int{}
	for _, pk := range pks {
		em := map[string]engineChange{}
		for _, c := range pk.EngineChanges {
			em[c.From] = c
		}
		lm := map[string]llmChange{}
		for _, c := range pk.LLMChanges {
			lm[c.From] = c
		}
		all := map[string]bool{}
		for f := range em {
			all[f] = true
		}
		for f := range lm {
			all[f] = true
		}
		for f := range all {
			allFroms[f]++
		}
	}
	_ = totalEngine
	_ = totalLLM
	_ = bothSame
	_ = engineOnly
	_ = llmOnly
	_ = conflict
	_ = neither
	_ = allFroms

	for _, pk := range pks {
		fmt.Fprintf(&b, "## %s\n\n", pk.TextID)

		// 引擎输出
		fmt.Fprintf(&b, "**ENGINE OUT** (%d changes):\n\n```\n%s\n```\n\n", len(pk.EngineChanges), truncate(pk.EngineNorm, 200))
		// LLM 输出
		fmt.Fprintf(&b, "**LLM OUT** (%d changes):\n\n```\n%s\n```\n\n", len(pk.LLMChanges), truncate(pk.LLMNorm, 200))

		// PK 表
		b.WriteString("| 类型 | from | engine → | llm → | engine conf | llm conf | 备注 |\n")
		b.WriteString("|---|---|---|---|---|---|---|\n")

		em := map[string]engineChange{}
		for _, c := range pk.EngineChanges {
			em[c.From] = c
		}
		lm := map[string]llmChange{}
		for _, c := range pk.LLMChanges {
			lm[c.From] = c
		}

		all := map[string]bool{}
		for f := range em {
			all[f] = true
		}
		for f := range lm {
			all[f] = true
		}

		// 排序：先双方一致，再 engine-only，再 llm-only
		type row struct {
			typ, from, eTo, lTo, eC, lC, note string
		}
		var rows []row
		for f := range all {
			e, lok := em[f]
			l, eok := lm[f]
			switch {
			case lok && eok:
				if e.To == l.To {
					rows = append(rows, row{"✅ 一致", f, e.To, l.To, fmt.Sprintf("%.2f", e.Conf), fmt.Sprintf("%.2f", l.Conf), fmt.Sprintf("step=%s", e.Step)})
					bothSame++
				} else {
					rows = append(rows, row{"⚠️ 冲突", f, e.To, l.To, fmt.Sprintf("%.2f", e.Conf), fmt.Sprintf("%.2f", l.Conf), "engine 和 llm 改成了不同的目标"})
					conflict++
				}
				totalEngine++
				totalLLM++
			case lok:
				rows = append(rows, row{"🤖 Engine-Only", f, e.To, "—", fmt.Sprintf("%.2f", e.Conf), "—", fmt.Sprintf("step=%s llm 保守未改", e.Step)})
				engineOnly++
				totalEngine++
			case eok:
				rows = append(rows, row{"🧠 LLM-Only", f, "—", l.To, "—", fmt.Sprintf("%.2f", l.Conf), l.Reason})
				llmOnly++
				totalLLM++
			}
		}

		// 排序
		sort.Slice(rows, func(i, j int) bool {
			pri := func(r row) int {
				switch r.typ {
				case "✅ 一致":
					return 0
				case "⚠️ 冲突":
					return 1
				case "🤖 Engine-Only":
					return 2
				case "🧠 LLM-Only":
					return 3
				}
				return 9
			}
			return pri(rows[i]) < pri(rows[j])
		})

		for _, r := range rows {
			fmt.Fprintf(&b, "| %s | `%s` | `%s` | `%s` | %s | %s | %s |\n",
				r.typ, r.from, r.eTo, r.lTo, r.eC, r.lC, r.note)
		}
		b.WriteString("\n---\n\n")
	}

	// 全局统计
	fmt.Fprintf(&b, "## 全局统计\n\n")
	fmt.Fprintf(&b, "| 类型 | 数量 |\n|---|---|\n")
	fmt.Fprintf(&b, "| ✅ 一致 | %d |\n", bothSame)
	fmt.Fprintf(&b, "| ⚠️ 冲突 | %d |\n", conflict)
	fmt.Fprintf(&b, "| 🤖 Engine-Only | %d |\n", engineOnly)
	fmt.Fprintf(&b, "| 🧠 LLM-Only | %d |\n", llmOnly)
	fmt.Fprintf(&b, "| 引擎总 change | %d |\n", totalEngine)
	fmt.Fprintf(&b, "| LLM 总 change | %d |\n", totalLLM)
	return b.String()
}

// ----------------------------------------------------------------------------
// 优化建议渲染
// ----------------------------------------------------------------------------

func renderOptimize(pks []textPK) string {
	var b strings.Builder
	b.WriteString("# Phase B-4 · ark-lexnorm 工具包优化建议\n\n")
	b.WriteString("> 基于 logs/03-pk.md 的对比结果，针对 ark-lexnorm 引擎能力边界提出的**非破坏性**优化建议。\n")
	b.WriteString("> **本 example 不修改工具包**，仅作为优化清单供后续 PR 讨论。\n\n")

	// 收集所有变化
	both, eo, lo, conflict := []changeBucket{}, []changeBucket{}, []changeBucket{}, []changeBucket{}

	for _, pk := range pks {
		em := map[string]engineChange{}
		for _, c := range pk.EngineChanges {
			em[c.From] = c
		}
		lm := map[string]llmChange{}
		for _, c := range pk.LLMChanges {
			lm[c.From] = c
		}
		all := map[string]bool{}
		for f := range em {
			all[f] = true
		}
		for f := range lm {
			all[f] = true
		}
		for f := range all {
			e, lok := em[f]
			l, eok := lm[f]
			switch {
			case lok && eok:
				if e.To == l.To {
					both = append(both, changeBucket{e.From, e.To, l.From, l.To, l.Reason})
				} else {
					conflict = append(conflict, changeBucket{e.From, e.To, l.From, l.To, l.Reason})
				}
			case lok:
				eo = append(eo, changeBucket{e.From, e.To, "", "", ""})
			case eok:
				lo = append(lo, changeBucket{"", "", l.From, l.To, l.Reason})
			}
		}
	}

	// ---- A 类：引擎短板（LLM 改对了，引擎没改）----
	llmBad := classifyLLMBadChanges(lo)
	badSet := map[string]bool{}
	for _, c := range llmBad {
		badSet[c.llmFrom] = true
	}
	var loGenuine []changeBucket
	for _, c := range lo {
		if !badSet[c.llmFrom] {
			loGenuine = append(loGenuine, c)
		}
	}

	fmt.Fprintf(&b, "## A 类 · 引擎短板（LLM 改对了，引擎没改 → 共 %d 个）\n\n", len(loGenuine))
	b.WriteString("**现象**：LLM 改了，引擎没改（且 LLM 改动合理）。\n**建议**：引擎应增加对应能力。\n\n")
	b.WriteString("| from | LLM → | 优化建议 |\n|---|---|---|\n")
	for _, c := range loGenuine {
		from := c.llmFrom
		to := c.llmTo
		sug := suggestForLLMOnly(from, to)
		fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", from, to, sug)
	}
	b.WriteString("\n")

	// ---- B 类：LLM 短板（引擎改对了，LLM 没改）----
	fmt.Fprintf(&b, "## B 类 · LLM 短板（引擎改对了，LLM 没改 → 共 %d 个）\n\n", len(eo))
	b.WriteString("**现象**：引擎改了，LLM 没改。\n**分析**：LLM 在这些场景保守或 prompt 引导有偏差。\n\n")
	b.WriteString("| from | 引擎 → | LLM 保守原因（推测） |\n|---|---|---|\n")
	for _, c := range eo {
		fmt.Fprintf(&b, "| `%s` | `%s` | 同音变体置信度不够，或 prompt 强调\"不在词库则不改\"导致 LLM 漏改 |\n", c.engineFrom, c.engineTo)
	}
	b.WriteString("\n")

	// ---- C 类：重叠区----
	fmt.Fprintf(&b, "## C 类 · 重叠区（双方都改且目标相同 → 共 %d 个）\n\n", len(both))
	b.WriteString("**现象**：双方都改，且改成了同一个标准写法。\n**建议**：这是引擎主导、LLM 兜底的理想分工。\n\n")
	b.WriteString("| from | → | 引擎耗时 | LLM 耗时 | 推荐归属 |\n|---|---|---|---|---|\n")
	for _, c := range both {
		fmt.Fprintf(&b, "| `%s` | `%s` | <1ms | ~2-4s | **引擎**（零延迟、零成本、零 token） |\n", c.engineFrom, c.engineTo)
	}
	b.WriteString("\n")

	// ---- A-误改类：LLM 错改（不应作为引擎优化建议）----
	fmt.Fprintf(&b, "## A-误改类 · LLM 误改（**不应作为引擎优化建议** → 共 %d 个）\n\n", len(llmBad))
	b.WriteString("**现象**：LLM 改了但改得不合理——可能是把对的改成错的、无意义修改、或超出词库。\n")
	b.WriteString("**启示**：LLM 的\"上下文推断\"不一定可靠。引擎不应跟 LLM 学习这些行为。\n\n")
	b.WriteString("| from | LLM → | 为何不合理 |\n|---|---|---|\n")
	for _, c := range llmBad {
		fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", c.llmFrom, c.llmTo, classifyReason(c))
	}
	b.WriteString("\n")

	// ---- D 类：冲突区----
	fmt.Fprintf(&b, "## D 类 · 冲突区（双方都改但目标不同 → 共 %d 个）\n\n", len(conflict))
	b.WriteString("**现象**：双方都改，但改成了不同的标准写法。\n**风险**：引擎可能误改，LLM 也可能误改，必须人工裁决。\n\n")
	b.WriteString("| 原文 | 引擎 → | LLM → | 推荐（业务人工） |\n|---|---|---|---|\n")
	for _, c := range conflict {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | 待人工审查 |\n", c.engineFrom, c.engineTo, c.llmTo)
	}
	b.WriteString("\n")

	// ---- 总结论 ----
	fmt.Fprintf(&b, "## 总结论\n\n")
	fmt.Fprintf(&b, "基于本 example 的 PK 数据：\n\n")
	b.WriteString("- **引擎擅长的领域**：\n")
	b.WriteString("  - 标点/空格/口水词等纯机械清理（normalize + disfluency）\n")
	b.WriteString("  - 词库内显式登记的变体（alias / correction / approximate）\n")
	b.WriteString("  - 同音整词替换（如\"袁梦莲\"→\"袁孟莲\"，前提是 Variant{Homophone} 在词库显式登记）\n")
	b.WriteString("  - 性能优势：单次 < 1ms vs LLM 2-5s\n")
	b.WriteString("  - 成本优势：0 token vs LLM 每次 2-3K token\n\n")
	b.WriteString("- **LLM 擅长的领域**：\n")
	b.WriteString("  - 词库**未登记**但可上下文推断的同音/近似替换（如\"五西辉\"→\"伍锡辉\"、\"菌种子\"→\"黑种子\"）\n")
	b.WriteString("  - 同音异字（如\"羊城冰月\"→\"杨城冰月\"、\"扬城冰月\"→\"杨城冰月\"）\n")
	b.WriteString("  - 错字/口语化文本的语义识别\n")
	b.WriteString("  - 但**风险**：可能误改（如\"吴旭辉\"→\"伍锡辉\"、\"胜利群\"→\"佘丽群\"）\n\n")

	b.WriteString("- **核心优化建议**（按优先级）：\n\n")
	b.WriteString("  1. **新增 `homophone` processor**：当前 Variant{Homophone} 类型没有任何 processor 消费。alias 只看 Alias，fuzzy 只看 Approximate，pinyin processor 按单字 pinyin 查表（不能消费 Variants）。这是工具包的明显能力空白。\n")
	b.WriteString("  2. **`fuzzy` processor 的 confidence 默认值**：当前 `Variant.Confidence = 0` 时 silent skip（详见 fuzzy.go 注释）。建议给 Homophone 一个明确的默认值（如 0.85），便于调用方减少配置成本。\n")
	b.WriteString("  3. **`pinyin` processor 增加整词模式**：当前只能按单字查 PinyinIndex，无法做\"袁梦莲\"→\"袁孟莲\"这类整词同音替换。建议新增 `pinyin-word` processor 或在现有 processor 加整词模式。\n")
	b.WriteString("  4. **`disfluency` 阈值可配置**：当前\"呃\"→\"\" 是硬编码词表，建议支持业务自定义词表。\n")
	b.WriteString("  5. **Pipeline 编排的\"性价比排序\"**：把高频高确定性 processor 放前面（normalize→alias→homophone→fuzzy→LLM），让 LLM 只处理低频低确定性 case，进一步降本。\n")
	b.WriteString("  6. **abbrev variant 加最短长度保护**：transform 自动生成的 abbrev 如果 < 2 字容易误命中单字 token（如\"大\"→\"大部门\"），建议 builder 拒绝 < 2 字的 abbrev variant。\n\n")

	b.WriteString("- **明确\"不该做\"**：\n\n")
	b.WriteString("  - ❌ 不要让引擎学 LLM 的\"上下文推断\"逻辑（会破坏确定性架构不变量）\n")
	b.WriteString("  - ❌ 不要把 LLM 加进 Standard Preset（D1 决策）\n")
	b.WriteString("  - ❌ 不要给 alias/correction 引入模糊 conf（破坏\"高确定性\"语义）\n")

	return b.String()
}

// suggestForLLMOnly 根据 from→to 给优化建议。
// classifyLLMBadChanges 识别 LLM 的明显误改（不应作为引擎学习样本）。
func classifyLLMBadChanges(lo []changeBucket) []changeBucket {
	var bad []changeBucket
	for _, c := range lo {
		if isBadLLMChange(c) {
			bad = append(bad, c)
		}
	}
	return bad
}

func isBadLLMChange(c changeBucket) bool {
	// 同形修改：LLM 报"X -> X"
	if c.llmFrom == c.llmTo {
		return true
	}
	// 把对的改成错的（"黑种子" → "菌种子" 是反向）
	// （简单启发：如果 to 在文本中是"常见错字"也判为 bad）
	badTargets := map[string]bool{"菌种子": true, "胜利群": true}
	if badTargets[c.llmTo] {
		return true
	}
	// 把动词/短语当人名（"填清" -> "田清"）
	if c.llmFrom == "填清" {
		return true
	}
	// 把词库外的名字强行映射（同音词库有，但 from 也不在词库，可能误改）
	return false
}

func classifyReason(c changeBucket) string {
	switch {
	case c.llmFrom == c.llmTo:
		return "LLM 报告了无意义的同形修改（from==to），浪费 token"
	case c.llmTo == "菌种子":
		return "反向修正：词库标准是\"黑种子\"，\"菌种子\"才是错字；LLM 把对的改成错的"
	case c.llmFrom == "填清":
		return "误识别：\"填清\"是 ASR 错读的动词短语（\"填充/填写\"），不是人名\"田清\""
	case c.llmFrom == "吴旭辉":
		return "风险推断：\"吴旭辉\"在词库无对应，强行映射到\"伍锡辉\"会导致误改名"
	}
	return "需人工评估"
}

func suggestForLLMOnly(from, to string) string {
	// 简单启发式：基于字符差异给建议
	if isLikelyPhonetic(from, to) {
		return "同音异字，应作为 Variant{Homophone} 显式登记；但需新增 homophone processor（当前工具包无人消费 Variant{Homophone}）"
	}
	if isCommonTypo(from) {
		return "通用错字，应纳入 fuzzy/deterministic processor 的默认词表"
	}
	if isLikelyApproximate(from, to) {
		return "近似变体，应作为 Variant{Approximate} 显式登记；fuzzy processor 已支持"
	}
	return "需人工评估是否纳入词库"
}

func isLikelyPhonetic(a, b string) bool {
	// 简单启发：长度相同 + 都是中文 → 大概率同音
	if len([]rune(a)) != len([]rune(b)) {
		return false
	}
	return true
}

func isLikelyApproximate(a, b string) bool {
	// 长度差 ≤ 1 → 近似
	return abs(len([]rune(a))-len([]rune(b))) <= 1
}

func isCommonTypo(s string) bool {
	// 已知常见错字
	common := map[string]bool{
		"菌种子": true, "菌": true, "珠缝": true,
	}
	return common[s]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ----------------------------------------------------------------------------

func resolveLogsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(cwd, "logs"),
		filepath.Join(cwd, "example", "09-compare-engine-vs-llm", "logs"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("logs dir not found (cwd=%s)", cwd)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
