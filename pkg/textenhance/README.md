# pkg/textenhance — 文本增强引擎

> **状态**：✅ M6b 完成（+ Observer + Decorator + 默认 Logging/Counting Observer）
> **设计文档**：`docs/services/evie-platform/development/12-evie-tool文本增强M6设计模式方案.md`
> **共用**：`evie/service`（迁移中）+ `evie/tool`（M6d 阶段直接 import）

## 一、核心模式

| Pattern | 体现 |
|---|---|
| **策略接口** | `processors.TextProcessor`（Name + Process）|
| **Functional Options** | `NewXxxProcessor(opts ...Option)` 每个 Processor 子包独立 Option 类型 |
| **Pipeline** | `Pipeline.Run(ctx, ec)` 顺序执行 + Observer 事件触发 |
| **Registry** | `Registry.Register/Build` + `builtins.NewDefaultRegistry()` |
| **Decorator** ⭐ | `processors.ObservingProcessor` 包装 inner + 注入 observer |
| **Observer** ⭐ | `processors.Observer` 接口 + `observers.{Logging,Counting}Observer` 默认实现 |
| **Builder** | `BuildPipeline(reg, policy, opts...)` 一行构造 |
| **Atomic Swap** | （M6d 阶段：PipelineManager）|

## 二、目录结构（M6b 完成）

```
pkg/textenhance/
├── pipeline.go               # Pipeline（HA + Observer 事件触发）
├── registry.go               # Registry
├── builder.go                # BuildPipeline + WithObservers Option
├── policy.go                 # Policy + DefaultPolicy + Validate + clamp
├── errors.go                 # Status 枚举 + 错误工厂
├── context.go                # EnhancementContext / Change / VocabularySnapshot re-export
├── integration_test.go       # 端到端测试（Pipeline + Observer + 真实 Processor）
├── textenhance_test.go       # 集成测试（11 个 case）
├── README.md
│
├── processors/               # ⭐ 策略层（Strategy Layer）
│   ├── processor.go          # TextProcessor interface
│   ├── context.go            # EnhancementContext 真实定义
│   ├── change.go             # Change struct
│   ├── constants.go          # Action / Type / Source 常量
│   ├── snapshot.go           # VocabularySnapshot 真实定义
│   ├── status.go             # Status 枚举 + StatusName
│   ├── observer.go           # ⭐ M6b: Observer 接口 + ObservingProcessor 装饰器 + safeNotify
│   ├── observer_test.go      # ⭐ Observer + Decorator 测试
│   ├── common.go             # 公共工具 + 类型 + 依赖（IsPunctOrSpace / ReplaceAll / Stopword / PinyinService）
│   ├── common_test.go        # 公共测试
│   └── <9 子包>               # 9 个默认 Processor 实现
│       ├── cleaning/
│       ├── filler/
│       ├── vocab_matching/
│       ├── alias_resolution/
│       ├── deterministic_replacement/
│       ├── phrase_standardization/
│       ├── pinyin_correction/  # 用 PinyinService 注入
│       ├── fuzzy_matching/     # 用 ReplaceAll
│       ├── context_correction/
│       └── llm_reserved/
│
├── observers/                # ⭐ 默认 Observer 实现
│   ├── observers.go          # Logger 接口 + DiscardLogger + StdLogger
│   ├── logging.go            # LoggingObserver
│   ├── counting.go           # CountingObserver（线程安全统计）
│   └── observers_test.go     # 8 个 case
│
└── builtins/                 # NewDefaultRegistry（10 个 processor 工厂）
    └── builtins.go
```

## 三、Observer 接口（M6b 核心）

```go
// processors.Observer 监听 pipeline + processor 全生命周期事件。
type Observer interface {
    // Pipeline 级
    OnPipelineStart(ctx context.Context, processorNames []string)
    OnPipelineComplete(ctx context.Context, snap EnhancementSnapshot)
    OnPipelineError(ctx context.Context, err error)

    // Step 级
    OnProcessorStart(ctx context.Context, name string)
    OnProcessorComplete(ctx context.Context, name string, dur time.Duration, changes []Change)
    OnProcessorError(ctx context.Context, name string, err error)
}
```

**约束**：
- 线程安全（多 goroutine 可并发调）
- 不允许 panic（safeNotify 兜底）
- 不允许重 I/O（耗时操作放异步）
- 不修改 ec / snapshot（只读）

## 四、ObservingProcessor 装饰器

```go
// ObservingProcessor 是 Decorator：包装 inner processor + 注入 observer。
type ObservingProcessor struct {
    inner     TextProcessor
    observers []Observer
}

// 构造时由 BuildPipeline 自动包裹：
pipeline, _ := textenhance.BuildPipeline(reg, policy,
    textenhance.WithObservers(loggingObs, countingObs),
)
// 此时所有 processor 已被 Decorator 包裹；Pipeline.Run 会触发所有 observer 事件
```

**事件流**（每次 processor 执行）：

```
OnPipelineStart
  ├─ OnProcessorStart("cleaning")
  │   └─ [cleaning.Process 执行]
  │   └─ OnProcessorComplete("cleaning", dur, changes)
  ├─ OnProcessorStart("filler")
  │   └─ [filler.Process 执行]
  │   └─ OnProcessorComplete("filler", dur, changes)
  └─ ...
OnPipelineComplete(snap)
```

## 五、默认 Observer 实现

### 5.1 LoggingObserver

把事件写到 logger（Debug / Warn level）：

```go
logger := myLogger  // 实现 observers.Logger 接口
obs := observers.NewLoggingObserver(logger)
pipeline, _ := textenhance.BuildPipeline(reg, policy,
    textenhance.WithObservers(obs),
)
```

**Logger 接口**（与 kratos log.Helper 兼容）：
```go
type Logger interface {
    WithContext(ctx context.Context) Logger
    Debugf(format string, args ...any)
    Infof(format string, args ...any)
    Warnf(format string, args ...any)
    Errorf(format string, args ...any)
}
```

提供 `DiscardLogger`（测试用）和 `StdLogger`（func 适配器）作为内置实现。

### 5.2 CountingObserver

线程安全统计每步调用次数 / 错误数 / 总耗时 / 最长耗时：

```go
counter := observers.NewCountingObserver()
pipeline, _ := textenhance.BuildPipeline(reg, policy,
    textenhance.WithObservers(counter),
)
pipeline.Run(ctx, ec)
pipeline.Run(ctx, ec)  // 多次

// 读取统计
stats := counter.Stats()
// stats["cleaning"] = {
//     Invocations: 2,
//     Successes:   2,
//     Errors:      0,
//     TotalTime:   10ms,
//     MaxTime:     6ms,
//     AvgTime:     5ms,
// }
```

**HA 保障**：CountingObserver 内部用 mutex；并发 100 goroutine 测试通过。

## 六、构建 Pipeline 含 Observer

```go
import (
    "backend-service/pkg/textenhance"
    "backend-service/pkg/textenhance/builtins"
    "backend-service/pkg/textenhance/observers"
)

reg := builtins.NewDefaultRegistry()
policy := textenhance.DefaultPolicy()

// 单个 observer
pipeline, _ := textenhance.BuildPipeline(reg, policy,
    textenhance.WithObservers(myObserver),
)

// 多个 observer（按顺序触发）
pipeline, _ := textenhance.BuildPipeline(reg, policy,
    textenhance.WithObservers(
        loggingObserver,
        countingObserver,
        myCustomObserver,
    ),
)

// 无 observer（与 M6a 行为一致）
pipeline, _ := textenhance.BuildPipeline(reg, policy)
```

## 七、HA 三重保护

1. **panic recover** —— `Pipeline.Run` + `runOneStep` + `safeNotify` 三重兜底
2. **ctx 超时** —— Pipeline 入口自动加 5s timeout
3. **错误累积** —— errors 非空 → 降级为 PARTIAL

**Observer panic 不传播**：
- `ObservingProcessor.Process` 内：inner panic → 记录到 OnProcessorError + re-panic（让 pipeline 兜底）
- observer 自身 panic → safeNotify recover + 主流程继续

## 八、9 个 Processor 顺序

| 顺序 | 名称 | 类型 | 用到 common |
|:---:|---|:---:|:---:|
| 1 | cleaning | 确定性 | - |
| 2 | filler | 确定性 | `IsPunctOrSpace`, `Stopword` |
| 3 | vocab_matching | 索引 | - |
| 4 | alias_resolution | 确定性 | - |
| 5 | deterministic_replacement | 确定性 | - |
| 6 | phrase_standardization | 确定性 | - |
| 7 | pinyin_correction | 推断 | `PinyinService` |
| 8 | fuzzy_matching | 推断 | `ReplaceAll` |
| 9 | context_correction | 推断 | - |
| 10 | llm_reserved | 保留 | - |

## 九、进度

| M | 状态 | 内容 |
|---|:---:|------|
| M6a | ✅ | TextProcessor + 9 个 Processor + Pipeline + Registry + Policy + Context + Snapshot + Status + BuildPipeline + processors 重组 + common 抽取 |
| M6b | ✅ | **Observer 接口 + ObservingProcessor 装饰器 + LoggingObserver + CountingObserver** |
| M6c | 📋 | 业务化：Policy 阈值 / 集成 wire |
| M6d | 📋 | PipelineManager（atomic swap）+ Watcher（仅 stub）|

## 十、验证

```bash
cd backend-service
go test -race ./pkg/textenhance/...    # 3 个包全绿，52 个 case
  - pkg/textenhance              (15 cases)
  - pkg/textenhance/observers    (8 cases)
  - pkg/textenhance/processors   (29 cases)
```