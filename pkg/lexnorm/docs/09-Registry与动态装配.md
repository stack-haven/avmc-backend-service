# 09 · Registry 与动态装配

> 源节：§24 Registry
> 适用阶段：Phase 8
> 受众：扩展开发者（写自定义 Processor 并通过 JSON/YAML 装配的人）
> 关键性：**Registry 不是 Processor 使用前提**，仅供动态装配场景使用

---

## 1. Registry 定位（§31）

Registry 是**可选的**动态装配机制。

**它不是 Processor 使用前提。**

### 普通代码不需要 Registry

```go
pipeline := lexnorm.NewPipeline(
    clean.New(),
    alias.New(lexicon),
)
```

不需要 Registry。

### Registry 主要用于

- JSON 配置
- YAML 配置
- 动态配置
- 插件发现
- 服务端装配

---

## 2. Registry 设计原则（§33）

### 禁止

```text
Registry → Processor → Registry
```

Processor 本身**不得依赖** Registry。

### Registry 职责

```text
Name
 ↓
Descriptor
 ↓
Processor
```

只有**配置 → Processor 构造**这一条路径。

---

## 3. Descriptor

```go
type Descriptor struct {
    Name      string
    Certainty Certainty            // 1.2 决议: CertaintyLevel → Certainty（与 1.1 一致）
    New       func(json.RawMessage) (Processor, error)
    Default   func() any           // 1.1 新增：默认配置（用于文档生成）
}
```

### 字段说明

| 字段 | 含义 | 必填 |
|---|---|:--:|
| Name | Processor 唯一标识 | ✅ |
| Certainty | 自声明的确定性级别（用于 Preset 编排参考） | ✅ |
| New | 从 JSON 配置构造 Processor 实例的工厂函数 | ✅ |
| Default | 返回默认配置（用于文档生成） | 否 |

### 约束

- **Name 全局唯一**（按 Registry 实例范围）
- **New 必须是纯函数**：相同输入产生等价 Processor（满足确定性 §2.4）
- **New 内部不得引用 Registry**：避免循环依赖
- **New 失败必须返回 error**：不静默回退

---

## 4. Registry API

```go
type Registry struct {
    // private
}

func NewRegistry() *Registry

func (r *Registry) Register(
    descriptor Descriptor,
) error

func (r *Registry) Create(
    name string,
    config json.RawMessage,
) (Processor, error)
```

### Register 行为

| 情况 | 行为 |
|---|---|
| Name 不存在 | 注册成功 |
| Name 已存在 | 返回 error（**不覆盖**） |
| Descriptor 字段非法 | 返回 error |
| New 为 nil | 返回 error |

### Create 行为

| 情况 | 行为 |
|---|---|
| Name 已注册 | 调用 New(config) |
| Name 未注册 | 返回 `ErrProcessorNotFound` |
| New 返回 error | 透传 error |
| config 格式错误 | New 内部校验，error 透传 |

---

## 5. JSON 配置示例

```yaml
processors:
  - name: clean
    config: {}

  - name: alias
    config:
      lexicon_ref: "default"

  - name: fuzzy
    config:
      auto_threshold: 0.80
      suggest_threshold: 0.65
      categories:
        PERSON: { auto: 0.65, suggest: 0.55 }
```

### 装配流程

```text
YAML → struct
  ↓
for each spec:
  registry.Create(name, json.RawMessage(config))
  ↓
[]Processor
  ↓
lexnorm.NewPipeline(...)
```

---

## 6. 自定义 Processor 通过 Registry 接入

### 用户侧代码

```go
// myproc/myproc.go
package myproc

import (
    "context"
    "encoding/json"
    "github.com/stack-haven/lexnorm"
)

type UpperProcessor struct{}

func (UpperProcessor) Name() string { return "upper" }
func (UpperProcessor) Process(ctx context.Context, s *lexnorm.State) error {
    // ...
    return nil
}

var Descriptor = lexnorm.Descriptor{
    Name: "upper",
    Certainty: lexnorm.CertaintyDeterministic,
    New: func(cfg json.RawMessage) (lexnorm.Processor, error) {
        return UpperProcessor{}, nil
    },
}
```

### 接入 Registry

```go
reg := lexnorm.NewRegistry()
if err := reg.Register(myproc.Descriptor); err != nil {
    log.Fatal(err)
}

// 由配置创建 Pipeline
processors, err := loadProcessorsFromYAML(reg, "config.yaml")
pipeline := lexnorm.NewPipeline(processors...)
```

---

## 7. 内置 Processor 也通过 Descriptor 注册

为了让 YAML 装配可工作，**内置 Processor 必须**也注册到默认 Registry：

```go
// processor/register.go
func RegisterBuiltin(r *Registry) error {
    for _, d := range []Descriptor{
        clean.Descriptor,
        disfluency.Descriptor,
        alias.Descriptor,
        deterministic.Descriptor,
        pinyin.Descriptor,
        fuzzy.Descriptor,
        context.Descriptor,
    } {
        if err := r.Register(d); err != nil {
            return err
        }
    }
    return nil
}
```

### 默认 Registry

```go
func DefaultRegistry() *Registry {
    r := NewRegistry()
    _ = RegisterBuiltin(r)
    return r
}
```

> 但 `NewEngine` 仍然接受直接传入 `*Pipeline`（**不强制**走 Registry），
> 普通用法不感知 Registry 存在。

---

## 8. Registry 与 Engine 的关系

```text
┌──────────────┐    Create(name, cfg)     ┌──────────────┐
│   Registry   │ ───────────────────────► │  Processor   │
└──────────────┘                          └──────────────┘
       ▲                                         │
       │ Register(Descriptor)                    │
       │                                         ▼
   用户 / YAML loader                      NewPipeline(...)

Engine 不感知 Registry
```

### 关键约束（再强调）

| 项 | 状态 |
|---|---|
| Engine 是否依赖 Registry | ❌ 否 |
| Processor 是否依赖 Registry | ❌ 否 |
| Pipeline 是否依赖 Registry | ❌ 否 |
| 普通 NewPipeline 是否需要 Registry | ❌ 否 |

> Registry 是**装配期工具**，不是**运行期依赖**。

---

## 9. 测试要求

| 测试 | 内容 |
|---|---|
| Register 已存在 Name | 返回 error，不覆盖 |
| Create 未注册 Name | 返回 `ErrProcessorNotFound` |
| New 返回 error | 透传，不 panic |
| 并发 Register | 数据竞争检测（可用 mutex / sync.Map） |
| 默认 Registry | 内置 7 个 Descriptor 全部可注册成功 |

---

## 10. 自检清单

- [ ] 是否让 Engine / Pipeline / Processor 反向依赖 Registry？
- [ ] Descriptor.New 是否为纯函数？
- [ ] Register 同名是否返回 error 而非覆盖？
- [ ] 内置 Processor 是否都注册到默认 Registry？
- [ ] Create 未注册 Name 是否返回 sentinel error？
- [ ] New 函数 panic 是否有 recover 兜底（避免污染 Registry）？

---

## 11. 相关文档

- 上游：[03-Processor接口与生命周期](03-Processor接口与生命周期.md)
- 配置：[10-配置校验与错误体系](10-配置校验与错误体系.md)
- 应用场景：[13-应用场景与Pipeline模板](13-应用场景与Pipeline模板.md)
