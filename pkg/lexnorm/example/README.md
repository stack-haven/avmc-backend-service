# ark-lexnorm 示例程序

> 8 个循序渐进的可运行示例，帮助新手从零开始使用 ark-lexnorm。

## 运行方式

每个示例都是独立的 `package main`，可以直接运行：

```bash
# 从 lexnorm 仓库根目录
go run ./example/01-basic
go run ./example/02-pipeline
go run ./example/03-presets
go run ./example/04-custom-processor
go run ./example/05-hooks
go run ./example/06-lexicon-store
go run ./example/07-profiles
go run ./example/08-error-handling
```

或进入子目录：

```bash
cd example/01-basic && go run .
```

## 示例索引

| # | 场景 | 你会学到 |
|--:|---|---|
| 01 | [基础替换](./01-basic) | `lexicon.Builder` 构建词典、`lexnorm.New` 构造引擎、`Engine.Normalize` 调用 |
| 02 | [自定义 Pipeline](./02-pipeline) | `lexnorm.NewPipeline` 自由组合 Processor |
| 03 | [内置 Preset](./03-presets) | `presets.Standard` / `presets.ASR` / `presets.Fast` 一键装配 |
| 04 | [自定义 Processor](./04-custom-processor) | 实现 `Processor` 接口（只需要 `Name()` + `Process()`） |
| 05 | [Hook 观测](./05-hooks) | 通过 `WithHooks` 监听每次 Change / Processor Start/End |
| 06 | [Lexicon 热更新](./06-lexicon-store) | `lexicon.Store` 实现零停机 HA 热更新 |
| 07 | [多 Profile 路由](./07-profiles) | `WithProfiles` + `WithDefaultProfile` 按 key 路由 |
| 08 | [错误处理](./08-error-handling) | 自定义 Processor panic 时引擎优雅降级 |

## 阅读顺序

```
01 → 02 → 03 → 04 ─┬─→ 05 → 06
                   └─→ 07
                                ↓
                              08
```

- **01–03**：三种构造引擎的方式（手动 Pipeline / 选 Processor / 用 Preset）
- **04**：扩展自己的处理逻辑
- **05–07**：高级场景（观测 / 热更新 / 多 Profile）
- **08**：生产环境的可靠性兜底

## 不在示例里的能力

下面这些已在测试覆盖，但为保持示例聚焦，未单独列出：

- `lexicon.Source` / `SliceSource`：从外部数据源批量加载
- `ProfileResolver`：运行时动态决定 Profile（比 `WithProfiles` 更灵活）
- `Middleware`：调用前后的横切逻辑（日志 / Trace / Metrics）
- `WithConfig`：自定义 `Config` 阈值（`AutoApplyThreshold` / `SuggestThreshold` / `MaxChanges`）
- `Engine.Normalize` 的 `CallOption`：`WithProfile`、`WithProtectedSpans`

需要时查阅根目录 `docs/`，或 `pkg.go.dev/github.com/stack-haven/lexnorm` 的 API 文档。

## 期望输出

每个示例的 README 都标注了**预期 stdout**（与代码 `Println` 完全对应）。
这样你可以直接对比运行结果是否正确，无需猜测。
