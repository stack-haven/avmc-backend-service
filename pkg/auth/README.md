# Go-Auth 认证与授权系统

## 简介

`go-auth` 是一个通用的身份认证与鉴权系统开发工具包，提供了可插拔的身份验证和身份鉴权功能。该包设计为高度解耦，支持多种认证和授权协议，可以轻松集成到基于 go-kratos 的微服务项目中。

## 特性

- 模块化设计，支持多种认证和授权提供者
- 可插拔架构，便于扩展和定制
- 完整的中间件支持，易于集成到 HTTP 和 gRPC 服务中
- 支持依赖注入（如 Google Wire 或 Uber Dig）
- 丰富的接口定义，满足各种认证和授权场景

## 结构

```
├── authn/                  # 身份验证模块
│   ├── authenticator.go    # 身份验证接口定义
│   ├── claims.go           # 认证声明相关定义
│   ├── errors.go           # 错误定义
│   ├── middleware/         # 中间件实现
│   ├── options.go          # 配置选项
│   └── provider/           # 认证提供者实现
│       ├── jwt/            # JWT 认证实现
│       ├── oidc/           # OIDC 认证实现
│       └── psk/            # PSK 认证实现
└── authz/                  # 身份鉴权模块
    ├── authorizer.go       # 身份鉴权接口定义
    ├── claims.go           # 鉴权声明相关定义
    ├── errors.go           # 错误定义
    ├── middleware/         # 中间件实现
    ├── options.go          # 配置选项
    └── provider/           # 鉴权提供者实现
        ├── casbin/         # Casbin 鉴权实现
        ├── opa/            # OPA 鉴权实现
        └── zanzibar/       # Zanzibar 鉴权实现
```

## 使用方法

请参考各模块下的示例和文档了解详细使用方法。

## 模块生成提示词
我正在开发一个身份认证模块，所在当前项目的pkg目录下，模块名称为go-auth。
角色：请作为一个golang编程语言的架构师或者是资深开发工程师完成我的需求，项目使用go-kratos微服务框架搭建。在身份认证模块中需要包含身份验证（简写：anthn）和身份鉴权（简写：anthz）子模块，因为这个是作为身份认证模块所以还需要增加中间件。

要求：模块设计需具备通用可扩展特性，同时让逻辑解耦、可插拔等能够接入兼容多种主流协议与后端提供者。
参考：设计思路以及编码过程可以参照go-micro和go-kratos的一些插件接口设计。
例如：go-kratos中的（registry/config）和go-micro中的（cache/store/registry/config），其中我更加欣赏go-mirco的设计方式。

使用场景说明：项目中实例化模块后端提供者，可以使用google的wire进行代码生成工具依赖注入自动连接组件，系统只需要在功能点中调用接口即可完成模块的完整调用。

服务接口基础方法：
身份验证（Authenticator）：身份初始化（Init）、身份验证、令牌验证、令牌发放、令牌续期（刷新）、令牌注销（Destroy）、提供者名称（String/Name）、等；
身份鉴权（Authorizer）：权限初始化（Init）、权限验证、添加权限策略、权限策略注销（Destroy）、提供者名称（String/Name）、等；

备注：接口方法需要你来优化和补充，我只是给你一些基础提示。

服务提供者：

身份验证提供者（AuthnProvider）：Jwt 、OIDC、PSK、等；

身份鉴权提供者（AuthzProvider）：Casbin、OPA、Zanzibar、等；

服务中间件：

身份验证中间件（AuthnMiddleware）：身份验证中间件，用于身份验证和令牌验证；

身份鉴权中间件（AuthzMiddleware）：身份鉴权中间件，用于权限验证；

以上需求中需要你提供设计思路和编码过程，需实现至少一个提供者的。

# Go Auth 身份认证模块开发规范

## 模块定位
- **路径**: `pkg/auth`
- **功能**: 提供统一的身份认证与鉴权能力
- **设计原则**: 
  - 接口驱动设计（Interface-based Design）
  - 依赖反转原则（Dependency Inversion）
  - 开闭原则（Open/Closed Principle）
- **核心组件**:
  ```mermaid
  graph TD
    A[Auth Module] --> B[Authn 身份验证]
    A --> C[Authz 身份鉴权]
    A --> D[Middleware]
    B --> E[JWT Provider]
    B --> F[OIDC Provider]
    C --> G[Casbin Provider]
    C --> H[OPA Provider]
    D --> I[Authn Middleware]
    D --> J[Authz Middleware]
    
```