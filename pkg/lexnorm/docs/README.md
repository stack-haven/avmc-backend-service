# ark-lexnorm 正式开发文档

> **权威规范**：`ark-lexnorm-架构设计与开发规范1.2.md`（1.1 主体 + 1.0 缺失章节 + 3 项冲突解决）
>
> **历史归档**：
> - `ark-lexnorm-架构设计与开发规范1.0.md`（81 节原始论述）
> - `ark-lexnorm-架构设计与开发规范1.1.md`（63 节原始论述）
>
> **拆分原则**：按"对象职责"聚合，可独立引用，分阶段交付友好
> **阅读顺序**：00 → 01 → 02 → 03 → 04 → 05 → 06 → 07 → 12 → 17
> **交叉引用**：每节标注「源 §X.X」，可与 1.2 规范双向定位
>
> **🤖 Coding Agent 必读入口**：
> 1. `../CLAUDE.md`（仓库根 Claude Code 入口）
> 2. `../.agents/AGENTS.md`（项目指南）
> 3. `../.agents/RULES.md`（规则 + 不变量 + 决策）
> 4. `../.agents/REVIEW.md`（Code Review 清单，36 项阻断项）
> 5. `../.agents/DESIGN.md`（设计原则）
> 6. 本文件（文档索引）

---

## 文档索引

### 规范文档（**权威 + 历史归档**）

| 版本 | 文档 | 状态 |
|:--:|---|:--:|
| **1.2** | [**ark-lexnorm-架构设计与开发规范1.2.md**](ark-lexnorm-架构设计与开发规范1.2.md) | ✅ **正式工作基线**（48 节，含决策日志 D1-D7） |
| 1.1 | [ark-lexnorm-架构设计与开发规范1.1.md](ark-lexnorm-架构设计与开发规范1.1.md) | 📚 历史归档（63 节原始论述） |
| 1.0 | [ark-lexnorm-架构设计与开发规范1.0.md](ark-lexnorm-架构设计与开发规范1.0.md) | 📚 历史归档（81 节原始论述） |

### 拆解文档（**18 份，按对象职责聚合**）

| # | 文档 | 对应 1.2 章节 | 适用阶段 | 受众 |
|:--:|---|:---:|:--:|---|
| 00 | [项目定位与设计原则](00-项目定位与设计原则.md) | §1, §2, §46 | 全阶段 | 所有人 |
| 01 | [架构总览与包结构](01-架构总览与包结构.md) | §3, §39, §45 | 全阶段 | 架构师 / Reviewer |
| 02 | [核心领域模型](02-核心领域模型.md) | §4, §10, §20 | Phase 1 | 核心开发者 |
| 03 | [Processor接口与生命周期](03-Processor接口与生命周期.md) | §4.1 | Phase 1, 5 | 核心 + 扩展开发者 |
| 04 | [Pipeline与执行顺序](04-Pipeline与执行顺序.md) | §4.2, §21 | Phase 2, 6 | 核心 + 业务开发者 |
| 05 | [State与保护区机制](05-State与保护区机制.md) | §5, §6, §7 | Phase 3 | 核心开发者 |
| 06 | [Lexicon与热更新](06-Lexicon与热更新.md) | §11~§15, §34 | Phase 4 | 核心开发者 |
| 07 | [Engine与Profile](07-Engine与Profile.md) | §4.3~§4.6 | Phase 7 | 核心开发者 |
| 08 | [横切能力Middleware与Hook](08-横切能力Middleware与Hook.md) | §25, §26, §27 | Phase 7 | 核心开发者 |
| 09 | [Registry与动态装配](09-Registry与动态装配.md) | §24 | Phase 8 | 扩展开发者 |
| 10 | [配置校验与错误体系](10-配置校验与错误体系.md) | §28, §29 | Phase 1, 8 | 所有人 |
| 11 | [确定性与匹配冲突消解](11-确定性与匹配冲突消解.md) | §22, §23, §36 | 全阶段 | Reviewer |
| 12 | [内置Processor规范](12-内置Processor规范.md) | §19, §34.4 | Phase 5 | 核心 + 学习者 |
| 13 | [应用场景与Pipeline模板](13-应用场景与Pipeline模板.md) | §41 | Phase 6 | 业务开发者 |
| 14 | [性能设计与算法优化](14-性能设计与算法优化.md) | §36, §37, §38 | Phase 9 | 性能优化者 |
| 15 | [测试策略与质量工程](15-测试策略与质量工程.md) | §37, §40 | 全阶段 | QA / 贡献者 |
| 16 | [开源工程治理](16-开源工程治理.md) | §39, §45 | Phase 10 | Release Manager |
| 17 | [开发实施路线](17-开发实施路线.md) | §42, §43, §44 | 全阶段 | Tech Lead |

---

## 1.2 决策摘要（与历史版本差异）

### 决策记录

| ID | 主题 | 1.2 决议 |
|:--:|---|---|
| D1 | LLM 在默认顺序 | **保留为可选扩展，不在 Standard Preset 内** |
| D2 | `New(...)` 返回值 | **保留 error 返回**（`(*Engine, error)`） |
| D3 | Result 字段 | **保留全部字段**：Original / Duration / Steps / Err 与 Suggestions / Errors / Runtime 并存 |
| D4 | Match 冲突规则 | **从 1.0 §63 补回**：Longest Match First → Higher Priority → Stable Lexicographical |

### 重大新增概念（来自 1.1）

- **Runtime Snapshot**：一次 Normalize 调用的完整不可变快照（§4.4）
- **ProfileResolver**：多 Profile 共用一个 Engine 的路由抽象（§4.6）
- **LexiconSource + Compose**：多源合并 Lexicon（§12, §13）
- **完整 HA 体系**：Engine / Lexicon / Request / Processor 四级（§34）
- **故障矩阵**：11 种故障 × 行为对照（§35）
- **业务场景验收**：场景 A-F（§41）

---

## 三条主线阅读路径

### 路径 A：核心开发者（要写 Phase 1~4 的代码）

```
00 → 01 → 02 → 03 → 04 → 05 → 06 → 10 → 11 → 17
```

### 路径 B：业务开发者（要用 ark-lexnorm 解决业务问题）

```
00 → 02 → 04 → 07 → 12 → 13 → 17
```

### 路径 C：扩展开发者（要写自定义 Processor / 接入 Registry）

```
00 → 02 → 03 → 04 → 06 → 09 → 11 → 17
```

---

## 文档维护约定

1. **任何对规范的修改**，先在 1.2 权威版中更新，再同步到对应拆解文档，并在该文档头部标注「修订记录」。
2. **新增示例**统一放 `examples/`，不在拆解文档中堆代码。
3. **架构不变量**（§42 的 15 条）在 Code Review 时按 [17-开发实施路线](17-开发实施路线.md) §3 清单逐项检查。
4. **跨文档引用**统一用相对路径：`[State API](05-State与保护区机制.md#state-公开能力)`。

---

## 与项目其他文档的关系

- 上游业务规划：`docs/services/evie-platform/development/8-词库中心与文本增强引擎开发计划.md`
- 上游评分报告：`docs/services/evie-platform/architecture/1-代码评分报告.md`
- 本文档不重复业务侧内容，仅定义开源库的契约

---

## 🤖 Coding Agent 快速参考

### 中断后恢复

```bash
# 1. 看当前 M
cat ../docs/17-开发实施路线.md | grep -A 30 "关键里程碑"

# 2. 看决策是否被违反
cat ../.agents/REVIEW.md | grep -A 10 "决策日志"

# 3. 跑测试看当前状态
cd .. && go test ./... 2>&1 | tail -20
```

### 常见违反（参考）

| 错误代码 / 现象 | 可能原因 | 查阅 |
|---|---|---|
| `processor "asr-clean"` | S01 业务词 | `RULES.md` §1 |
| `tenantID` 出现在 PR | A11 业务耦合 | `RULES.md` §1 |
| `New() *Engine`（无 error） | D2 被违反 | `RULES.md` §3 |
| `preset.Standard()` 含 LLM | D1 被违反 | `RULES.md` §3 |
| Result 缺少 `Original` 字段 | D3 被违反 | `RULES.md` §3 |
| 测试不同运行结果不一致 | DET 系列违反 | `RULES.md` §6 |
| `go list -deps` 含第三方 | B01 被违反 | `RULES.md` §4 |

### 修改决策的正确流程

```
提议 D8 → 更新 1.2 决策日志 → 更新 RULES.md §3 → PR 描述明确说明
```

> **禁止** 不经过 D8+ 直接修改 D1-D7 默认值。
