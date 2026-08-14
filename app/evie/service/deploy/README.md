# Evie ASR 依赖部署（FunASR Runtime Server）

本目录用于部署 Evie 语音识别的 ASR 引擎依赖。采用 docker compose，默认对接 FunASR Runtime Server。

## 前置条件

| 项 | 说明 |
|----|------|
| Docker | 20.10+ |
| Docker Compose | v2 |
| GPU（可选但推荐） | NVIDIA Driver + NVIDIA Container Toolkit |

> 无 GPU 也可用 CPU 跑通（Paraformer onnx 导出），但实时流式场景延迟会明显升高，建议生产用 GPU。

## 快速开始

```bash
cd app/evie/service/deploy

# GPU（推荐）
docker compose --profile gpu up -d

# 或 CPU
docker compose --profile cpu up -d
```

首次启动会从 ModelScope 下载模型到 `./models`，耗时取决于网络（国内快，海外需代理）。

查看日志与状态：

```bash
docker compose logs -f funasr-gpu   # 或 funasr-cpu
docker ps                            # 确认端口 10095/10096 已监听
```

## 端口约定

| 端口 | 协议 | 用途 |
|------|------|------|
| 10095 | gRPC | evie 后端 `pkg/asr/funasr` 客户端对接 |
| 10096 | HTTP/WebSocket | 调试、离线转写 REST 调用 |

> 端口与 `app/evie/service/configs/config.yaml` 中预留的 `funasr addr: localhost:10095` 一致。

## 模型选型

| 模型 | 场景 | 说明 |
|------|------|------|
| **SenseVoice-Small** | 中文企业语音（推荐） | 新一代，中文最优，支持情感/语气，速度快 |
| Paraformer-large | 中文经典 | SenseVoice 效果不满意时回退 |

默认镜像已内置默认模型配置；如需切换模型，可在容器内调整 FunASR Runtime 的模型加载配置（`config.yaml`），或重新打镜像指定模型。

## 验证部署成功

```bash
# 1. 确认端口监听
lsof -iTCP:10095 -sTCP:LISTEN

# 2. gRPC 健康检查（可用 grpcurl）
grpcurl -plaintext localhost:10095 list

# 3. HTTP 调试接口
curl http://localhost:10096/health
```

## 与 evie 后端集成

部署完成后，在管理后台「语音智能引擎 → 供应商管理」页面启用 `funasr` 并配置连接地址：

```json
{ "addr": "localhost:10095" }
```

或直接调用 evie 接口配置：

```bash
curl -X PUT http://localhost:8100/evie/v1/providers/config \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"provider_name":"funasr","is_active":true,"config_json":"{\"addr\":\"localhost:10095\"}","sample_rate":16000,"language":"zh"}'
```

> 下一步（B2）：evie 后端实现 `pkg/asr/funasr` gRPC 客户端，从租户 config 读取 `addr` 并路由到 FunASR，完成语音识别端到端链路。

## 停止与清理

```bash
docker compose --profile gpu down        # 停止（保留模型缓存）
docker compose --profile gpu down -v     # 停止并删除卷
```
