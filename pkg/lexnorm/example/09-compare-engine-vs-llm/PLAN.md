# 实施计划：`example/09-compare-engine-vs-llm`

> **目的**：用真实业务数据驱动 ark-lexnorm 引擎 vs DeepSeek 大模型，
> 在两条 ASR 风格的真实文本上做规范化对比，得出 ark-lexnorm 工具包的
> **优化建议**（不修改工具包，只给清单）。

---

## 1. 边界 / 非目标

| 类别 | 决定 |
|---|---|
| 是否修改 ark-lexnorm 工具包 | ❌ **不修改**，只读 API |
| 是否引入第三方 SDK | ❌ 不引入；LLM client 用 stdlib `net/http` + `encoding/json` |
| user.json 敏感字段 | ❌ **不写入**手机号 / 头像 / userId / createTime |
| 脏数据处理 | ✅ **保留真实人名 + 少量边界脏数据** |
| LLM 选用 | ✅ **DeepSeek**（`deepseek-chat`） |
| 性能 / 压测 | ❌ 不做 |
| 生产化 LLM 客户端 | ❌ 仅 demo 级别 |

---

## 2. 数据 Schema 约定

### `data/user.json`
```json
{
  "version": "user-2025-09-08",
  "source": "admin-api/qua/member-extended/page",
  "entries": [
    {
      "id": "user-2096146491009257474",
      "text": "龚千友",
      "pinyin": "gong qian you",
      "variants": [
        { "text": "工千友", "kind": "homophone", "confidence": 0.85, "source": "auto-pinyin" }
      ],
      "meta": { "dept_id": "1904450235179954177", "dept_name": "万康盛鼎集团" }
    }
  ]
}
```

### `data/dept.json`
```json
{
  "version": "dept-2025-09-08",
  "source": "admin-api/system/dept/list",
  "entries": [
    {
      "id": "dept-1904450235179954177",
      "text": "万康盛鼎集团",
      "pinyin": "wan kang sheng ding ji tuan",
      "variants": [
        { "text": "万康盛鼎", "kind": "alias", "confidence": 0.9 }
      ],
      "meta": { "parent_id": "0", "path": "万康盛鼎集团" }
    }
  ]
}
```

### `data/system.json`（业务术语 + pipeline 元数据）
```json
{
  "version": "system-2025-09-08",
  "source": "auto-mined-from-texts",
  "entries": [
    { "id": "term-jinzhongzi", "text": "金种子", "pinyin": "jin zhong zi",
      "variants": [{"text":"金种子","kind":"alias","confidence":1.0}],
      "meta": {"category":"reward","freq_in_text1":11} }
  ],
  "meta": {
    "pipeline": {
      "processors": ["normalize", "disfluency", "pinyin", "alias", "fuzzy"],
      "thresholds": { "fuzzy_auto": 0.85, "fuzzy_suggest": 0.70 },
      "max_changes": 50
    },
    "mined_from": ["text1", "text2"]
  }
}
```

> system.json 的 `meta.pipeline` 是给最终 example 展示配置可追溯用的元数据，引擎不直接读。

---

## 3. 工程结构

```
example/09-compare-engine-vs-llm/
├── README.md
├── PLAN.md                    # 本文件
├── data/
│   ├── dept.raw.json
│   ├── member.raw.json
│   ├── user.json
│   ├── dept.json
│   └── system.json
├── logs/
│   ├── 01-engine.log
│   ├── 02-llm.log
│   ├── 03-pk.md
│   └── 04-optimize.md
└── cmd/
    ├── 01-fetch/main.go
    ├── 02-transform/main.go
    ├── 03-engine-test/main.go
    ├── 04-llm-test/main.go
    └── 05-pk-report/main.go
```

---

## 4. 步骤分解

### Step 1 · 抓 raw 数据
- 产物：`data/dept.raw.json`, `data/member.raw.json`
- 验证：JSON 完整、code=0、字段齐全

### Step 2 · 数据清洗 + 术语挖掘
- 产物：`data/user.json`, `data/dept.json`, `data/system.json`
- 清洗规则：仅保留 `id/name/deptName`；脏数据启发式保留 ~20%
- 拼音生成：`internal/pinyinlite`（3500 常用字手写表，多音字按高频音）
- 术语挖掘：基于两条文本 + 高频词表

### Step 3 · 引擎规范化
- Pipeline：`normalize → disfluency → alias → fuzzy`
- Lexicon：`Compose(SliceSource(user), SliceSource(dept), SliceSource(system))`
- 日志：每条 Change 一行 JSON（text_id, input, output, match_type, source, confidence, step, latency_ms）

### Step 4 · LLM 规范化
- Client：`internal/llmclient/deepseek.go`（OpenAI 兼容协议，stdlib）
- Prompt：词库 JSON + 文本 + JSON 输出 schema

### Step 5 · PK 矩阵
- 逐 token 对比：✅一致 / ⚠️冲突 / 🤖Engine-Only / 🧠LLM-Only / ⏭ Skip
- 统计：按类型、按耗时
- 产物：`logs/03-pk.md`

### Step 6 · 优化建议
- 4 类：A 引擎短板 / B LLM 短板 / C 重叠区 / D 互补区
- 对 ark-lexnorm 工具包的具体建议
- 产物：`logs/04-optimize.md`

### Step 7 · 最终 example 落盘
- 整合 cmd 下 5 个程序为可独立运行的整体
- 写 README：说明这是 M11+ 的对比基准，不是推荐生产用法

---

## 5. 验证清单

| # | 验证项 | 命令 |
|---|---|---|
| V1 | 所有 cmd 编译通过 | `go build ./...` |
| V2 | 抓数据成功 | `ls data/*.raw.json` |
| V3 | 词库转换成功 | `ls data/{user,dept,system}.json` |
| V4 | 引擎跑通 | `cat logs/01-engine.log | wc -l` ≥ 1 |
| V5 | LLM 跑通 | `cat logs/02-llm.log | wc -l` ≥ 1 |
| V6 | PK 报告生成 | `cat logs/03-pk.md` 存在且非空 |
| V7 | 优化建议生成 | `cat logs/04-optimize.md` 存在且非空 |
| V8 | example 整体可跑 | `go run ./example/09-compare-engine-vs-llm/cmd/...` 串联通 |

---

## 6. 风险 & 兜底

| 风险 | 兜底 |
|---|---|
| LLM 返回非 JSON | 重试 1 次；失败则把原始响应落到 log |
| LLM 超时 | 30s 超时；失败标 "timeout" |
| Token 超限 | 文本 + 词库预估< 4K token；超就只发 top-50 entries |
| 同名歧义 | user.json 用 `(dept_id, name)` 联合做 ID |
| 引擎把脏数据当人名 | 启发式：低 confidence 不改 |