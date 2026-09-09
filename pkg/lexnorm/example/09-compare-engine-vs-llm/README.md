# example/09-compare-engine-vs-llm

> **目标**：用真实业务数据驱动 ark-lexnorm 引擎 vs DeepSeek 大模型做规范化对比，
> 输出引擎优化计划。

## ⚠️ 项目术语约定（重要，请勿改回）

本项目的 system.json 里术语 canonical text **已用户手工改过**：

| 标准写法 | 旧写法 | 说明 |
|---|---|---|
| **金种籽** | 金种子 | canonical text |
| **黑种籽** | 黑种子 | canonical text |

> 这是项目偏好，不是因为"种籽"是 ASR 错读。所有变体（`金种仔`、`金钟子`、`金种资`、`紧种子` 等）依然指向 canonical `金种籽`。  
> **任何人修改 system.json 时，请保持 canonical text 为 `金种籽` / `黑种籽`，不要改回 `金种子` / `黑种子`。**

## 跑法

```bash
# 0. 准备：确保环境变量
export DEEPSEEK_API_KEY=sk-...   # Phase 4 需要

# 1. 抓 raw 数据 → data/*.raw.json
go run ./cmd/01-fetch

# 2. raw → 词库 JSON + 术语挖掘 → data/{user,dept,system}.json
go run ./cmd/02-transform

# 3. 引擎规范化 → logs/01-engine.log
go run ./cmd/03-engine-test

# 4. LLM 规范化 → logs/02-llm.log
go run ./cmd/04-llm-test

# 5. PK 报告 + 优化建议 → logs/03-pk.md + logs/04-optimize.md
go run ./cmd/05-pk-report
```

## 数据源

- **dept** — `admin-api/system/dept/list`（9 个部门）
- **member** — `admin-api/qua/member-extended/page?selectAll=true`（79 条成员 → 69 条词条）

## 测试文本

- **text1** — 叶海燕夏奇君等多人加金种子场景（约 340 字）
- **text2** — 熊龙军、田华、田青等投标+种子分配场景（约 395 字）

## 产物结构

```
data/
├── dept.raw.json          # curl ① 原始响应
├── member.raw.json        # curl ② 原始响应
├── user.json              # 转换后人名词库（65 entries：48 真实 + 17 边界脏数据）
├── dept.json              # 转换后部门词库（8 entries：合并同名 dept）
└── system.json            # 业务术语 + Pipeline 元数据（21 terms）

logs/
├── 01-engine.log          # 引擎逐条 Change 日志（JSON Lines）
├── 02-llm.log             # LLM prompt + 响应 + 解析日志（JSON Lines）
├── 03-pk.md               # PK 矩阵（按 from 对比两侧）
└── 04-optimize.md         # 对 ark-lexnorm 工具包的优化建议

cmd/
├── 01-fetch/              # 抓 raw 数据
├── 02-transform/          # raw → 词库 + 术语挖掘
├── 03-engine-test/        # 引擎规范化 + 打 log
├── 04-llm-test/           # DeepSeek 规范化 + 打 log
└── 05-pk-report/          # PK + 优化建议

internal/
├── pinyinlite/            # 363 汉字拼音表 + lexicon.PinyinConverter 实现
├── lexjson/               # JSON 词库 ↔ lexnorm Entry 互转
└── llmclient/             # DeepSeek OpenAI 兼容 client（stdlib）
```

## 实验结论（详见 `logs/04-optimize.md`）

| 维度 | 引擎 | LLM |
|---|---|---|
| **速度** | < 1ms | 2-5 秒 |
| **成本** | 0 token | ~2.5K token/次 |
| **确定性** | 100% | 浮动（同 prompt 不同次有差异） |
| **擅长的场景** | 标点/空格清理、词库内变体、显式同音 | 上下文推断、同音异字、ASR 错字 |
| **短板** | 缺 homophone processor、缺整词拼音匹配 | 偶尔误改、无意义修改、上下文漂移 |

**关键优化建议**：
1. 新增 `homophone` processor（当前 Variant{Homophone} 无人消费）
2. `pinyin` processor 增加整词模式
3. `fuzzy` processor 给出 Homophone 的默认 confidence
4. Pipeline 编排的"性价比排序"（高频高确定性放前面）
5. abbrev variant 加最短长度保护

详见 `PLAN.md`、`logs/03-pk.md`、`logs/04-optimize.md`。