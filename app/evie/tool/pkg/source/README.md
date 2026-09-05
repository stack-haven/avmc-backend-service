# source — Vocabulary Source 抽象

> Pluggable vocabulary source adapters. Fetch opaque entities from external systems
> (HTTP / file / qua) and project them through YAML rules into normalized entries.
> 通用 vocabulary 数据源抽象：HTTP / 文件 / qua 三种开箱即用实现。

---

## 概览 / Overview

`pkg/source` 让 `evie/tool` 可以从**任意业务系统**拉取用户/部门/产品/术语，并把它们统一投影成 `RawEntity`，再经由 Normalizer (YAML 规则) 转成 `NormalizedEntry`，最后 `VocabularyBuilder` 装配到引擎里。

新增业务系统只需实现一个 `Source.Fetch(ctx)`，或者注册一个 Factory。

`pkg/source` lets `evie/tool` pull users/depts/products/terms from **any business system** and project them uniformly into `RawEntity`. A Normalizer (YAML rules) then maps to `NormalizedEntry`, which the `VocabularyBuilder` ingests.

Add a new business system by implementing `Source.Fetch(ctx)` or registering a `Factory`.

| 实现 / Implementation | 文件 / File | 适用场景 / Scenario |
|---|---|---|
| **HTTP**（通用） | `pkg/source/http` | 任何 REST API + Spring Cloud / RuoYi / 自研 JSON API |
| **File** | `pkg/source/file` | JSON 离线文件 / demo / 单元测试 |
| **qua** | `pkg/source/qua` | qua 系统（参考实现；推荐用 http 替代） |

---

## 一、核心概念 / Core Concepts

### 1.1 `Source` 接口

```go
type Source interface {
    Name() string
    Fetch(ctx context.Context) ([]RawEntity, error)
}
```

任何 vocabulary 数据源都回答同一个问题：**"给我当前所有可知的实体（用户、部门、词条……）"**。
实现方可以是 HTTP、文件、数据库、消息队列。

### 1.2 `RawEntity` 不透明 payload

```go
type RawEntity struct {
    SourceID   string         // 上游系统的稳定 ID（如 qua 的 row id）
    EntityType string         // 实体分类（"user" / "department" / "product" 等）
    Source     string         // 数据源名（来自 Source.Name()，便于日志追溯）
    Data       map[string]any // 不透明的 JSON payload
}
```

**关键设计**：`Data` 是不透明 map。下游 Normalizer 决定哪些字段有意义、怎么转换——不是 Source。
这样 Source 包不依赖任何业务系统字段名。

**Key design**: `Data` is an opaque map. The downstream Normalizer decides which fields matter, not the Source. This keeps the Source package free of business-system field names.

### 1.3 Factory 注册中心

```go
type Factory func(cfg map[string]any) (Source, error)

func Register(name string, f Factory)
func Build(name string, cfg map[string]any) (Source, error)
func Names() []string
```

通过 YAML `name:` 字段动态选择实现。新增实现只需 `Register("my-adapter", factory)`。

### 1.4 数据流 / Data Flow

```
┌──────────────────┐
│ External System  │   (qua / Feishu / LDAP / OIDC / CSV / ...)
└────────┬─────────┘
         │ HTTP / file / DB
         ↓
┌──────────────────┐
│ pkg/source/Source│   returns []RawEntity (opaque)
└────────┬─────────┘
         │
         ↓
┌─────────────────────────┐
│ biz.VocabularyNormalizer│   YAML rules → []NormalizedEntry (canonical)
└────────┬────────────────┘
         │
         ↓
┌─────────────────────────┐
│ VocabularyBuilder       │   builds per-tenant in-memory lexicon
└─────────────────────────┘
```

---

## 二、子包结构 / Sub-packages

```
source/
├── source.go        Source 接口 + RawEntity + Factory 注册中心
├── adapter/         biz.VocabularySource 桥接（ctx-aware TenantIDProvider）
├── http/            通用 HTTP adapter（任何 JSON API）
├── file/            JSON 文件 adapter（离线 / demo / 测试）
└── qua/             qua 系统参考实现（薄包装 http）
```

---

## 三、HTTP Adapter（通用）

`pkg/source/http` 是**通用 HTTP vocabulary 适配器**。通过配置可对接任意 REST API。

It is a **generic HTTP vocabulary adapter** that connects to any REST API via configuration.

### 3.1 快速开始

```go
import "backend-service/app/evie/tool/pkg/source/http"

p, err := http.New(http.Config{
    BaseURL:  "https://qua.example.com",
    UserPath: "/admin-api/qua/member-extended/page?selectAll=true",
    DeptPath: "/admin-api/system/dept/list",

    // ctx-aware: 从 ctx 提取 caller token 透传给上游
    TokenProvider: http.TokenFunc(func(ctx context.Context) (string, error) {
        id, ok := adapter.AuthFrom(ctx)
        if !ok { return "", errors.New("no auth") }
        return id.AccessToken, nil
    }),

    // ctx-aware: 从 ctx 提取 tenant id 加到 header
    TenantHeader: "tenant-id",
    TenantIDProvider: http.TenantIDFunc(func(ctx context.Context) (string, error) {
        id, ok := adapter.AuthFrom(ctx)
        if !ok { return "", errors.New("no auth") }
        return id.TenantID, nil
    }),

    // 上游响应壳（Spring Cloud 风格）
    Envelope: http.Envelope{
        UsersPath: "data.list", // dotted path
        DeptsPath: "data",
        CodePath:  "code",
        CodeOK:    0,
    },

    UserEntityType: "user",
    DeptEntityType: "department",

    // 业务错误码 → 错误
    CodeErrorMap: map[int]func(code int, msg string) error{
        400: func(_ int, msg string) error { return fmt.Errorf("bad request: %s", msg) },
        401: func(_ int, msg string) error { return fmt.Errorf("unauthorized: %s", msg) },
    },

    Timeout: 30 * time.Second,
})
```

### 3.2 `Config` 字段全表 / Config Field Reference

| 字段 / Field | 必填 / Required | 说明 / Notes |
|---|---|---|
| `BaseURL` | ✅ | 上游 API 根地址 |
| `UserPath` | ✅ | 用户列表端点（可带 query string） |
| `DeptPath` | ✅ | 部门列表端点 |
| `Method` | ❌ | 默认 `"GET"` |
| `Body` | ❌ | POST/PUT 请求体 |
| `QueryParams` | ❌ | 合并到 URL 的额外 query 参数 |
| `Headers` | ❌ | 静态 header（`zone: cn-north` 等） |
| `AuthHeader` | ❌ | 默认 `"Authorization"` |
| `AuthScheme` | ❌ | 默认 `"Bearer "` |
| `TokenProvider` | ❌ | ctx-aware bearer token 提供者 |
| `TenantHeader` | ❌ | 启用 tenant header 时填写（如 `"tenant-id"`） |
| `TenantIDProvider` | ❌ | ctx-aware tenant id 提供者 |
| `Envelope` | ❌ | 响应壳路径解析 |
| `IDKey` | ❌ | 实体 ID 的 JSON key，默认 `"id"` |
| `UserEntityType` | ✅ | 写入 `RawEntity.EntityType`（如 `"user"`） |
| `DeptEntityType` | ✅ | 同上（`"department"`） |
| `CodeErrorMap` | ❌ | 业务错误码 → `error` 工厂 |
| `Timeout` | ❌ | HTTP client 超时（默认 30s） |

### 3.3 `Envelope` 响应壳解析

很多业务系统用 `{code, msg, data}` 风格：

```go
Envelope: http.Envelope{
    UsersPath: "data.list", // dotted path 下钻到用户数组
    DeptsPath: "data",      // dotted path 下钻到部门数组
    CodePath:  "code",
    CodeOK:    0,
}
```

也支持非壳 / 部分壳（如 list 直接在根）。把 `UsersPath`/`DeptsPath` 设为 `""` 跳过。

### 3.4 公开方法 / Public Methods

| Method | 说明 / Notes |
|---|---|
| `Fetch(ctx) ([]RawEntity, error)` | 拉取用户 + 部门，返回合并的 `[]RawEntity` |
| `FetchWithCtx(ctx, path string) ([]map[string]any, error)` | 拉取**单端点** + ctx-aware header（qua 用） |
| `UserPath() string` | 返回配置的 UserPath（debug / 日志） |
| `DeptPath() string` | 同上 |

---

## 四、File Adapter（离线 / demo）

`pkg/source/file` 读 JSON 文件列表。**完全离线**，适合 demo / 单元测试 / 单租户离线部署。

Reads JSON files. **Fully offline** — for demo / unit tests / single-tenant offline.

### 4.1 配置文件格式

```json
{
  "version": "v1",
  "users": [
    {"id": "u1", "name": "Alice", "dept_id": "d1"},
    {"id": "u2", "name": "Bob",   "dept_id": "d1"}
  ],
  "departments": [
    {"id": "d1", "name": "Engineering"}
  ]
}
```

### 4.2 使用

```go
import "backend-service/app/evie/tool/pkg/source/file"

p, err := file.New(file.Config{
    Path:    "/etc/evie-tool/dictionaries/demo_vocab.json",
    IDKey:   "id",               // 可选，默认 "id"
    Watch:   true,               // 启用 fsnotify 热重载
})
```

### 4.3 特性 / Features

- **热重载**（可选）：文件变更后 5 秒内重新加载（需 `Watch: true`）
- **YAML/JSON 都支持**：通过扩展名识别
- **不透明 payload**：`Data` 字段保留所有非 id 字段，下游 Normalizer 决定意义

---

## 五、qua Adapter（参考实现）

`pkg/source/qua` 是 qua 系统的**薄包装**——内部委托 `http` adapter，把 qua 协议特化到 Config 中。

It is a **thin wrapper** over the `http` adapter that bakes qua's protocol conventions into `Config`.

```go
import "backend-service/app/evie/tool/pkg/source/qua"

p, _ := qua.New(qua.Config{
    BaseURL:  "https://qua.example.com",
    UserPath: "/admin-api/qua/member-extended/page?selectAll=true",
    DeptPath: "/admin-api/system/dept/list",
    Tokens:   qua.TokenFunc(func(ctx context.Context) (string, error) { ... }),
    Headers:  map[string]string{"zone": "cn-north"},
})
```

> **新部署建议直接用 http adapter**。qua 包作为**现有用户的参考实现**保留。

> **New deployments should use the http adapter directly**. The qua package is preserved as a **working reference implementation** for existing users.

### 5.1 qua 协议特化

`qua.New` 自动设置：
- `Envelope: {UsersPath: "data.list", DeptsPath: "data", CodePath: "code", CodeOK: 0}`
- `UserEntityType: "user"`
- `DeptEntityType: "department"`

其它可定制字段：`Headers` / `TenantHeader` / `TenantID`。

---

## 六、Adapter 桥接（业务 ctx 集成）

`pkg/source/adapter` 把通用 `source.Source` 桥接到 biz 层 `VocabularySource`：

`pkg/source/adapter` bridges the generic `source.Source` to the biz-layer `VocabularySource`:

```go
import (
    "backend-service/app/evie/tool/pkg/source"
    "backend-service/app/evie/tool/pkg/source/adapter"
)

src, _ := http.New(...)
vocabSrc := adapter.Wrap(src)  // → biz.VocabularySource
```

桥接后下游可以注入到 `biz.VocabularyBuilder` / `biz.VocabularySync` 等。

---

## 七、添加新 Adapter / Adding a New Adapter

实现 `Source` 接口 + 注册 Factory：

Implement `Source` and register a `Factory`:

```go
// 1. 实现
type MyAdapter struct{}
func (a *MyAdapter) Name() string { return "my-adapter" }
func (a *MyAdapter) Fetch(ctx context.Context) ([]source.RawEntity, error) {
    // ...
}

// 2. 注册（通常在 init()）
func init() {
    source.Register("my-adapter", func(cfg map[string]any) (source.Source, error) {
        // 反序列化 cfg，构造 MyAdapter
        return &MyAdapter{}, nil
    })
}

// 3. YAML 配置即生效
// sources:
//   - name: my-adapter
//     config: {...}
```

或者用 `Build(name, cfg)` 在运行时构造：

```go
src, err := source.Build("my-adapter", map[string]any{
    "endpoint": "https://...",
})
```

---

## 八、错误处理 / Error Handling

HTTP adapter 错误分两类：

| 类别 / Category | 触发条件 / Trigger | 含义 / Meaning |
|---|---|---|
| **网络错误** | DNS / TCP / TLS / timeout | 网络层失败 |
| **业务错误** | `Code != CodeOK`（按 `CodeErrorMap` 映射） | 上游返回业务失败 |
| **响应解析错误** | JSON 解析失败 / 路径下钻失败 | 协议不一致 |

调用方按需区分 retry / 熔断 / 透传。

---

## 九、测试覆盖 / Test Coverage

| 包 / Package | 测试 / Tests | 覆盖点 / Coverage |
|---|---|---|
| `pkg/source/http` | 10 sub-tests | httptest mock、token/tenant header、code error、QueryParams |
| `pkg/source/file` | 8 sub-tests | JSON 解析、热重载、缺字段、空数组 |
| `pkg/source/qua` | (via integration) | qua 协议特化、tokens/headers 转发 |
| `pkg/source/adapter` | (compile-time) | 桥接类型一致性 |

所有测试通过 `go test -race` 验证。

All tests pass `go test -race`.

---

## 十、最佳实践 / Best Practices

1. **优先用 http adapter** — 任何支持 REST + JSON 的系统都能接入，避免新增 adapter。
2. **`RawEntity.Data` 保持不透明** — 不要在 Source 包里 import 任何业务系统的结构体。
3. **业务错误码用 `CodeErrorMap`** — 让上游错误语义向下游传递，不要吞掉。
4. **`TokenProvider` / `TenantIDProvider` 是 ctx-aware** — 不要在 Source 内部用全局变量取 caller 信息。
5. **qua 仅作参考** — 新部署配置 http 即可；qua 包保留只是为了兼容既有用户。

1. **Prefer the http adapter** — any REST + JSON system fits without a new adapter.
2. **Keep `RawEntity.Data` opaque** — never import a business-system struct inside the Source package.
3. **Map business error codes via `CodeErrorMap`** — propagate upstream semantics, never swallow.
4. **TokenProvider / TenantIDProvider are ctx-aware** — no globals inside Source.
5. **qua is a reference** — new deployments configure http; qua exists for backward compatibility.

---

## 十一、不在范围 / Out of Scope

- **增量同步（CDC / webhook）** — 本包只做**全量拉取**。增量同步在 `biz/vocab_sync.go` 实现。
- **字段映射规则（拼音化、单位转换）** — 在 `biz/vocab_normalizer.go` 的 YAML 规则里。
- **缓存** — 在 `biz/vocabulary.go` 的 VocabularyBuilder 里。

- **Incremental sync (CDC / webhooks)** — this package does **full pull** only. Incremental sync lives in `biz/vocab_sync.go`.
- **Field-mapping rules (pinyin, unit conversion)** — YAML rules in `biz/vocab_normalizer.go`.
- **Caching** — in `biz/vocabulary.go` VocabularyBuilder.

---

**License**: Apache 2.0（同 evie/tool 主仓库）
**Status**: ✅ v1 — http / file / qua 三种实现 + adapter 桥接，13 个测试全绿