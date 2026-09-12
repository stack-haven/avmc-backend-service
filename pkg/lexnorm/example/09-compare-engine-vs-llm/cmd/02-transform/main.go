// Command transform 把原始 curl 响应转换为 ark-lexnorm 可消费的词库 JSON。
//
// 步骤：
//  1. 读 data/dept.raw.json    → 字段裁剪、拼音生成 → data/dept.json
//  2. 读 data/member.raw.json  → 启发式过滤（保留真实人名 + ~20% 边界脏数据）→ data/user.json
//  3. 从测试文本中挖掘业务术语（硬编码清单 + 频率统计）→ data/system.json
//
// 设计目标：
//   - 不写入敏感字段（手机号、头像、userId、createTime、email）
//   - 保留少量边界脏数据（"10086+", "rebbitmq", "测试123456"）作为引擎鲁棒性测试用例
//   - 自动生成常见同音变体（用于 fuzzy matching 兜底）
//
// 运行：go run ./cmd/02-transform
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/stack-haven/lexnorm/example/09-compare-engine-vs-llm/internal/pinyinlite"
)

// ----------------------------------------------------------------------------
// JSON schema（与 PLAN.md §2 一致）
// ----------------------------------------------------------------------------

type VariantJSON struct {
	Text       string  `json:"text"`
	Kind       string  `json:"kind"` // alias|correction|homophone|approximate
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source,omitempty"`
}

type EntryJSON struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Pinyin   string         `json:"pinyin,omitempty"`
	Variants []VariantJSON  `json:"variants"`
	Meta     map[string]any `json:"meta,omitempty"`
}

type LexiconJSON struct {
	Version string         `json:"version"`
	Source  string         `json:"source"`
	Entries []EntryJSON    `json:"entries"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// ----------------------------------------------------------------------------
// raw data
// ----------------------------------------------------------------------------

type deptRaw struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	ParentID json.RawMessage `json:"parentId"` // 可能是 string 或 number
	Sort     int             `json:"sort"`
	Status   int             `json:"status"`
}

type memberRaw struct {
	ID       string `json:"id"`
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	OrgID    string `json:"orgId"`
	DeptName string `json:"deptName"`
	Mobile   string `json:"mobile"`
}

// ----------------------------------------------------------------------------
// 启发式分类
// ----------------------------------------------------------------------------

var (
	reDigitOrSym = regexp.MustCompile(`^[\d+]+$`)
	reEnglish    = regexp.MustCompile(`^[A-Za-z]+$`)
	reMixed      = regexp.MustCompile(`[A-Za-z]`)
	reHasTestKw  = regexp.MustCompile(`(测试|入职|员工|管理|账号|哒哒|动态|野原)`)
	reHasSep     = regexp.MustCompile(`[·.・]`)
	reIsAllHan   = regexp.MustCompile(`^[\x{4e00}-\x{9fff}]+$`)
)

// nameKind 分类
type nameKind int

const (
	kindRealName    nameKind = iota // 真实中文人名（2~4 字）
	kindLongZh                      // 长中文（非人名短语）
	kindSepZh                       // 含分隔符
	kindEnglish                     // 纯英文
	kindDigitOrSym                  // 纯数字/符号
	kindMixed                       // 中英混合
	kindTestAccount                 // 测试/入职账号
)

func classify(name string) nameKind {
	n := strings.TrimSpace(name)
	if n == "" {
		return -1
	}
	if reDigitOrSym.MatchString(n) {
		return kindDigitOrSym
	}
	if reEnglish.MatchString(n) {
		return kindEnglish
	}
	if reHasSep.MatchString(n) {
		return kindSepZh
	}
	if reMixed.MatchString(n) {
		return kindMixed
	}
	if reHasTestKw.MatchString(n) {
		return kindTestAccount
	}
	if reIsAllHan.MatchString(n) && len([]rune(n)) > 4 {
		return kindLongZh
	}
	if reIsAllHan.MatchString(n) && len([]rune(n)) >= 2 && len([]rune(n)) <= 4 {
		return kindRealName
	}
	// 含中文+数字（"于云海2号"等）→ kindMixed 边界脏数据
	if len([]rune(n)) >= 2 && strings.ContainsAny(n, "0123456789") {
		return kindMixed
	}
	return -1
}

// ----------------------------------------------------------------------------
// dept → entries
// ----------------------------------------------------------------------------

func transformDept(raw []deptRaw, version string) []EntryJSON {
	// 按 parent 关系计算 path
	id2dept := make(map[string]deptRaw, len(raw))
	for _, d := range raw {
		id2dept[d.ID] = d
	}

	parentID := func(d deptRaw) string {
		var s string
		if err := json.Unmarshal(d.ParentID, &s); err == nil {
			return s
		}
		var n json.Number
		if err := json.Unmarshal(d.ParentID, &n); err == nil {
			return n.String()
		}
		return ""
	}

	buildPath := func(d deptRaw) string {
		parts := []string{d.Name}
		cur := parentID(d)
		depth := 0
		for cur != "" && cur != "0" && depth < 8 {
			p, ok := id2dept[cur]
			if !ok {
				break
			}
			parts = append([]string{p.Name}, parts...)
			cur = parentID(p)
			depth++
		}
		return strings.Join(parts, "/")
	}

	out := make([]EntryJSON, 0, len(raw))
	seenText := make(map[string]int) // text → index in out
	for _, d := range raw {
		name := strings.TrimSpace(d.Name)
		if name == "" {
			continue
		}
		pid := parentID(d)
		py := pinyinlite.Convert(name)
		abbrev := pickAbbrev(name)
		path := buildPath(d)

		if existingIdx, exists := seenText[name]; exists {
			// 同名 dept：保留第一个，把后续作为 variant（或 path 别名）
			pathAlias := strings.Replace(path, "/", "·", -1)
			if pathAlias == name {
				pathAlias = name + "·子部门"
			}
			if idx := existingIdx; idx < len(out) {
				out[idx].Variants = append(out[idx].Variants, VariantJSON{
					Text:       pathAlias,
					Kind:       "alias",
					Confidence: 0.95,
					Source:     "auto-path-disambig",
				})
			}
			continue
		}

		variants := []VariantJSON{}
		if abbrev != "" && abbrev != name {
			variants = append(variants, VariantJSON{
				Text: abbrev, Kind: "alias", Confidence: 0.9, Source: "auto-abbrev",
			})
		}
		entry := EntryJSON{
			ID:       "dept-" + d.ID,
			Text:     name,
			Pinyin:   py,
			Variants: variants,
			Meta: map[string]any{
				"parent_id": pid,
				"path":      path,
				"sort":      d.Sort,
			},
		}
		seenText[name] = len(out)
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// pickAbbrev 从中文名挑常见缩写（去掉"集团/科技/公司/部门"等）。
// 返回 abbrev 必须 >= 2 个汉字，避免被 Aho-Corasick 误命中单字 token。
func pickAbbrev(name string) string {
	for _, suffix := range []string{"集团", "科技有限公司", "科技", "公司", "部门", "组"} {
		if strings.HasSuffix(name, suffix) && len([]rune(name)) > len([]rune(suffix)) {
			ab := strings.TrimSuffix(name, suffix)
			if len([]rune(ab)) >= 2 {
				return ab
			}
			return ""
		}
	}
	return ""
}

// ----------------------------------------------------------------------------
// member → entries（含启发式过滤）
// ----------------------------------------------------------------------------

func transformMember(raw []memberRaw, version string) []EntryJSON {
	// 第一轮：分类
	type classified struct {
		raw  memberRaw
		kind nameKind
		keep bool
	}

	classifieds := make([]classified, 0, len(raw))
	kindCount := map[nameKind]int{}
	for _, m := range raw {
		k := classify(m.Name)
		if k < 0 {
			continue
		}
		kindCount[k]++
		cls := classified{raw: m, kind: k}
		// 保留规则
		switch k {
		case kindRealName, kindLongZh, kindSepZh, kindMixed:
			cls.keep = true
		case kindDigitOrSym, kindEnglish:
			cls.keep = true // 边界脏数据，全部保留
		case kindTestAccount:
			// 50% 保留作边界挑战
			cls.keep = (rand.Intn(2) == 0)
		}
		classifieds = append(classifieds, cls)
	}

	kept, skipped := 0, 0
	out := make([]EntryJSON, 0, len(classifieds))
	seen := make(map[string]bool)

	for _, c := range classifieds {
		if !c.keep {
			skipped++
			continue
		}
		name := strings.TrimSpace(c.raw.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		// 构造 ID：dept_id + name 联合，避免同名歧义
		id := "user-" + c.raw.OrgID + "-" + sanitizeID(name)
		py := pinyinlite.Convert(name)
		variants := buildExplicitHomophones(name)
		variants = append(variants, buildExplicitTitles(name)...)
		out = append(out, EntryJSON{
			ID:       id,
			Text:     name,
			Pinyin:   py,
			Variants: variants,
			Meta: map[string]any{
				"dept_id":   c.raw.OrgID,
				"dept_name": c.raw.DeptName,
				"kind":      kindName(c.kind),
				"is_dirty":  c.kind != kindRealName,
			},
		})
		kept++
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Text < out[j].Text })

	fmt.Printf("member transform: total=%d kept=%d skipped=%d\n", len(classifieds), kept, skipped)
	fmt.Printf("  kind distribution: %v\n", kindCount)
	return out
}

// explicitHomophones 是手工登记的同音人名对。
//
// 策略上只登记**一半**已知同音对——
//   - 登记的：引擎会改；用于 Phase B 观察"引擎 vs LLM 是否一致"
//   - 未登记的（如夏奇君/夏其军）：引擎不改；用于观察 LLM 的边界判断
//
// 这是 Phase B PK 的核心设计：让两端面对**不同子集**的真实人名，
// 才能看出谁更激进、谁更保守。
func explicitHomophones(canonical string) []VariantJSON {
	pairs := map[string][]struct {
		text string
		conf float64
	}{
		"叶海嫣": {{"叶海燕", 0.95}},
		"袁孟莲": {{"袁梦莲", 0.95}},
		"陈兴静": {{"陈新静", 0.95}},
		"陈科沆": {{"陈科航", 0.95}},
		"伍锡辉": {{"伍西辉", 0.95}, {"五西辉", 0.95}}, // P0-3: 补 “五西辉”（五↔伍同音近似）
		"卢川":  {{"芦川", 0.95}},
		"邓梓":  {{"邓子", 0.95}},
		"田清":  {{"田青", 0.95}},
		"田华":  {{"田花", 0.70}}, // 边界 Suggest（仅提示不替换）
		// 故意不登记：
		//   - 夏奇君 ↔ 夏其军：不同人，LLM 应保留
		//   - 杨行宇 → ?：无明显对应，LLM 应判为不确定
		//   - 胜利群 → 佘丽群：距离大，模糊边界
	}
	vs, ok := pairs[canonical]
	if !ok {
		return nil
	}
	out := make([]VariantJSON, 0, len(vs))
	for _, p := range vs {
		out = append(out, VariantJSON{
			Text:       p.text,
			Kind:       "approximate",
			Confidence: p.conf,
			Source:     "manual-homophone",
		})
	}
	return out
}

func buildExplicitHomophones(canonical string) []VariantJSON {
	return explicitHomophones(canonical)
}

// explicitTitles 是手工登记的“姓+称谓”别名变体。
//
// 业务场景：员工在口头 / ASR 转写中被叫“田工”、“张总”、“李老师”等，
// 需要被引擎映射回真实姓名“田华”、“张三”、“李四”。
//
// 实现：手工为 TOP 高频人名手工登记 variant，kind=alias，conf=0.85。
// 这是“工具包原生能力”——alias processor 会扫描所有 Variant{Alias} 并 Aho-Corasick 命中。
//
// 风险提示：
//   - 同姓歧义（公司有两个“张三”）：文本上下文中有部门/项目名时才能消歧
//   - 本例词库以 dept_id + name 作为联合 ID，实际生产可加 (dept_id, surname) 索引
func explicitTitles(canonical string) []VariantJSON {
	pairs := map[string][]VariantJSON{
		// "田华" ← "田工" / "田总"
		"田华": {
			{Text: "田工", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
			{Text: "田总", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
		},
		// "伍锡辉" ← "伍工" / "伍总"
		"伍锡辉": {
			{Text: "伍工", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
			{Text: "伍总", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
		},
		// "林宇豪" ← "林工" / "林总"
		"林宇豪": {
			{Text: "林工", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
			{Text: "林总", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
		},
		// "龚千友" ← "龚工" / "龚总"
		"龚千友": {
			{Text: "龚工", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
			{Text: "龚总", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
		},
		// "冯春晓" ← "冯工" / "冯总"
		"冯春晓": {
			{Text: "冯工", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
			{Text: "冯总", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
		},
		// "于云海" ← "于工" / "于总"
		"于云海": {
			{Text: "于工", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
			{Text: "于总", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
		},
		// "龚建军" ← "龚工" / "龚总"（注意同姓不同人，未消歧）
		"龚建军": {
			{Text: "建军工", Kind: "alias", Confidence: 0.85, Source: "manual-title"},
		},
	}
	vs, ok := pairs[canonical]
	if !ok {
		return nil
	}
	return vs
}

func buildExplicitTitles(canonical string) []VariantJSON {
	return explicitTitles(canonical)
}

func kindName(k nameKind) string {
	switch k {
	case kindRealName:
		return "real-name"
	case kindLongZh:
		return "long-zh"
	case kindSepZh:
		return "sep-zh"
	case kindEnglish:
		return "english"
	case kindDigitOrSym:
		return "digit-or-sym"
	case kindMixed:
		return "mixed"
	case kindTestAccount:
		return "test-account"
	}
	return "unknown"
}

func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || (r >= 0x4e00 && r <= 0x9fff) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// ----------------------------------------------------------------------------
// 业务术语挖掘（硬编码清单 + 在文本中统计频次）
// ----------------------------------------------------------------------------

// 业务术语清单（手工从两条文本中提取的高价值词）。
//
// system.json 的设计意图：**模拟业务术语的常见 ASR 错读变体**。
// 每条术语至少配 N 个 ASR 错读 variant，让 fuzzy processor 能命中。
//
// Variant 置信度策略：
//   - 0.95：明确错字（子↔仔/种↔钟/明↔名等同音或近似）
//   - 0.85：模糊变体（同音但跨语义边界）
//   - 0.70：仅 Suggest（仅记录不自动替换，audit 用）
type businessTerm struct {
	ID       string
	Text     string
	Category string
	Variants []VariantJSON
}

var businessTerms = []businessTerm{
	// 奖励体系
	{
		"term-jinzhongzi", "金种籽", "reward", // ⚠️ 项目偏好：canonical = 金种籽
		[]VariantJSON{
			{Text: "金种仔", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 子↔仔
			{Text: "金种子", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 旧写法作为 variant（项目偏好保留）
			{Text: "金钟子", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 钟↔种
			{Text: "金种资", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 资↔子
			{Text: "紧种子", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 紧↔金
		},
	},
	{
		"term-heizhongzi", "黑种籽", "reward", // ⚠️ 项目偏好：canonical = 黑种籽
		[]VariantJSON{
			{Text: "黑种仔", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"},
			{Text: "黑种子", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 旧写法作为 variant
			{Text: "黑钟子", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"},
			{Text: "菌种子", Kind: "approximate", Confidence: 0.70, Source: "manual-asr"}, // 黑↔菌 不同音但 ASR 会错读
			{Text: "嘿种子", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 黑↔嘿 近音
		},
	},
	// 课程 / 项目
	{
		"term-bozhong", "播种", "course",
		[]VariantJSON{
			{Text: "拨种", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"},  // 拨↔播
			{Text: "博种", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"},  // 博↔播
			{Text: "搏种", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"},  // 搏↔播
			{Text: "播种类", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 多字
		},
	},
	{
		"term-zhaobiaotoubiao", "招投标", "project",
		[]VariantJSON{
			{Text: "投招标", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 倒序
			{Text: "召标投", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 召↔招
			{Text: "招表投", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 表↔标
		},
	},
	{
		"term-zhongbiao", "中标", "project",
		[]VariantJSON{
			{Text: "重标", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 重↔中
			{Text: "终标", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 终↔中
			{Text: "中镖", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 镖↔标
			{Text: "中表", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 表↔标
		},
	},
	{
		"term-gongyingshang", "供应商", "project",
		[]VariantJSON{
			{Text: "共应商", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 共↔供
			{Text: "供应伤", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 伤↔商
			{Text: "供应上", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 上↔商
		},
	},
	{
		"term-shijianka", "实践卡", "course",
		[]VariantJSON{
			{Text: "时间卡", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 间↔践 同音
			{Text: "实践咖", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 咖↔卡
			{Text: "十件卡", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 十↔实 近音
		},
	},
	// 工具/系统
	{
		"term-ruanjian", "软件", "tool",
		[]VariantJSON{
			{Text: "软见", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 见↔件
			{Text: "软减", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 减↔件
			{Text: "软健", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 健↔件
		},
	},
	{
		"term-pptmoban", "PPT模板", "tool",
		[]VariantJSON{
			{Text: "PPT模版", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 板↔版
		},
	},
	{
		"term-kehuduanshangxufei", "续费", "tool",
		[]VariantJSON{
			{Text: "需费", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 需↔续
			{Text: "叙费", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 叙↔续
		},
	},
	{
		"term-shipinzhanghao", "视频账号", "tool",
		[]VariantJSON{
			{Text: "视屏账号", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 屏↔频 近音
			{Text: "视频帐号", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 号↔号
		},
	},
	{
		"term-biaoshu", "标书", "tool",
		[]VariantJSON{
			{Text: "表书", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 表↔标 近音
			{Text: "镖书", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 镖↔标
		},
	},
	// 数据/物料
	{
		"term-mingxi", "明细", "data",
		[]VariantJSON{
			{Text: "名细", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 名↔明
			{Text: "命细", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 命↔明
		},
	},
	{
		"term-wuliao", "物料", "data",
		[]VariantJSON{
			{Text: "无料", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 无↔物
			{Text: "悟料", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 悟↔物
		},
	},
	// 阶段
	{
		"term-jieduan", "阶段", "phase",
		[]VariantJSON{
			{Text: "截断", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 截↔阶
			{Text: "节段", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 节↔阶
		},
	},
	{
		"term-zhixing", "执行", "phase",
		[]VariantJSON{
			{Text: "指行", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 指↔执
			{Text: "至行", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 至↔执
			{Text: "知行", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 知↔执
		},
	},
	{
		"term-zhuizong", "抓取", "phase",
		[]VariantJSON{
			{Text: "抓曲", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 曲↔取
			{Text: "抓趣", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 趣↔取
		},
	},
	// 组织
	{
		"term-zhongliuyuanyi", "肿瘤医院", "org",
		[]VariantJSON{
			{Text: "中流医院", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 流↔瘤
		},
	},
	// 关系
	{
		"term-bangzhu", "帮助", "action",
		[]VariantJSON{
			{Text: "邦助", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 邦↔帮
			{Text: "棒助", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 棒↔帮
		},
	},
	{
		"term-peihe", "配合", "action",
		[]VariantJSON{
			{Text: "配和", Kind: "approximate", Confidence: 0.95, Source: "manual-asr"}, // 和↔合
			{Text: "佩合", Kind: "approximate", Confidence: 0.85, Source: "manual-asr"}, // 佩↔配
		},
	},
	// 补充用户特别关心的 ASR 错读：
	//   “个种子 -> 颗种籽”这种 ASR 错读（个↔颗不同音，但 ASR 中常见）。
	//   上面“金种子/黑种子”variants 已涵盖“种籽”等子↔籽变体。
}

func buildSystemLexicon(text1, text2, version string) LexiconJSON {
	entries := make([]EntryJSON, 0, len(businessTerms))
	for _, t := range businessTerms {
		py := pinyinlite.Convert(t.Text)
		freq := countOccurrences(text1+text2, t.Text)
		entries = append(entries, EntryJSON{
			ID:       t.ID,
			Text:     t.Text,
			Pinyin:   py,
			Variants: t.Variants,
			Meta: map[string]any{
				"category":      t.Category,
				"freq":          freq,
				"variant_count": len(t.Variants),
			},
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Text < entries[j].Text })
	return LexiconJSON{
		Version: version,
		Source:  "manual-curated-asr-typos",
		Entries: entries,
		Meta: map[string]any{
			"pipeline": map[string]any{
				"processors": []string{"normalize", "disfluency", "alias", "fuzzy"},
				"thresholds": map[string]any{
					"fuzzy_auto":    0.85,
					"fuzzy_suggest": 0.70,
					"alias_auto":    0.95,
				},
				"max_changes": 50,
			},
			"mined_from": []string{"text1", "text2"},
		},
	}
}

func countOccurrences(haystack, needle string) int {
	return strings.Count(haystack, needle)
}

// ----------------------------------------------------------------------------
// main
// ----------------------------------------------------------------------------

func main() {
	dataDir, err := resolveDataDir()
	if err != nil {
		log.Fatal(err)
	}

	// --- dept ---
	rawBytes, err := os.ReadFile(filepath.Join(dataDir, "dept.raw.json"))
	if err != nil {
		log.Fatalf("read dept.raw.json: %v", err)
	}
	var deptResp struct {
		Code int       `json:"code"`
		Data []deptRaw `json:"data"`
	}
	if err := json.Unmarshal(rawBytes, &deptResp); err != nil {
		log.Fatalf("parse dept.raw.json: %v", err)
	}
	if deptResp.Code != 0 {
		log.Fatalf("dept resp code=%d", deptResp.Code)
	}
	deptEntries := transformDept(deptResp.Data, "dept-2025-09-08")
	writeJSON(filepath.Join(dataDir, "dept.json"), LexiconJSON{
		Version: "dept-2025-09-08",
		Source:  "admin-api/system/dept/list",
		Entries: deptEntries,
	})
	fmt.Printf("dept: %d entries → data/dept.json\n", len(deptEntries))

	// --- user (member) ---
	rawBytes, err = os.ReadFile(filepath.Join(dataDir, "member.raw.json"))
	if err != nil {
		log.Fatalf("read member.raw.json: %v", err)
	}
	var memberResp struct {
		Code int `json:"code"`
		Data struct {
			List []memberRaw `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawBytes, &memberResp); err != nil {
		log.Fatalf("parse member.raw.json: %v", err)
	}
	if memberResp.Code != 0 {
		log.Fatalf("member resp code=%d", memberResp.Code)
	}
	userEntries := transformMember(memberResp.Data.List, "user-2025-09-08")
	writeJSON(filepath.Join(dataDir, "user.json"), LexiconJSON{
		Version: "user-2025-09-08",
		Source:  "admin-api/qua/member-extended/page",
		Entries: userEntries,
	})
	fmt.Printf("user: %d entries → data/user.json\n", len(userEntries))

	// --- system (business terms) ---
	sysLex := buildSystemLexicon(testText1, testText2, "system-2025-09-08")
	writeJSON(filepath.Join(dataDir, "system.json"), sysLex)
	fmt.Printf("system: %d terms → data/system.json\n", len(sysLex.Entries))

	fmt.Println("\n[OK] transform done. Next: go run ./cmd/03-engine-test")
}

// 测试文本（与 PLAN.md §3 一致）
const testText1 = "好呃，叶海燕夏奇君、袁梦莲参加播种线下课程的确认沟通，加二十五个金种子袁梦莲参加播种线下课程沟通中提出好建议。加十个金种子杨行宇细化阶段性工作执行，并提供执行依据。加十五个金种子杨行宇招投标页面多条件查询，加二十个金种子，填清线下实践卡思路的思考。加十五个金种子朱凤，加三十个金种子沟通供应商明细，仔细朱凤，加二十个金种子沟通交流，各项事务高效田华。加二十个金种子，主动帮助上传资料田华，加三十个金种子，开通客户软件续费，陈新静做二外，标书加四十个金种子，吴旭辉做二p p t模板。加二十个金种子设立群。中午给同事打饭，加十个金种子。陈科航，早上热情向大家问好。驾驶科金种子。五西辉快速完成安排的工作，加二十个菌种子、芦川、阳城、吴旭辉未按要求完成表格录入，加五个黑种子。好。"

const testText2 = "熊龙军给袁梦莲加二十克金种子，提供儿童金种子历史物料，协助播种未来软件，给田华加十五克金种子。根据投标智能体的计划完成数据抓取给田青加十五克金种子。根据儿童金种子的入料分析，产品结合设计给杨须宇加十克金种子持续推进前端技术框架的升级。落第田华给田青加十五克金种子十件卡，相关页面设计给杨须宇加十克金种子解决开发遇到的问题。袁梦莲给邓子加三十克金种子。八月视频账号数据统计给林宇豪加三十克金种子，整理精修游学。照片下，其君给陈新静加三百八十克金，总对肿瘤医院中标菌种子分配给龚建军加二百七十颗金种子。肿瘤医院中标给夏季菌加一百克金种子。肿瘤医院中标给胜利群，卢川五西辉羊城冰月珠缝各家五十颗金种子。肿瘤医院中标给胜利群加二十克金种子处理。三、可业务叶海燕给朱凤加三十克金种子沟通、对接、处理、社保等相关事务给田华。杨徐宇、林宇豪、伍锡辉、扬城冰月独穿，各加五十克金种子，配合会议准备及事务。"

func writeJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}

// resolveDataDir 与 cmd/01-fetch 保持一致
func resolveDataDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(cwd, "data"),
		filepath.Join(cwd, "example", "09-compare-engine-vs-llm", "data"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return candidates[0], nil
}
