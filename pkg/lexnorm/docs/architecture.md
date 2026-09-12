# 架构总览（索引）

> 本文是 ark-lexnorm 架构文档的导航层。逐主题的权威细节在 `docs/00~17`
> 拆解文档与《ark-lexnorm-架构设计与开发规范1.2.md》中，本文不复制其内容。

## 核心抽象

```text
Engine ── Normalize(ctx, text) ──> Result
  │
  ├─ Runtime        # 每次调用的不可变快照（Profile/Lexicon/Pipeline/Config）
  ├─ Pipeline       # Processor 的有序组合（Pipeline 本身也是 Processor）
  ├─ State          # 单次请求工作区（Replace/Suggest/Lock/Rewrite）
  └─ Lexicon        # 词法知识（Entry/Variant/Relation + 各类索引）
```

不变量（I1~I11，权威定义见 `.agents/RULES.md` §2）：Processor 独立运行、
修改必须经 State、Runtime Snapshot 一致、核心零三方依赖、无业务概念等。

## Processor 体系（v1.1）

八大分类、能力 Descriptor、独立运行与组合规则：
见 **[processor.md](processor.md)**（正式规范）。

## 逐主题文档索引

| 主题 | 文档 |
|---|---|
| 项目定位与设计原则 | [00-项目定位与设计原则.md](00-项目定位与设计原则.md) |
| 架构总览与包结构 | [01-架构总览与包结构.md](01-架构总览与包结构.md) |
| 核心领域模型 | [02-核心领域模型.md](02-核心领域模型.md) |
| Processor 接口与生命周期 | [03-Processor接口与生命周期.md](03-Processor接口与生命周期.md) |
| Pipeline 与执行顺序 | [04-Pipeline与执行顺序.md](04-Pipeline与执行顺序.md) |
| State 与保护区机制 | [05-State与保护区机制.md](05-State与保护区机制.md) |
| Lexicon 与热更新 | [06-Lexicon与热更新.md](06-Lexicon与热更新.md) |
| Engine 与 Profile | [07-Engine与Profile.md](07-Engine与Profile.md) |
| Middleware 与 Hook | [08-横切能力Middleware与Hook.md](08-横切能力Middleware与Hook.md) |
| Registry 与动态装配 | [09-Registry与动态装配.md](09-Registry与动态装配.md) |
| 配置校验与错误体系 | [10-配置校验与错误体系.md](10-配置校验与错误体系.md) |
| 确定性与匹配冲突消解 | [11-确定性与匹配冲突消解.md](11-确定性与匹配冲突消解.md) |
| 内置 Processor 规范 | [12-内置Processor规范.md](12-内置Processor规范.md) |
| 应用场景与 Pipeline 模板 | [13-应用场景与Pipeline模板.md](13-应用场景与Pipeline模板.md) |
| 性能设计与算法优化 | [14-性能设计与算法优化.md](14-性能设计与算法优化.md) |
| 测试策略与质量工程 | [15-测试策略与质量工程.md](15-测试策略与质量工程.md) |
| 开源工程治理 | [16-开源工程治理.md](16-开源工程治理.md) |
| 开发实施路线 | [17-开发实施路线.md](17-开发实施路线.md) |
| 工具包缺陷清单 | [19-工具包缺陷清单.md](19-工具包缺陷清单.md) |
| 词条字段说明 | [20-词条字段说明.md](20-词条字段说明.md) |

## 行为基线与回归

`golden_test.go` + `testdata/golden/` 冻结内置预设的可观察行为；
任何行为变更必须以 `-update` 再生成并在 CHANGELOG 中逐条说明。

## 已知边界（当前版本）

- Contextual v1：只消费 `Variant{Contextual}` 并 Suggest，跨词 ASR 错读（T17）
  与同姓歧义评分（T33）为后续增强。见 [19-工具包缺陷清单.md](19-工具包缺陷清单.md)。
- module path 为 `github.com/stack-haven/lexnorm`（与早期规范的 `github.com/ark/lexnorm`
  不同，改名属破坏性变更，暂缓）。
