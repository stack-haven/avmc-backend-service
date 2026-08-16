# Evie 服务部署

本目录部署 Evie 语音识别服务及其依赖。

## FunASR 引擎依赖

Evie 语音识别依赖的 FunASR 服务已**独立为单独服务**（可单独开源），
源码与部署脚本均位于：

```
backend-service/app/funasr/service/
├── funasr_server/      # Python 包
├── Dockerfile
├── README.md           # 安装、配置、部署说明
└── deploy/
    ├── docker-compose.yml
    └── k8s/funasr.yaml
```

FunASR 的安装、配置与部署方式详见 `app/funasr/service/README.md`。

部署完成后，在 evie 管理后台「语音智能引擎 → 供应商管理」启用 `funasr`
并配置连接地址（`addr`/`stream_addr`），整段批量识别会自动路由到 funasr。

## 遗留内容

- `docker/`：官方 FunASR Runtime Server gRPC 方案（已废弃，保留备查）。
- `models/`：模型缓存目录（不入库）。
