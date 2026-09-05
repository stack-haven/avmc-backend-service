# credential — Bearer Token 认证抽象

> Pluggable Bearer token authentication. Business-system agnostic.
> 后端无依赖（除 `go-redis`）、与外部业务系统解耦、支持 5 种开箱即用的 Provider。

---

## 概览 / Overview

`pkg/credential` 提供了一套**与具体业务系统无关**的 Bearer Token 认证抽象。它让 `evie/tool` 可以在不同部署形态下无缝切换身份来源：

| 场景 / Scenario | 推荐 Provider | 说明 / Notes |
|---|---|---|
| 与业务系统共享 Redis（`oauth2_access_token:<token>`） | `redis` | 默认生产配置 |
| 自签 JWT（独立部署） | `jwt` | HS256 / RS256 本地验签 |
| 本地开发 / 演示 | `static` | 配置文件写死 (token, tenant, user) |
| 单元测试 | `static` + `AddUser` | 注入测试 fixture |

`pkg/credential` is a **business-system agnostic** Bearer token authentication abstraction. It lets `evie/tool` switch identity sources across deployment topologies without code changes.

| Deployment | Recommended Provider |
|---|---|
| Shared Redis with business system (`oauth2_access_token:<token>`) | `redis` (default production) |
| Self-issued JWT (standalone deployment) | `jwt` (HS256 / RS256, self-contained) |
| Local development / demo | `static` (config-file tokens) |
| Unit tests | `static` + `AddUser` |

---

## 一、核心概念 / Core Concepts

### 1.1 `Provider` 接口

```go
type Provider interface {
    Name() string
    Authenticate(ctx context.Context, token string) (*CallerIdentity, error)
}
```

所有 Provider 都回答同一个问题：**给定一个 Bearer token，返回对应的调用者身份（或 sentinel error）**。
实现方可以是 Redis、JWT、配置文件、数据库、LDAP、CSV……

Every Provider answers the same question: **"Given this Bearer token, who is the caller?"**
Implementations may be Redis, JWT, config file, database, LDAP, CSV — anything.

### 1.2 `CallerIdentity` 中性视图

```go
type CallerIdentity struct {
    TenantID    string
    UserID      string
    UserName    string
    DeptID      string
    UserType    int32
    Scopes      []string
    ExpiresAt   time.Time
    AccessToken string // echoed for downstream auditing
}
```

字段不带任何业务词（无 qua / oauth2 / employee 等）。下游业务层通过 `pkg/credential/adapter` 桥接到自己的 AuthContext。

The struct carries no business words (no `qua` / `oauth2` / `employee`). Application code bridges through `pkg/credential/adapter` to its own AuthContext.

### 1.3 `FieldMapper` 配置化 JSON 映射

```go
type FieldMapper struct {
    TenantID    string // JSON key for tenant
    UserID      string // JSON key for user
    UserName    string
    DeptID      string
    UserType    string
    ExpiresAt   string // epoch ms / s / RFC3339 都支持
    AccessToken string
}
```

**核心设计**：所有 JSON 字段名都是**配置**而非代码。切换业务系统只需改 YAML，不需要改 Go 代码。
还支持 dot-path（`UserName: "userInfo.nickname"` 自动下钻）。

**Key design**: all JSON field names are **configuration**, not code. Switching business systems requires only YAML changes. Dot-paths are supported (`"userInfo.nickname"` drills down automatically).

### 1.4 Sentinel Errors

| Error | 含义 / Meaning | 何时返回 / When |
|---|---|---|
| `ErrTokenNotFound` | Token 在存储中不存在 / 不存在 | Redis miss / no static user / missing JWT claim |
| `ErrTokenInvalid` | 存在但签名/格式/过期失败 / signature / format / expiry | bad signature, expired, malformed, bad issuer/audience |
| `ErrProviderUnavailable` | 后端存储不可达 / backing store unreachable | Redis down / network timeout |
| `ErrInvalidConfig` | 构造时配置错误 / misconfiguration | missing secret, bad PEM, unsupported alg |

调用方用 `errors.Is(err, credential.ErrTokenNotFound)` 判断语义。

---

## 二、子包结构 / Sub-packages

```
credential/
├── credential.go       Provider 接口 + CallerIdentity + FieldMapper + 4 sentinel errors
├── mapping.go          ParseExpiresAt / LookupPath / ExtractString / ExtractInt32 / MapFromMapper
├── adapter/            ctx 桥接到 application-specific AuthContext
├── redis/              RedisProvider — 共享业务系统 Redis
├── jwt/                JWTProvider — 自签 JWT (HS256/RS256)
├── static/             StaticProvider — 配置文件 / 测试 fixture
└── middleware/         HTTPMiddleware + GRPCUnaryInterceptor
```

---

## 三、Provider 详细 / Provider Details

### 3.1 Redis Provider

读业务系统共享的 Redis（如 `oauth2_access_token:<token>`），按 `FieldMapper` 投影到 `CallerIdentity`。

Reads the shared business-system Redis key (e.g. `oauth2_access_token:<token>`) and projects to `CallerIdentity` via `FieldMapper`.

```go
import "backend-service/app/evie/tool/pkg/credential/redis"

p, err := redis.New(redis.Config{
    Client:    rdb,
    KeyPrefix: "oauth2_access_token:",
    Fields: redis.FieldMapper{
        TenantID:    "tenantId",
        UserID:      "userId",
        UserName:    "userInfo.nickname",
        UserType:    "userType",
        ExpiresAt:   "expiresTime",
        AccessToken: "accessToken",
    },
})
```

| 字段 / Field | 必填 / Required | 说明 / Notes |
|---|---|---|
| `Client` | ✅ | 已连接的 `*redis.Client` |
| `KeyPrefix` | ❌ | 默认 `"oauth2_access_token:"` |
| `Fields` | ✅ | JSON 字段映射；空字段保持 CallerIdentity 零值 |

### 3.2 JWT Provider

**自实现** HS256 / RS256 验签（无第三方 JWT 库依赖），支持 issuer / audience / exp / nbf 校验。

**Self-contained** HS256 / RS256 verification (no third-party JWT library). Supports issuer / audience / exp / nbf checks.

```go
import "backend-service/app/evie/tool/pkg/credential/jwt"

// HS256 (对称密钥)
p, _ := jwt.New(jwt.Config{
    Algorithm: "HS256",
    Secret:    []byte("dev-secret"),
    Issuer:    "evie-tool",
    Audience:  "evie-tool-api",
    Fields: jwt.FieldMapper{
        TenantID: "tenant_id",
        UserID:   "sub",
        UserName: "name",
    },
})

// RS256 (RSA 公钥)
p, _ = jwt.New(jwt.Config{
    Algorithm:    "RS256",
    PublicKeyPEM: []byte(pemBytes),
    Issuer:       "evie-tool",
})
```

| 字段 / Field | 必填 / Required | 说明 / Notes |
|---|---|---|
| `Algorithm` | ❌ | `"HS256"` / `"RS256"`，默认 HS256 |
| `Secret` | HS256 必填 | HMAC 共享密钥 |
| `PublicKeyPEM` | RS256 必填 | PEM 编码 RSA 公钥（PKIX / PKCS1 都支持） |
| `Issuer` | ❌ | 非空时校验 `iss` claim |
| `Audience` | ❌ | 非空时校验 `aud` claim（支持 string / []string） |
| `Leeway` | ❌ | exp/nbf 时钟偏移容忍，默认 0 |
| `Fields` | ✅ | JWT claims 映射（同 redis） |

### 3.3 Static Provider

配置文件中写死 (token, identity) 列表，用于 demo / 单租户部署 / 测试。

In-memory (token, identity) pairs declared in config — for demo / single-tenant / tests.

```go
import "backend-service/app/evie/tool/pkg/credential/static"

p, _ := static.New(static.Config{
    DefaultTenant: "demo",
    Users: []static.User{
        {Token: "demo-token",  TenantID: "demo", UserID: "u1", UserName: "Demo"},
        {Token: "admin-token", TenantID: "demo", UserID: "admin", UserType: 1},
    },
})

// 运行时新增用户 (用于测试 fixture)
p.AddUser(static.User{Token: "test-foo", TenantID: "test", UserID: "foo"})
```

| 字段 / Field | 说明 / Notes |
|---|---|
| `Users` | 接受的 (token, identity) 对 |
| `DefaultTenant` | 当某 User 未指定 TenantID 时填此默认值 |

> **生产部署不应使用 static** — 任何能接触配置文件的人都将获得所有 token。生产请使用 redis / jwt。

> **Do NOT use static in production** — anyone with config-file access obtains all tokens. Use redis or jwt in production.

---

## 四、中间件 / Middleware

### HTTP

```go
import "backend-service/app/evie/tool/pkg/credential/middleware"

mw := middleware.HTTPMiddleware(middleware.Config{
    Provider: provider,
    // HeaderName: "Authorization", // 默认
    // Scheme:     "Bearer ",       // 默认
    // UnauthorizedStatus: 401,     // 默认
})

handler := mw(next)
```

### gRPC (Unary)

```go
import "backend-service/app/evie/tool/pkg/credential/middleware"

interceptor := middleware.GRPCUnaryInterceptor(middleware.Config{
    Provider: provider,
})

server := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
```

### 取出身份 / Retrieve identity

```go
// 标准方式 / canonical
id, ok := middleware.FromContext(ctx)

// 应用代码推荐方式（携带业务语义）
id, ok := adapter.AuthFrom(ctx)
```

### 配置字段 / Config Fields

| 字段 / Field | 默认 / Default | 说明 / Notes |
|---|---|---|
| `Provider` | — (必填) | `credential.Provider` 实例 |
| `HeaderName` | `"Authorization"` | HTTP 头部名 |
| `Scheme` | `"Bearer "` | 期望前缀（含尾部空格） |
| `UnauthorizedStatus` | `401` | HTTP 拒绝时返回的状态码 |
| `MetadataHeader` | `"authorization"` | gRPC metadata key |

---

## 五、桥接到业务 ctx / Application Adapter

`pkg/credential` 不带任何业务词。`adapter` 子包提供 ctx 桥接：

The `pkg/credential` package carries no business words. The `adapter` sub-package bridges to application contexts:

```go
import (
    "backend-service/app/evie/tool/pkg/credential"
    "backend-service/app/evie/tool/pkg/credential/adapter"
)

// 把身份放入 ctx
ctx := adapter.WithAuth(ctx, &credential.CallerIdentity{
    TenantID: "t1", UserID: "u1",
})

// 从 ctx 取出
id, ok := adapter.AuthFrom(ctx)
if ok && id.TenantID == "t1" {
    // ...
}

// 把身份复制到新的 ctx (例如 gRPC metadata 转发)
outCtx := adapter.CopyAuthContext(outCtx, inCtx)
```

---

## 六、辅助函数 / Mapping Helpers

位于 `mapping.go`，所有 Provider 共用。

Located in `mapping.go`, shared by all providers.

| 函数 / Function | 用途 / Purpose |
|---|---|
| `ParseExpiresAt(v any) (time.Time, bool)` | 解析上游过期时间；支持 epoch ms、epoch s、RFC3339 |
| `LookupPath(m map[string]any, path string) any` | 点路径下钻（`"userInfo.nickname"` → `m["userInfo"]["nickname"]`） |
| `ExtractString(v any) string` | 安全地转 string（容忍 number / bool） |
| `ExtractInt32(v any) (int32, bool)` | 安全地转 int32 |
| `MapFromMapper(root map[string]any, m FieldMapper) CallerIdentity` | 把 map 通过 FieldMapper 投影成 CallerIdentity |

---

## 七、选择 Provider 决策表 / Choosing a Provider

```
                            ┌─────────────────────┐
                            │ Have shared Redis?   │
                            └──────────┬──────────┘
                                yes ↓       ↓ no
              ┌─────────────────────────┐   │
              │ Use redis.New(...)       │   │
              │ with oauth2_access_token │   │
              └─────────────────────────┘   │
                                             ↓
                                  ┌─────────────────────┐
                                  │ Self-issued JWT?    │
                                  └──────────┬──────────┘
                                      yes ↓       ↓ no
                        ┌─────────────────────┐  │
                        │ Use jwt.New(HS256/RS256)│ │
                        └─────────────────────┘  │
                                                  ↓
                                       ┌─────────────────────┐
                                       │ Local dev / demo?   │
                                       └──────────┬──────────┘
                                           yes ↓       ↓ no
                             ┌─────────────────────┐  │
                             │ Use static.New(Users) │  │
                             └─────────────────────┘  │
                                                        ↓
                                          ┌─────────────────────────┐
                                          │ Need custom backend?    │
                                          └──────────┬──────────────┘
                                              yes ↓          ↓ no (review!)
                                            ┌────────────────────────┐
                                            │ Implement your own      │
                                            │ credential.Provider     │
                                            └────────────────────────┘
```

---

## 八、最佳实践 / Best Practices

1. **生产用 Redis / JWT，不用 Static** — Static 的 token 列表会进配置文件 / 镜像，泄漏即破防。
2. **始终用 `errors.Is`** 判断 sentinel — 不要 string-match 错误信息。
3. **`Provider` 是接口** — 测试时注入 `static` + `AddUser`，无需启动 Redis。
4. **FieldMapper 留空即保留零值** — 不用每个字段都映射，按需声明。
5. **adapter.AuthFrom 是 ctx 取身份的首选** — 比 `middleware.FromContext` 更明确表达"应用层 auth"语义。

1. **Use redis / jwt in production, never static** — Static's tokens land in config files / images.
2. **Always use `errors.Is`** to check sentinels — never string-match error messages.
3. **`Provider` is an interface** — inject `static` + `AddUser` in tests, no Redis required.
4. **Empty FieldMapper entries preserve zero values** — declare only what you need.
5. **Prefer `adapter.AuthFrom` for application code** — clearer than `middleware.FromContext`.

---

## 九、测试覆盖 / Test Coverage

| 包 / Package | 测试 / Tests | 关注点 / Coverage |
|---|---|---|
| `pkg/credential` (mapping) | 8 sub-tests | dot-path / 类型强转 / epoch ms vs s |
| `pkg/credential/redis` | (via miniredis) | key prefix / 字段缺失 / 服务不可达 |
| `pkg/credential/jwt` | 10 sub-tests | HS256 happy/bad-sig/expired/issuer/malformed/alg-mismatch, RS256 解析 |
| `pkg/credential/static` | 5 sub-tests | 用户查找 / 大小写不敏感 / AddUser |
| `pkg/credential/middleware` | 6 sub-tests | HTTP Bearer 解析 / gRPC metadata / FromContext |

所有测试通过 `go test -race` 验证。

All tests pass `go test -race`.

---

## 十、不在范围 / Out of Scope

- **OAuth2 授权码流程** — 本包只做 Bearer Token 校验，不做 token 签发。
- **会话管理 / 刷新 token** — `CallerIdentity.AccessToken` 仅用于审计回传，不参与刷新逻辑。
- **权限策略 (Casbin)** — 上层 `pkg/auth/authz` 处理 ABAC / RBAC。

- **OAuth2 authorization-code flow** — this package only verifies Bearer tokens, never issues them.
- **Session management / refresh tokens** — `CallerIdentity.AccessToken` is for auditing only.
- **Authorization policies (Casbin)** — handled by the upper `pkg/auth/authz` layer.

---

**License**: Apache 2.0（同 evie/tool 主仓库）
**Status**: ✅ v1 — 5 个 Provider 全部可用，11 个测试包全绿