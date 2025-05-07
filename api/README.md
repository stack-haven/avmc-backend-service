# API 文档目录 (api)

## 目录结构
```
api/
├── admin/                 # 管理后台API
│   ├── interface/         # 接口定义
│   │   ├── user.proto     # 用户接口
│   │   └── system.proto   # 系统接口
│   └── service/           # 服务定义
│       ├── user.proto     # 用户服务
│       └── system.proto   # 系统服务
├── common/                # 公共定义
│   ├── base.proto        # 基础定义
│   ├── error.proto       # 错误定义
│   └── types/            # 类型定义
└── docs/                  # API文档
    ├── admin/            # 管理后台文档
    │   ├── interface/    # 接口文档
    │   └── service/      # 服务文档
    └── swagger/          # Swagger文档
```

## API设计规范
1. Proto文件规范
   - 使用 Protocol Buffers v3
   - 文件命名：i_*.proto（接口）, s_*.proto（服务）
   - 必须包含完整注释

2. 接口版本控制
   - URL路径包含版本号：/v1/
   - 向后兼容原则
   - 版本升级规则

3. 错误处理
   - 统一错误码机制
   - 错误信息国际化
   - 详细错误描述

4. 文档生成
   - 使用 buf 工具链
   - 自动生成 OpenAPI 文档
   - 维护版本变更记录

## 开发流程
1. 接口设计
   - 遵循 RESTful 规范
   - 定义清晰的请求/响应结构
   - 考虑向后兼容性

2. 文档更新
   - 实时更新接口文档
   - 提供接口变更说明
   - 包含使用示例

3. 测试验证
   - 完整的接口测试用例
   - 性能测试指标
   - 兼容性测试