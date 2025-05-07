# 中间件目录 (internal/middleware)

## 目录说明
该目录用于存放项目内部使用的中间件组件。遵循 Go-Kratos 微服务项目规范，按功能模块划分子目录：

```
middleware/
├── auth/          # 认证中间件
│   ├── jwt.go     # JWT认证
│   └── oauth.go   # OAuth认证
├── logging/       # 日志中间件
│   └── logging.go # 访问日志记录
├── metrics/       # 监控中间件
│   └── metrics.go # 性能指标收集
├── recovery/      # 恢复中间件
│   └── recovery.go# 服务恢复处理
├── tracing/       # 追踪中间件
│   └── tracing.go # 链路追踪
└── validate/      # 验证中间件
    └── validate.go# 参数验证
```

## 使用说明
1. 所有中间件必须实现 Kratos 的中间件接口
2. 中间件配置应通过依赖注入方式提供
3. 中间件注册顺序应遵循：
   - Recovery (最外层)
   - Tracing
   - Logging
   - Metrics
   - Auth
   - Validate (最内层)

## 开发规范
1. 每个中间件必须有完整的单元测试
2. 必须提供详细的使用文档和示例
3. 错误处理必须使用 Kratos 的错误码机制
4. 配置项必须提供默认值