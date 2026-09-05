# 08 · 横切能力：Middleware 与 Hook

> 源节：§25 Middleware · §26 Recover Middleware · §27 Hook
> 适用阶段：Phase 7
> 受众：核心开发者
> 关键性：**统一横切扩展机制**，避免出现两套并存导致用户困惑
> **1.2 更新**：对齐 1.2 §25-27；Hook Event 字段（Changes/Result）已与新 Result 字段一致

---

## 1. 设计动机

横切关注点：panic recover、timing、tracing、metrics、logging、timeout……

ark-lexnorm 提供**单一**横切扩展机制：

- **Middleware**：包裹 Pipeline/Processor 执行链，可读可写
- **Hook**：旁路观察，**只读**，不得影响规范结果

> **一个库只该有一种横切扩展机制。**

---

## 2. Middleware（§29）

### 类型

```go
type Handler func(
    context.Context,
    *State,
) error

type Middleware func(Handler) Handler
```

### 内置 Middleware

| Middleware | 作用 |
|---|---|
| `Recover()` | panic → `*PanicError`（含原始 value + stack） |
| `Timing()` | 单点计时（消除 Pipeline/Decorator 双重计时） |
| `Timeout(d)` | 单次执行超时控制 |
| `WithHooks(h ...Hook)` | 把 Hook 接入 Pipeline 执行链 |

### Processor 不负责

- panic recovery
- timing
- tracing
- metrics
- logging

> 这些全部由 Middleware / Hook 承担，Processor 保持纯粹。

---

## 3. Middleware 链组合

```go
eng, _ := lexnorm.New(
    lexnorm.WithMiddleware(
        lexnorm.Recover(),
        lexnorm.Timing(),
        lexnorm.WithHooks(hookA, hookB),
        myTracingMiddleware,
    ),
)
```

### 执行顺序

```text
Recover           ← 最外层
  ↓
Timing
  ↓
WithHooks
  ↓
myTracing
  ↓
Pipeline.Process  ← 最内层
  ↓
返回
```

> **声明顺序 = 执行顺序（外→内）**。

---

## 4. Middleware 顺序约束（§66）

Middleware 采用**显式顺序**，执行顺序必须稳定。

```go
WithMiddleware(
    Recover(),
    Timing(),
    WithHooks(...),
)
```

### 禁止

```text
Middleware 隐式改变 Processor Order
```

Middleware 可以**包裹**执行，但**不得重排** Processor。

---

## 5. Middleware 实现模式

```go
func MyMiddleware() lexnorm.Middleware {
    return func(next lexnorm.Handler) lexnorm.Handler {
        return func(ctx context.Context, s *lexnorm.State) error {
            // 前置逻辑
            start := time.Now()

            err := next(ctx, s)

            // 后置逻辑
            log.Printf("duration=%v err=%v", time.Since(start), err)
            return err
        }
    }
}
```

### 注意事项

- **不要吞掉 error**：`return nil` 会让上层 Middleware / Pipeline 误判成功
- **不要阻塞**：Middleware 内的 I/O 必须接受 ctx 取消
- **不要修改 *State 文本**：Middleware 不得调用 `state.Replace` / `state.Suggest`，只读 + 旁路

---

## 6. Hook（§30）

### 接口

```go
type Hook interface {
    OnEvent(context.Context, Event)
}
```

### Event

```go
type Event struct {
    Type      EventType
    Processor string         // 当前 Processor 名（PipelineStart/End 为空）
    Duration  time.Duration
    Changes   int            // 本步产生的 Change 数
    Err       error          // 本步错误（若有）
    Result    *Result        // 仅 PipelineEnd 非 nil
}
```

### EventType

```go
type EventType uint8

const (
    EventPipelineStart EventType = iota
    EventProcessorStart
    EventProcessorEnd
    EventPipelineEnd
)
```

> **1.2 决议**：Event.Type 在 1.1 中改为 `string`（放松类型安全）。1.2 保留为 `EventType uint8` 枚举（与 Result / Action / Certainty 一致）。`Changes` 字段保留为 `[]Change` 类型以提供完整切片。

---

## 7. Hook 安全约束（§65）

| 维度 | 约束 |
|---|---|
| Hook 失败影响 | 默认**不影响** Processor / Pipeline / Result |
| Hook 与 State | Hook **不得修改 State** |
| Hook 与 Result | Hook **不得修改 Result** |
| Hook 同步性 | Hook 调用应是**同步**且**短小** |

> Hook 属于旁路能力（side channel）。

### Hook 内部错误处理

```go
func (h *MyHook) OnEvent(ctx context.Context, e lexnorm.Event) {
    defer func() {
        // Hook 自身的 panic 也不应影响规范结果
        _ = recover()
    }()
    // ...
}
```

---

## 8. 内置 Hook 实现

### `hooks/slog.go`

基于 `log/slog`：

```go
hooks.NewSlog(logger, hooks.WithText())  // 默认不打文本内容
```

| 选项 | 默认 | 说明 |
|---|---|---|
| `WithText()` | 关 | 是否输出文本内容（默认关，避免敏感信息） |
| `WithLevel(level)` | INFO | 日志级别 |

### `hooks/metrics.go`

基于 `expvar` / Prometheus-friendly 接口：

```go
hooks.NewMetrics(hooks.WithNamespace("lexnorm"))
```

暴露：

- `lexnorm_pipeline_duration_seconds`（Histogram）
- `lexnorm_processor_duration_seconds{processor="..."}`（Histogram）
- `lexnorm_changes_total{processor="...",applied="true|false"}`（Counter）
- `lexnorm_errors_total{processor="..."}`（Counter）

---

## 9. Middleware vs Hook 选型

| 场景 | 用 Middleware | 用 Hook |
|---|:--:|:--:|
| panic recover | ✅ | ❌ |
| 单次执行计时 | ✅ | ❌ |
| tracing 注入 | ✅ | ❌ |
| 超时控制 | ✅ | ❌ |
| 日志记录 | ❌ | ✅ |
| metrics 计数 | ❌ | ✅ |
| 事件订阅 | ❌ | ✅ |
| 修改 ctx 值 | ✅ | ❌ |

> **口诀**：Middleware = 改 ctx / 拦截 error；Hook = 旁路观察。

---

## 10. 与 v1 之前的设计取舍

| 历史方案 | 现状 | 原因 |
|---|---|---|
| Decorator 包裹单个 Processor | ❌ 取消 | 与 Middleware 重复 |
| `Notify*` 自由函数 | ❌ 取消 | 与 Hook 重复 |
| 内联 `safeNotify` + `recover` | ❌ 取消 | 收敛到 Recover() Middleware |

> 一个库只该有**一种**横切扩展机制，两种会让用户困惑「我该用哪个」。

---

## 11. 自检清单

- [ ] 是否引入了第二套横切机制（Decorator / Notify）？
- [ ] Middleware 是否在吞掉 error？
- [ ] Middleware 是否修改了 State 文本？
- [ ] Hook 是否在修改 State / Result？
- [ ] Hook 内部是否有 panic 防护？
- [ ] Middleware 链顺序是否有歧义？
- [ ] Middleware 是否隐式重排了 Processor？

---

## 12. 相关文档

- 上游：[07-Engine与Profile](07-Engine与Profile.md) §6
- 错误：[10-配置校验与错误体系](10-配置校验与错误体系.md) §4
- 性能：[14-性能设计与算法优化](14-性能设计与算法优化.md) §7 Timing Middleware
- 测试：[15-测试策略与质量工程](15-测试策略与质量工程.md) §6
