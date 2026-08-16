# auth · 通用认证鉴权工具包

面向多租户微服务的 **认证（Authentication）与鉴权（Authorization）** 公共库，基于 Go + go-kratos 生态。

提供 JWT/OIDC 本地认证、Casbin 鉴权、Redis 会话管理、登录防护与开箱即用的 HTTP/gRPC 中间件，具备 Provider 插件化、多租户感知、安全加固与高可用降级能力。

---

## 特性

- **认证 / 鉴权分离**：`authn`（你是谁）与 `authz`（你能做什么）独立抽象，边界清晰
- **Provider 插件化**：认证支持 JWT / OIDC / Noop，鉴权支持 Casbin；通过 `init()` 自动注册，按名称创建
- **多租户感知**：`tenant` 贯穿 Token、Session、Policy 全链路
- **完整会话管理**：Redis 持久化、在线监控、踢下线、Token 轮换，均内置
- **安全加固**：Token 类型区分（access/refresh）、密钥轮换（kid）、时钟偏移容差（leeway）、登录失败锁定、常量时间比较
- **高可用降级**：Redis 故障时可配置 `fail-open`（降级为仅 JWT 验签），默认 `fail-closed`（安全优先）
- **类型安全 Claims**：链式 Setter 消除魔法字符串

---

## 目录结构

```
pkg/auth/
├── authn/                  # 认证：身份验证 + Token 签发/验签
│   ├── authenticator.go    # Authenticator / TokenManager / SecurityUser 接口
│   ├── provider.go         # Provider 注册机制
│   ├── claims.go           # Claims + 类型安全 Setter
│   ├── security.go         # SecurityUser 工厂
│   ├── forward.go          # 跨服务 Token 转发
│   ├── jwt/                # JWT Provider
│   ├── oidc/               # OIDC Provider
│   └── noop.go             # Noop Provider（测试/降级）
├── authz/                  # 鉴权：权限判断
│   ├── enforcer.go         # Enforcer（核心最小契约）
│   ├── policy.go           # PolicyManager（策略管理）
│   ├── role.go             # RoleManager（RBAC 关系）
│   ├── authorizer.go       # Authorizer = Enforcer + PolicyManager + RoleManager
│   ├── provider.go         # Provider 注册机制
│   └── casbin/             # Casbin Provider
├── session/                # 会话：Redis 持久化 / 在线监控 / 踢下线
│   └── manager.go          # Manager + Store + Session/Info
├── loginattempt/           # 登录防护（失败锁定）
├── middleware/             # HTTP/gRPC 中间件（kratos 集成）
├── errs/                   # 统一错误结构
├── examples/               # 使用示例
└── factory.go              # 认证工厂（高层封装）
```

---

## 快速开始

### 1. 创建认证器（JWT 本地验签）

```go
import (
    "backend-service/pkg/auth"
    "backend-service/pkg/auth/authn"
    _ "backend-service/pkg/auth/authn/jwt" // blank import 触发 JWT Provider 注册
)

// 方式 A：高层工厂（推荐）
authenticator, err := auth.NewAuthenticator(auth.AuthConfig{
    Key:               "your-shared-secret-key", // 所有服务共享
    Method:            "HS256",
    AccessExpiration:  24 * time.Hour,
    RefreshExpiration: 7 * 24 * time.Hour,
}, authn.NewSecurity(logger))

// 方式 B：Provider 按名称创建
authenticator, err = authn.NewAuthenticator("jwt", ctx,
    authn.WithSigningKey([]byte("your-shared-secret-key")),
    authn.WithSigningMethod("HS256"),
    authn.WithIssuer("your-service"),
    authn.WithAudience("your-clients"),
)
```

### 2. 创建鉴权器（Casbin）

```go
import (
    "backend-service/pkg/auth/authz"
    _ "backend-service/pkg/auth/authz/casbin"
)

authorizer, err := authz.NewAuthorizer("casbin", ctx,
    authz.WithAdapterType(authz.AdapterMySQL),
    authz.WithAdapterDSN("root:pass@tcp(127.0.0.1:3306)/platform_system"),
)
```

### 3. 创建会话管理器（Redis 持久化）

```go
import "backend-service/pkg/auth/session"

manager := session.NewManager(redisClient, logger, authenticator,
    session.WithFailOpen(false), // 可选：Redis 故障降级策略
)
```

### 4. 注册中间件

```go
import authMiddleware "backend-service/pkg/auth/middleware"

// HTTP / gRPC 中间件链
selector.Server(
    authMiddleware.AuthnMiddleware(manager),    // 认证（JWT + session 校验）
    authMiddleware.AuthzMiddleware(authorizer), // 鉴权（Casbin）
).Match(matcher).Build()
```

---

## 核心概念

### 认证（authn）

| 接口 | 职责 |
|------|------|
| `Authenticator` | 认证器：`Authenticate`（从上下文验签）+ `CreateToken`/`ValidateToken` |
| `AuthProvider` | 认证提供者：`Name` + `NewAuthenticator` |
| `SecurityUser` | 安全用户：从 Claims 解析出 subject/tenant/object/action |

**Provider 注册**：各 Provider 包通过 `init()` 调用 `authn.RegisterProvider` 自动注册，使用者 `import _` 后即可 `authn.NewAuthenticator("jwt", ...)` 按名称创建。

### 鉴权（authz）

接口按能力拆分：

| 接口 | 职责 | 实现者 |
|------|------|--------|
| `Enforcer` | `Enforce` / `BatchEnforce`（核心） | 所有实现 |
| `PolicyManager` | 策略增删查 | 本地引擎（Casbin） |
| `RoleManager` | 用户-角色 RBAC 关系 | 本地引擎（Casbin） |
| `Authorizer` | 组合以上三者 + 生命周期 | 完整实现 |

**委托型实现只需 `Enforcer`**：跨服务 gRPC 委托鉴权时，仅实现 `Enforce` 即可，无需背负策略/RBAC 管理。

### 会话（session）

`session.Manager` 提供：

- **持久化**：Redis 存储 access/refresh token 与会话元数据
- **在线监控**：`ListUserSessions` / `ListTenantSessions`
- **踢下线**：`RevokeSession` / `RevokeUserSessions` / `RevokeTenantSessions`
- **Token 轮换**：`RotateSessionToken`（保留会话，轮换令牌对）

### Claims

```go
claims := authn.AuthClaims{}.
    SetID(sessionID).
    SetSubject(userID).
    SetTenant(tenantID).
    SetPlatformOperator(true).
    SetTokenType(authn.TokenTypeAccess)

// 读取
claims.GetSubject()      // 主体
claims.GetTenant()       // 租户
claims.GetTokenType()    // access / refresh
claims.IsPlatformOperator()
```

---

## 安全特性

| 特性 | 说明 |
|------|------|
| **Token 类型区分** | access/refresh 分离，`RefreshToken` 强制校验类型，防止混用 |
| **密钥轮换（kid）** | 支持 `VerificationKeys`（kid→key），签名带 kid 头，验证按 kid 查找 |
| **时钟偏移容差** | `WithLeeway(d)` 容忍分布式节点时钟不同步 |
| **登录失败锁定** | `loginattempt.RedisGuard`，Lua 原子脚本防并发绕过 |
| **常量时间比较** | session token 校验用 `subtle.ConstantTimeCompare` 防时序攻击 |
| **算法混淆防护** | 验证时强制校验签名方法白名单 |
| **issuer / audience 校验** | 强制校验签发者与接收者 |

---

## 高可用

`session.Manager` 区分「session 不存在」与「Redis 连接故障」：

| Redis 状态 | 行为 |
|-----------|------|
| `redis.Nil`（session 不存在） | 拒绝（token 已踢下线/过期） |
| 连接故障 | 按 `WithFailOpen` 决定：`true` 降级为仅 JWT 验签 / `false` 拒绝 |

```go
// 可用性优先：Redis 故障时降级为仅 JWT 验签
manager := session.NewManager(rdb, logger, authenticator, session.WithFailOpen(true))
```

> 注意：即使 `fail-open`，被踢下线的 token 仍会拒绝——因为那是 `redis.Nil` 而非连接故障。

---

## 扩展指南

### 新增认证 Provider

```go
package myprovider

import "backend-service/pkg/auth/authn"

type MyProvider struct{}

func init() {
    authn.RegisterProvider(&MyProvider{})
}

func (p *MyProvider) Name() string { return "my-provider" }

func (p *MyProvider) NewAuthenticator(ctx context.Context, opts ...authn.Option) (authn.Authenticator, error) {
    // 实现你的认证逻辑
}
```

使用者：

```go
import _ "your/module/myprovider"
authenticator, _ := authn.NewAuthenticator("my-provider", ctx, opts...)
```

### 新增鉴权 Provider

同理，实现 `authz.AuthzProvider` 接口（或仅 `authz.Enforcer` 用于委托场景），`init()` 注册即可。

---

## 错误体系

统一错误结构在 `errs` 子包：

```go
import "backend-service/pkg/auth/errs"

err := errs.New(authn.ErrCodeInvalidToken, "invalid token", cause)
errs.Is(err)            // 判断是否统一错误
errs.GetCode(err)       // 获取错误码
```

- `authn.AuthError` / `authz.AuthzError` 均为 `errs.Error` 的类型别名
- 中间件将内部错误码映射为 kratos errors（HTTP 状态码 + reason）

---

## 示例

完整可运行示例见 [`examples/example.go`](examples/example.go)，覆盖 JWT 认证器、Casbin 鉴权器、中间件链的组装。

---

## 约定

1. **JWT 密钥全平台共享**：所有服务读同一环境变量 `ARK_JWT_KEY`，禁止各自配置不同密钥
2. **生产密钥 ≥ 32 字节**：启动时强校验，拒绝 `dev-only` / `replace-with` 占位值
3. **鉴权委托平台**：产品服务不维护本地 Casbin 策略，通过 gRPC 委托鉴权
4. **认证本地验签**：JWT 无状态验证，不建设统一认证中心
