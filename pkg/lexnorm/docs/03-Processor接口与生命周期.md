# 03 · Processor 接口与生命周期

> 源节：§4.1 Processor · §22 Processor Version（1.1 新增）
> 适用阶段：Phase 1, 5
> 受众：核心开发者 + 扩展开发者（写自定义 Processor 的人）
> 关键性：**v1 后 API 冻结**

---

## 1. 接口定义

```go
type Processor interface {
    Name() string
    Process(context.Context, *State) error
}
```

**该接口必须保持极简。**

> 这两个方法是 v1 后**永久冻结**的稳定 API。

### 1.2 推荐扩展接口（可选，非强制）

```go
// Versioner 是可选接口，由 Processor 自愿实现
type Versioner interface {
    Version() string
}
```

实现 `Versioner` 后，`Result.Change.ProcessorVersion` 与 `Result.Runtime.ProcessorVersions[name]` 会被自动填充，便于审计与重放。

**未实现 `Versioner` 的 Processor**：`ProcessorVersion` 字段填空字符串。**不报错**，不阻断。

---

## 2. Processor 生命周期（§5.2）

Processor 应尽量保持无状态。

### 推荐流程

```text
Processor
    ↓
Read State
    ↓
Calculate
    ↓
State.Replace / Suggest
    ↓
Return
```

### 禁止行为

```text
Processor
    ↓
修改 Engine
    ↓
修改 Pipeline
    ↓
修改共享 Lexicon
```

| 禁止操作 | 原因 |
|---|---|
| 持有 `*Engine` / `*Pipeline` 引用 | 破坏 Processor Independence（§2.1） |
| 修改 Lexicon | 违反 Lexicon Runtime Read Only |
| 通过全局变量传递状态 | 违反显式优于隐式（§2.3） |

---

## 3. Processor 不得直接修改文本（§5.3）

**禁止**：

```go
strings.ReplaceAll(...)
```

直接改变 State 外部文本。

**所有修改必须通过**：

```go
state.Replace(...)
```

**所有建议必须通过**：

```go
state.Suggest(...)
```

> Replace / Suggest 的实现细节、偏移维护、保护区检查由 State 统一负责。
> Processor 不需要、也不允许手动维护偏移。

---

## 4. Processor 独立运行（§6）

Processor **必须**支持脱离 Engine、Pipeline 单独使用。

> 这是正式 API 契约，**不是示例功能**。

### 标准调用形式

```go
p := alias.New(lexicon)

state := lexnorm.NewState(
    text,
    lexnorm.WithProfile("default"),
    lexnorm.WithLexicon(lexicon),
)

err := p.Process(ctx, state)

result := state.Result()
```

### 强制要求

- `processor/<name>` 子包**必须**提供 `New(config, ...) (*ProcessorType, error)` 构造函数
- 子包**不应**强制依赖 Engine
- 子包**不应**强制依赖 Pipeline
- 子包**不应**强制依赖 Registry

> 这条约束与 §2.1 Processor Independence 直接对应，违反任一即视为破坏架构不变量。

---

## 5. 自定义 Processor（§9）

开发者只需要实现 `Processor` 接口：

```go
type MyProcessor struct{}

func (p *MyProcessor) Name() string {
    return "my-processor"
}

func (p *MyProcessor) Process(
    ctx context.Context,
    state *lexnorm.State,
) error {
    // custom normalization logic
    // 仅通过 state.Replace / state.Suggest 修改文本
    return nil
}
```

### 直接加入 Pipeline

```go
pipeline := lexnorm.NewPipeline(
    clean.New(),
    &MyProcessor{},
    fuzzy.New(config),
)
```

### 约束

- **不得要求用户修改核心 Engine**
- **不得要求用户修改核心 Pipeline**
- **不得要求用户修改核心 Lexicon**
- 自定义 Processor 与内置 Processor **使用完全相同的接口契约**

---

## 6. 内置 Processor 必须可被独立调用（§5 + §6 联合约束）

每个内置 Processor 子包除了 `Process(ctx, *State) error` 之外，还应满足：

| 要求 | 说明 |
|---|---|
| 子包提供 `New(...)` 构造函数 | 不依赖 Engine / Pipeline / Registry |
| 子包可单独 import | `import "github.com/stack-haven/lexnorm/processor/clean"` 即可使用 |
| 子包自带测试 | 不强制启动完整 Engine |

### 推荐子包结构

```text
processor/clean/
├── clean.go         # Processor 实现 + New()
├── clean_test.go    # 单 Processor 测试
└── doc.go           # 包级文档
```

---

## 7. 单 Processor 测试规范（§67）

每个 Processor 必须可以单独测试：

```go
state := NewState(...)
processor.Process(ctx, state)
```

### 测试不得强制启动完整 Engine

### 必须覆盖的场景

- 命中典型输入
- 未命中
- 保护区场景（被 Lock 的区间不应被修改）
- 边界：空文本、空 Lexicon、单字符
- 错误：上下文取消、配置非法

### 示例测试结构

```go
func TestCleanProcessor(t *testing.T) {
    tests := []struct{
        name    string
        input   string
        want    string
        locked  []Span  // 预 Lock 的区间
    }{
        {"去除控制字符", "hello\x00world", "helloworld", nil},
        {"合并重复标点", "hello!!!world", "hello!world", nil},
        {"保护区间不被改", "hello!!!world", "hello!!!world", []Span{{6,9}}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            state := NewState(tt.input)
            for _, sp := range tt.locked {
                state.Lock(sp)
            }
            p := clean.New()
            if err := p.Process(context.Background(), state); err != nil {
                t.Fatal(err)
            }
            if got := state.Text(); got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

---

## 8. 错误处理约定

- `Process` 返回 error 时，Pipeline 按 ErrorPolicy 处理（详见 [10-配置校验与错误体系](10-配置校验与错误体系.md)）
- 推荐包装为 `*ProcessorError`：

```go
return &lexnorm.ProcessorError{
    Name: p.Name(),
    Op:   "process",
    Err:  errors.New("lexicon not initialized"),
}
```

- 调用方可用 `errors.As(err, &target)` 判定

---

## 9. 资源管理

- Processor 在 `New` 阶段完成所有资源加载（正则编译、词典加载）
- 运行期不进行 I/O
- 不持有 ctx 之外的 Goroutine
- 关闭资源（如需要）由 `Processor` 实现额外可选接口 `io.Closer`（**非核心接口，可选**）

---

## 10. 自检清单

- [ ] 是否持有 `*Engine` / `*Pipeline` 引用？（应否）
- [ ] 是否使用 `strings.ReplaceAll` 等直接修改文本？
- [ ] 是否能从子包单独导入测试？
- [ ] 是否在 New 阶段完成所有资源加载？
- [ ] 错误是否包装为 `*ProcessorError`？
- [ ] 运行期是否做了 I/O？

---

## 11. 相关文档

- 上游：[02-核心领域模型](02-核心领域模型.md) §2 Processor
- 下游：[04-Pipeline与执行顺序](04-Pipeline与执行顺序.md)
- 错误：[10-配置校验与错误体系](10-配置校验与错误体系.md)
- 测试：[15-测试策略与质量工程](15-测试策略与质量工程.md) §2
- Processor Version 1.1 新增 / 1.2 采纳，详见 `01-架构总览与包结构.md`
