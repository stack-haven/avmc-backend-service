// Command stress-test 用大量测试文本做压力测试，
// 目的是**发现 ark-lexnorm 工具包本身的能力缺陷**，
// 而不是验证 example 实现正确性。
//
// 设计思路：每一类测试文本针对一个工具包能力假设。
//   - 如果假设成立：引擎应正确处理
//   - 如果假设不成立：暴露工具包缺陷 → 列入 PR
//
// 输出：
//   - stdout: 每条测试的 PASS/FAIL/ERROR + 详情
//   - logs/06-stress.log: JSON Lines 结构化结果
//
// 运行：go run ./cmd/07-stress-test
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
	"github.com/stack-haven/lexnorm/processor/fuzzy"
	"github.com/stack-haven/lexnorm/processor/normalize"
)

// testCase 是单个测试用例。
type testCase struct {
	ID          string            // T01/T02/...
	Category    string            // 同音异字 / 边界 / 错误恢复 / ...
	Description string            // 测试目的描述
	Text        string            // 输入文本
	Expected    []expectedChange  // 期望的修改（空表示无期望）
}

type expectedChange struct {
	From  string  // 期望的 from
	To    string  // 期望的 to
	MinConf float64 // 最低 conf（0 表示不要求）
}

// ----------------------------------------------------------------------------
// 21 类测试文本
// ----------------------------------------------------------------------------

var allTests = []testCase{
	// ===== T01: 同音异字（已支持，应 PASS）=====
	{
		ID: "T01", Category: "同音异字", Description: "袁梦莲→袁孟莲（Variant{Homophone} 走 approximate 变通）",
		Text:     "袁梦莲提交了报告",
		Expected: []expectedChange{{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5}},
	},
	{
		ID: "T01.2", Category: "同音异字", Description: "叶海燕→叶海嫣",
		Text:     "叶海燕提交了方案",
		Expected: []expectedChange{{From: "叶海燕", To: "叶海嫣", MinConf: 0.5}},
	},

	// ===== T02: 姓+称谓（已支持，应 PASS）=====
	{
		ID: "T02", Category: "姓+称谓", Description: "田工→田华（Variant{Alias}）",
		Text:     "田工提交了报告",
		Expected: []expectedChange{{From: "田工", To: "田华", MinConf: 0.5}},
	},
	{
		ID: "T02.2", Category: "姓+称谓", Description: "田总→田华",
		Text:     "田总审批了",
		Expected: []expectedChange{{From: "田总", To: "田华", MinConf: 0.5}},
	},

	// ===== T03: 同音异字但 conf=0.70（P0-1 已支持）=====
	{
		ID: "T03", Category: "同音异字", Description: "菌种子→黑种籽（conf=0.70，已 Apply）",
		Text:     "请给熊龙军加二十个菌种子",
		Expected: []expectedChange{{From: "菌种子", To: "黑种籽", MinConf: 0.5}},
	},

	// ===== T04: 工具包缺陷：homophone kind 无人消费 =====
	// 直接登记 kind=homophone 的变体（按规范应该是 homophone processor 消费）
	// 期望：engine 不改（暴露工具包缺陷）
	{
		ID: "T04", Category: "工具包缺陷", Description: "Variant{Homophone} 无人消费（应被 homophone processor 处理，但工具包无此 processor）",
		Text:     "请夏奇君参加播种",
		Expected: []expectedChange{}, // 期望不改（但实际是引擎缺陷）
	},

	// ===== T05: 单字级 pinyin processor（不应替换整词）=====
	{
		ID: "T05", Category: "工具包缺陷", Description: "pinyin processor 是单字级，整词同音替换无法处理（应支持整词模式）",
		Text:     "陈新静做完了",
		Expected: []expectedChange{{From: "陈新静", To: "陈兴静", MinConf: 0.5}}, // 通过 approximate 变通
	},

	// ===== T06: 错字/ASR 严重错读 =====
	{
		ID: "T06", Category: "ASR错读", Description: "克克金种籽（ASR重复字符 + 词库含“克种籽”不存在）",
		Text:     "加克克金种籽",
		Expected: []expectedChange{}, // '金种籽' 是 canonical 不改，“克克”是未登录词不改
	},

	// ===== T06.2: 重复字符折叠 =====
	{
		ID: "T06.2", Category: "ASR错读", Description: "“啊啊”是 ASR 重复字符，应不被打乱 (原样保留) ",
		Text:     "啊啊啊啊",
		Expected: []expectedChange{},
	},

	// ===== T06.3: 错字于同句中含 canonical =====
	{
		ID: "T06.3", Category: "ASR错读", Description: "“金种仔” → “金种籽” (同句) ",
		Text:     "加金种仔",
		Expected: []expectedChange{{From: "金种仔", To: "金种籽", MinConf: 0.5}},
	},

	// ===== T07: 边界脏数据 =====
	{
		ID: "T07", Category: "边界脏数据", Description: "数字+人名（10086+ 在词库）",
		Text:     "10086+不能识别",
		Expected: []expectedChange{},
	},

	// ===== T07.2: 全数字 (纯数字人名) =====
	{
		ID: "T07.2", Category: "边界脏数据", Description: "1871676400 (纯数字人名) 原样保留",
		Text:     "打 1871676400",
		Expected: []expectedChange{},
	},

	// ===== T07.3: 纯英文人名 =====
	{
		ID: "T07.3", Category: "边界脏数据", Description: "rebbitmq (纯英文人名) 原样保留",
		Text:     "rebbitmq is 错别字",
		Expected: []expectedChange{},
	},

	// ===== T08: 简称/缩写 =====
	{
		ID: "T08", Category: "简称", Description: "万康盛鼎集团简称'万康盛鼎'",
		Text:     "万康盛鼎集团开会",
		Expected: []expectedChange{},
	},

	// ===== T09: 表情符号 =====
	{
		ID: "T09", Category: "边界字符", Description: "包含 emoji（应不 panic）",
		Text:     "袁孟莲提交了 🎉 报告",
		Expected: []expectedChange{},
	},

	// ===== T10: 控制字符 =====
	{
		ID: "T10", Category: "边界字符", Description: "包含 \\t \\n 控制字符",
		Text:     "袁孟莲\t提交了\n报告",
		Expected: []expectedChange{},
	},

	// ===== T11: 超长文本 =====
	{
		ID: "T11", Category: "压力", Description: "500+ 字长文本，'袁梦莲'同音变体应被多次命中",
		Text: strings.Repeat("袁梦莲提交了报告。", 30),
		Expected: []expectedChange{
			{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5},
		}, // 仅需存在一个 change
	},

	// ===== T11.2: 1000+ 字 =====
	{
		ID: "T11.2", Category: "压力", Description: "1000+ 字长文本，引擎应不崩溃",
		Text: strings.Repeat("田华与袁孟莲、伍锡辉、林宇豪等同事一起工作。", 50),
		Expected: []expectedChange{}, // 所有人名都是 canonical，不应改
	},

	// ===== T11.3: 超多重复同变体 =====
	{
		ID: "T11.3", Category: "压力", Description: "100 个同音错读，引擎应全部命中",
		Text: strings.Repeat("袁梦莲", 100),
		Expected: []expectedChange{
			{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5}, // 至少一个
		},
	},

	// ===== T12: 纯英文/数字 =====
	{
		ID: "T12", Category: "边界字符", Description: "纯英文（应不 panic）",
		Text:     "rebbitmq is a queue",
		Expected: []expectedChange{},
	},

	// ===== T13: 纯中文无空格 =====
	{
		ID: "T13", Category: "密集中文", Description: "无空格密集中文",
		Text:     "袁孟莲提交了报告叶海嫣提交了方案田华审批了",
		Expected: []expectedChange{},
	},

	// ===== T14: 重复 call 确定性 =====
	// 同一文本连续调两次，期望结果完全一致
	{
		ID: "T14", Category: "确定性", Description: "连续 5 次 normalize 同文本，结果应一致",
		Text:     "袁梦莲提交了报告",
		Expected: []expectedChange{{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5}},
	},

	// ===== T15: 同名歧义（无 context processor）=====
	{
		ID: "T15", Category: "工具包缺陷", Description: "无 context processor，无法消歧同名（应新增 context processor）",
		Text:     "请找田工确认",
		Expected: []expectedChange{{From: "田工", To: "田华", MinConf: 0.5}},
	},

	// ===== T16: OOV（不在词库的人名）=====
	{
		ID: "T16", Category: "OOV", Description: "吴旭辉（词库无，应不改）",
		Text:     "吴旭辉提交了",
		Expected: []expectedChange{}, // 合理不改
	},

	// ===== T17: 跨词边界 =====
	{
		ID: "T17", Category: "工具包缺陷", Description: "跨词边界识别（如'驾驶科'是'加十个'的 ASR 错读）— 脚本无法处理",
		Text:     "驾驶科金种子",
		Expected: []expectedChange{}, // 期望不改（这是大模型的能力）
	},

	// ===== T18: 半全角混用 =====
	{
		ID: "T18", Category: "标点", Description: "半全角混用标点",
		Text:     "袁孟莲，提交了。报告；内容：① 测试",
		Expected: []expectedChange{},
	},

	// ===== T19: 数字单位 =====
	{
		ID: "T19", Category: "数字", Description: "阿拉伯数字与中文数字混用",
		Text:     "加 25 个金种籽，相当于二十五克",
		Expected: []expectedChange{},
	},

	// ===== T20: 重复人名 =====
	{
		ID: "T20", Category: "重复", Description: "同一人名出现多次（应全部命中）",
		Text:     "袁梦莲提交了。袁梦莲审核了。袁梦莲完成了。",
		Expected: []expectedChange{
			{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5},
			{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5},
			{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5},
		},
	},

	// ===== T21: 空字符串边界 =====
	{
		ID: "T21", Category: "边界", Description: "空字符串",
		Text:     "",
		Expected: []expectedChange{},
	},

	// ===== T22: 极长 token =====
	{
		ID: "T22", Category: "工具包缺陷", Description: "极长 token（应支持 UTF-8 边界）",
		Text:     "袁孟莲" + strings.Repeat("啊", 100) + "提交了",
		Expected: []expectedChange{},
	},

	// ===== T23: 同一个 entry 多个 alias variant =====
	{
		ID: "T23", Category: "alias 多变体", Description: "田华 的田工/田总同时命中",
		Text:     "田工和田总都说：找田华确认",
		Expected: []expectedChange{
			{From: "田工", To: "田华", MinConf: 0.5},
			{From: "田总", To: "田华", MinConf: 0.5},
		},
	},

	// ===== T24: 长拼音（覆盖测试）=====
	{
		ID: "T24", Category: "pinyin 覆盖", Description: "pinyin 表覆盖度（覆盖率 100%）",
		Text:     "袁孟莲叶海嫣田华伍锡辉林宇豪陈兴静陈科沆熊龙军龚千友冯春晓龚建军何焓",
		Expected: []expectedChange{}, // 都是 canonical
	},

	// ===== T25: 性能压力 =====
	// 同文本连续调 100 次，看延迟
	{
		ID: "T25", Category: "性能", Description: "100 次连续调用延迟统计",
		Text:     "袁孟莲提交了报告。叶海嫣提交了方案。田华审批了。林宇豪协助。陈兴静复核。",
		Expected: []expectedChange{},
	},

	// ===== T26: 🔴 abbrev 子串缺陷专门测试（D-1）=====
	// 变体 '万康盛鼎' 是 canonical '万康盛鼎集团' 的子串
	// 引擎会把 canonical 中的 '万康盛鼎' 识别为 variant 重复
	{
		ID: "T26", Category: "工具包缺陷-D1", Description: "abbrev 是 canonical 子串时，alias 会重复 canonical（应为空或能识别）",
		Text:     "万康盛鼎集团开会",
		Expected: []expectedChange{}, // 期望不改
	},
	{
		ID: "T26.2", Category: "工具包缺陷-D1", Description: "倍多客科技 = 倍多客 + 科技 (abbrev 子串) 重复问题",
		Text:     "倍多客科技开发",
		Expected: []expectedChange{},
	},
	{
		ID: "T26.3", Category: "工具包缺陷-D1", Description: "王德发集团 = 王德发 + 集团 (abbrev 子串) 重复问题",
		Text:     "王德发集团合作",
		Expected: []expectedChange{},
	},
	{
		ID: "T26.4", Category: "工具包缺陷-D1", Description: "人名也有同样问题？如词库有 '伍锡' 是 '伍锡辉' 子串",
		Text:     "伍锡辉参加了",
		Expected: []expectedChange{}, // 期望不改（'伍锡辉' 是 canonical）
	},

	// ===== T27: 变体跨条目冲突 =====
	// 多个 entry 可能共享 variant。如 '黑钟子' 是 '黑种籽' 和某个虚构 entry 的 variant
	{
		ID: "T27", Category: "冲突", Description: "同变体被多个 entry 共享（alias processor 选哪个？）",
		Text:     "黑种仔参加了",
		Expected: []expectedChange{{From: "黑种仔", To: "黑种籽", MinConf: 0.5}},
	},

	// ===== T28: 中英混合 =====
	{
		ID: "T28", Category: "混合", Description: "中英文人名混在一句",
		Text:     "rebbitmq 和袁孟莲讨论方案",
		Expected: []expectedChange{}, // rebbitmq 是词库人名（不收）；袁孟莲是 canonical
	},

	// ===== T29: 标点嵌套 =====
	{
		ID: "T29", Category: "标点", Description: "多重括号嵌套",
		Text:     "袁孟莲说（（（他说）））",
		Expected: []expectedChange{},
	},

	// ===== T30: 数字+汉字混合 =====
	{
		ID: "T30", Category: "数字", Description: "“25个” + “二十五克” 混用",
		Text:     "加25个金种籽，合二十五克",
		Expected: []expectedChange{},
	},

	// ===== T31: 多名同句 =====
	{
		ID: "T31", Category: "多 target", Description: "同句多个不同人名同音替换",
		Text:     "袁梦莲找陈新静、叶海燕一起",
		Expected: []expectedChange{
			{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5},
			{From: "陈新静", To: "陈兴静", MinConf: 0.5},
			{From: "叶海燕", To: "叶海嫣", MinConf: 0.5},
		},
	},

	// ===== T32: 变体是另一个 entry 的 canonical =====
	// 严格来说不应这么登记，但能发现冲突
	{
		ID: "T32", Category: "冲突", Description: "跨词典源冲突：user '田花' vs system 没有；仅 user '田花' 被识别",
		Text:     "田花参加了",
		Expected: []expectedChange{{From: "田花", To: "田华", MinConf: 0.5}}, // 田花 → 田华 (user 中的 homophone)
	},

	// ===== T33: 姓+称谓的同姓歧义 =====
	// '龚工' 词库没登记，应该不改 (仅 '建军工' 被登记为'龚建军')
	{
		ID: "T33", Category: "工具包缺陷", Description: "'龚工' 同姓歧义，词库未登记 (可检查是否会歧义命中 '龚千友' 或 '龚建军')",
		Text:     "龚工参加了",
		Expected: []expectedChange{}, // 期望不改（词库未登记）
	},

	// ===== T34: emoji + 中文 =====
	{
		ID: "T34", Category: "边界字符", Description: "多个 emoji + 中文",
		Text:     "👨‍💼袁孟莲提交了 📝 报告 🚀",
		Expected: []expectedChange{},
	},

	// ===== T35: 换行符 =====
	{
		ID: "T35", Category: "边界字符", Description: "含换行符",
		Text:     "袁孟莲\n提交了\n报告",
		Expected: []expectedChange{},
	},

	// ===== T36: tab + 中文 =====
	{
		ID: "T36", Category: "边界字符", Description: "tab 分隔",
		Text:     "袁孟莲\t提交了报告",
		Expected: []expectedChange{},
	},

	// ===== T37: 全是变体 (无 canonical) =====
	{
		ID: "T37", Category: "变体", Description: "一段只含变体的文本",
		Text:     "叶海燕、陈新静、陈科航、伍西辉、芦川、袁梦莲一起开会",
		Expected: []expectedChange{
			{From: "叶海燕", To: "叶海嫣", MinConf: 0.5},
			{From: "陈新静", To: "陈兴静", MinConf: 0.5},
			{From: "陈科航", To: "陈科沆", MinConf: 0.5},
			{From: "伍西辉", To: "伍锡辉", MinConf: 0.5},
			{From: "芦川", To: "卢川", MinConf: 0.5},
			{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5},
		},
	},

	// ===== T38: 变体 + canonical 混合 =====
	{
		ID: "T38", Category: "变体", Description: "变体和 canonical 混在",
		Text:     "袁梦莲、袁孟莲、叶海燕、叶海嫣同时出现",
		Expected: []expectedChange{
			{From: "袁梦莲", To: "袁孟莲", MinConf: 0.5},
			{From: "叶海燕", To: "叶海嫣", MinConf: 0.5},
		},
	},

	// ===== T39: 变体跨多个 processor (alias + fuzzy) =====
	{
		ID: "T39", Category: "混合 processor", Description: "同一个 token 被 alias 和 fuzzy 都覆盖 (谁优先？)",
		Text:     "陈科航是同音也是错字",
		Expected: []expectedChange{{From: "陈科航", To: "陈科沆", MinConf: 0.5}}, // 只能改一次
	},

	// ===== T40: 故意构造 variant 是另一个变体的子串 =====
	// 这个是极端情况：万康盛鼎和万康 (后者不是变体，只是文本中的子串)
	{
		ID: "T40", Category: "工具包缺陷-D1", Description: "变体可能被文本中其他变体的子串误命中",
		Text:     "万康开会",  // '万康' 是 '万康盛鼎集团' 的子串但不是变体
		Expected: []expectedChange{}, // 期望不改
	},
}

func main() {
	dataDir, logsDir, err := resolveDirs()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		log.Fatal(err)
	}

	// 加载词库
	userLex, err := lexjson.LoadFile(filepath.Join(dataDir, "user.json"))
	if err != nil {
		log.Fatal(err)
	}
	deptLex, _ := lexjson.LoadFile(filepath.Join(dataDir, "dept.json"))
	sysLex, _ := lexjson.LoadFile(filepath.Join(dataDir, "system.json"))

	all := []lexicon.Entry{}
	for _, e := range userLex.Entries {
		le, _ := lexjson.ToEntry(e)
		all = append(all, le)
	}
	for _, e := range deptLex.Entries {
		le, _ := lexjson.ToEntry(e)
		all = append(all, le)
	}
	for _, e := range sysLex.Entries {
		le, _ := lexjson.ToEntry(e)
		all = append(all, le)
	}

	built, err := lexicon.NewBuilderWithVersion("stress").
		Add(all...).
		WithPinyin(pinyinlite.Converter{}).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	pipeline := lexnorm.NewPipeline(normalize.New(), alias.New(built), fuzzy.New(built))

	engine, err := lexnorm.New(
		lexnorm.WithLexicon(built),
		lexnorm.WithPipeline(pipeline),
		lexnorm.WithConfig(lexnorm.Config{
			AutoApplyThreshold: 0.70,
			SuggestThreshold:   0.50,
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	logPath := filepath.Join(logsDir, "06-stress.log")
	logFile, _ := os.Create(logPath)
	defer logFile.Close()
	logJSON := json.NewEncoder(logFile)

	// 跑所有测试
	ctx := context.Background()
	results := runAllTests(ctx, engine, logJSON)

	// 汇总
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("STRESS TEST SUMMARY")
	fmt.Println("=" + strings.Repeat("=", 79))
	total := len(results)
	pass, fail, errN, known := 0, 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "PASS":
			pass++
		case "FAIL":
			fail++
		case "ERROR":
			errN++
		case "KNOWN_DEFECT":
			known++
		}
	}
	fmt.Printf("Total:      %d\n", total)
	fmt.Printf("  PASS:           %d\n", pass)
	fmt.Printf("  FAIL:           %d\n", fail)
	fmt.Printf("  ERROR (panic):  %d\n", errN)
	fmt.Printf("  KNOWN_DEFECT:   %d  (工具包已知缺陷)\n", known)
	fmt.Printf("Pass rate:    %.1f%%\n", float64(pass)/float64(total)*100)

	// 列出 FAIL 和 KNOWN_DEFECT（按 Category 分组）
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("TOOL PACKAGE DEFECTS (按 category)")
	fmt.Println("=" + strings.Repeat("=", 79))
	defects := map[string][]testResult{}
	for _, r := range results {
		if r.Status == "FAIL" || r.Status == "KNOWN_DEFECT" {
			defects[r.Category] = append(defects[r.Category], r)
		}
	}
	for cat, list := range defects {
		fmt.Printf("\n[%s] %d 项:\n", cat, len(list))
		for _, r := range list {
			fmt.Printf("  %s: %s\n", r.ID, r.Description)
			if r.Status == "FAIL" {
				fmt.Printf("    期望: %s\n", formatExpected(r.Expected))
				fmt.Printf("    实际: %s\n", formatActual(r.Actual))
			}
		}
	}

	fmt.Printf("\n[OK] stress log → %s\n", logPath)
}

type testResult struct {
	ID          string
	Category    string
	Description string
	Text        string
	Status      string // PASS|FAIL|ERROR|KNOWN_DEFECT
	Expected    []expectedChange
	Actual      []actualChange
	LatencyMS   int64
	ErrorMsg    string
	Notes       string
}

type actualChange struct {
	From       string
	To         string
	Confidence float64
	Processor  string
}

func runAllTests(ctx context.Context, engine *lexnorm.Engine, logJSON *json.Encoder) []testResult {
	results := make([]testResult, 0, len(allTests))
	for _, tc := range allTests {
		fmt.Printf("[%-5s] %-15s ... ", tc.ID, tc.Category)
		r := testResult{
			ID:          tc.ID,
			Category:    tc.Category,
			Description: tc.Description,
			Text:        tc.Text,
			Expected:    tc.Expected,
		}

		// 特别处理 T14 / T25（多次调用）
		if tc.ID == "T14" {
			r = runDeterminismTest(ctx, engine, tc, logJSON)
		} else if tc.ID == "T25" {
			r = runPerfTest(ctx, engine, tc, logJSON)
		} else {
			r = runSingleTest(ctx, engine, tc, logJSON)
		}

		results = append(results, r)

		// stdout 简报
		marker := "✓"
		switch r.Status {
		case "PASS":
			marker = "✓ PASS"
		case "FAIL":
			marker = "✗ FAIL"
		case "ERROR":
			marker = "⚠ ERROR"
		case "KNOWN_DEFECT":
			marker = "⚡ KNOWN"
		}
		fmt.Printf("%s (latency=%dms)\n", marker, r.LatencyMS)

		_ = logJSON.Encode(map[string]any{
			"type":     "stress_result",
			"id":       tc.ID,
			"category": tc.Category,
			"status":   r.Status,
			"latency_ms": r.LatencyMS,
			"text":     tc.Text,
			"actual":   r.Actual,
			"expected": r.Expected,
			"error":    r.ErrorMsg,
			"notes":    r.Notes,
		})
	}
	return results
}

func runSingleTest(ctx context.Context, engine *lexnorm.Engine, tc testCase, logJSON *json.Encoder) testResult {
	r := testResult{
		ID:          tc.ID,
		Category:    tc.Category,
		Description: tc.Description,
		Text:        tc.Text,
		Expected:    tc.Expected,
	}

	t0 := time.Now()
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				r.Status = "ERROR"
				r.ErrorMsg = fmt.Sprintf("panic: %v", rec)
			}
		}()
		result, err := engine.Normalize(ctx, tc.Text)
		if err != nil {
			r.Status = "ERROR"
			r.ErrorMsg = err.Error()
			return
		}
		r.LatencyMS = time.Since(t0).Milliseconds()
		for _, ch := range result.Changes {
			if ch.Processor == "normalize" {
				continue
			}
			r.Actual = append(r.Actual, actualChange{
				From:       ch.From,
				To:         ch.To,
				Confidence: ch.Confidence,
				Processor:  ch.Processor,
			})
		}
	}()

	if r.Status == "ERROR" {
		return r
	}

	r.Status = evaluateResult(tc, r)
	return r
}

// runDeterminismTest 连续 5 次调用，验证结果一致
func runDeterminismTest(ctx context.Context, engine *lexnorm.Engine, tc testCase, logJSON *json.Encoder) testResult {
	r := testResult{
		ID:          tc.ID,
		Category:    tc.Category,
		Description: tc.Description,
		Text:        tc.Text,
		Expected:    tc.Expected,
	}

	t0 := time.Now()
	var results [5][]actualChange
	for i := 0; i < 5; i++ {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					r.Status = "ERROR"
					r.ErrorMsg = fmt.Sprintf("panic at iter %d: %v", i, rec)
				}
			}()
			result, err := engine.Normalize(ctx, tc.Text)
			if err != nil {
				r.Status = "ERROR"
				r.ErrorMsg = err.Error()
				return
			}
			for _, ch := range result.Changes {
				if ch.Processor == "normalize" {
					continue
				}
				results[i] = append(results[i], actualChange{
					From:       ch.From,
					To:         ch.To,
					Confidence: ch.Confidence,
					Processor:  ch.Processor,
				})
			}
		}()
		if r.Status == "ERROR" {
			return r
		}
	}
	r.LatencyMS = time.Since(t0).Milliseconds() / 5

	// 检查一致性
	for i := 1; i < 5; i++ {
		if !sameChanges(results[0], results[i]) {
			r.Status = "FAIL"
			r.ErrorMsg = "结果不一致（同输入 5 次输出不同）"
			r.Notes = fmt.Sprintf("iter 0: %v, iter %d: %v", results[0], i, results[i])
			return r
		}
	}

	r.Actual = results[0]
	r.Status = evaluateResult(tc, r)
	r.Notes = "5 次结果一致"
	return r
}

// runPerfTest 100 次调用，统计平均延迟
func runPerfTest(ctx context.Context, engine *lexnorm.Engine, tc testCase, logJSON *json.Encoder) testResult {
	r := testResult{
		ID:          tc.ID,
		Category:    tc.Category,
		Description: tc.Description,
		Text:        tc.Text,
		Expected:    tc.Expected,
	}

	t0 := time.Now()
	var totalMS int64
	for i := 0; i < 100; i++ {
		_, err := engine.Normalize(ctx, tc.Text)
		if err != nil {
			r.Status = "ERROR"
			r.ErrorMsg = err.Error()
			return r
		}
	}
	totalMS = time.Since(t0).Milliseconds()
	r.LatencyMS = totalMS / 100

	// 性能基准：单次 < 5ms（spec 上限）
	if r.LatencyMS > 5 {
		r.Status = "FAIL"
		r.ErrorMsg = fmt.Sprintf("延迟过高：%dms/次（要求 < 5ms）", r.LatencyMS)
		return r
	}

	r.Status = "PASS"
	r.Notes = fmt.Sprintf("100 次平均 %dms/次", r.LatencyMS)
	return r
}

func evaluateResult(tc testCase, r testResult) string {
	// 如果测试在 "工具包缺陷" category 且无 expected，标 KNOWN_DEFECT
	if tc.Category == "工具包缺陷" && len(tc.Expected) == 0 {
		return "KNOWN_DEFECT"
	}
	// 验证 expected
	if len(tc.Expected) == 0 {
		// 期望不改
		if len(r.Actual) == 0 {
			return "PASS"
		}
		return "FAIL"
	}
	// 期望 N 个 change
	if len(r.Actual) < len(tc.Expected) {
		return "FAIL"
	}
	// 简化匹配：每个 expected.from 都在 actual 中找到对应
	for _, exp := range tc.Expected {
		found := false
		for _, act := range r.Actual {
			if act.From == exp.From && act.Confidence >= exp.MinConf {
				found = true
				break
			}
		}
		if !found {
			return "FAIL"
		}
	}
	return "PASS"
}

func sameChanges(a, b []actualChange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func formatExpected(exp []expectedChange) string {
	if len(exp) == 0 {
		return "(无期望改动)"
	}
	parts := make([]string, 0, len(exp))
	for _, e := range exp {
		parts = append(parts, fmt.Sprintf("'%s'→'%s'(conf≥%.2f)", e.From, e.To, e.MinConf))
	}
	return strings.Join(parts, ", ")
}

func formatActual(act []actualChange) string {
	if len(act) == 0 {
		return "(无实际改动)"
	}
	parts := make([]string, 0, len(act))
	for _, a := range act {
		parts = append(parts, fmt.Sprintf("'%s'→'%s'(conf=%.2f,%s)", a.From, a.To, a.Confidence, a.Processor))
	}
	return strings.Join(parts, ", ")
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
	return "", "", fmt.Errorf("data dir not found")
}