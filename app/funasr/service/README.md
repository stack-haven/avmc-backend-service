# funasr-server

工程化的 [FunASR](https://github.com/modelscope/FunASR) 语音识别 HTTP 服务，提供两个服务端口：

| 端口 | 协议 | 模型 | 用途 |
|------|------|------|------|
| 18000 | HTTP | SenseVoice-Small | 批量识别（整段音频，带标点/情感） |
| 18001 | HTTP | paraformer-zh-streaming | 流式识别（实时逐帧） |

## 特性

- **工程化结构**：模块化 Python 包（配置/模型/音频/识别/流式/服务分层），非单文件脚本。
- **可安装**：`pip install .` 后通过 `funasr-server` 命令启动。
- **环境变量配置**：端口、模型、采样率等均可用环境变量覆盖，适配容器编排。
- **容器化**：提供 Dockerfile + docker-compose + Kubernetes 清单。
- **懒加载**：模型首次请求时加载；`--preload` 可启动即加载。

## 快速开始

### 本地运行

```bash
# 安装（需 Python 3.9+，torch 依赖较重）
pip install .

# 启动（批量 + 流式）
funasr-server

# 或仅启动批量识别
funasr-server --batch-only --preload
```

首次启动会从 ModelScope 下载模型（SenseVoice ~1GB，paraformer-streaming 更小）。

### Docker

```bash
docker build -t funasr-server .
docker run -p 18000:18000 -p 18001:18001 funasr-server
```

## 部署

部署脚本位于 `deploy/` 目录。

### 离线模型准备（推荐）

默认启动会从 ModelScope 下载模型（约 2.8GB，国内快、海外慢）。为避免每次启动重复拉取，
可先将模型缓存复制到 `models/` 目录，Docker 会将其挂载为容器内的模型缓存：

```bash
# 在本机先运行一次 funasr-server 下载模型（或已有缓存），然后：
./scripts/prepare_models.sh
```

`models/` 目录需包含以下完整模型结构（约 2.8GB，含权重 + 配置 + 词表，**非单个 model.pt**）：

```
models/iic/
├── SenseVoiceSmall/            # 批量识别（权重 model.pt + config.yaml + tokens.json 等）
├── speech_paraformer-large_asr_nat-zh-cn-16k-common-vocab8404-online/  # 流式识别
├── speech_fsmn_vad_zh-cn-16k-common-pytorch/   # VAD
└── punc_ct-transformer_cn-en-common-vocab471067-large/  # 标点
```

### Docker Compose

```bash
cd deploy
docker compose up -d --build
```

`docker-compose.yml` 将 `../models` 挂载到容器 `/root/.cache/modelscope/hub/models`，
模型已存在则不再联网下载。

### Kubernetes

```bash
kubectl apply -f deploy/k8s/funasr.yaml
```

清单包含 PVC（模型缓存）+ Deployment + Service，镜像需先构建并推送：

```bash
docker build -t <registry>/funasr-server:latest .
docker push <registry>/funasr-server:latest
# 修改 k8s/funasr.yaml 中的 image 为实际镜像地址后 apply
```

> K8s 环境建议将模型预置到镜像或将模型存放于共享存储（PVC/NFS），避免节点漂移后重新下载。

## 接口

### 批量识别

```bash
curl -X POST http://localhost:18000/recognize \
  -H 'Content-Type: audio/wav' \
  --data-binary @audio.wav
# {"text": "识别结果文本"}
```

请求体为音频字节（WAV/MP3/OPUS 等 ffmpeg 可识别的格式，或 raw PCM 会自动封装为 WAV）。

### 流式识别

```bash
curl -X POST http://localhost:18001/stream/recognize \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"s1","audio":"<base64 PCM>","is_final":false}'
# {"text": "增量文本", "is_final": false}

curl -X POST http://localhost:18001/stream/end \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"s1"}'
```

流式音频为 16kHz/16bit/单声道 PCM（int16），base64 编码。

### 健康检查

```bash
curl http://localhost:18000/health
# {"status": "ok"}
```

## 配置（环境变量）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `FUNASR_BATCH_PORT` | 18000 | 批量识别端口 |
| `FUNASR_STREAM_PORT` | 18001 | 流式识别端口 |
| `FUNASR_BATCH_MODEL` | iic/SenseVoiceSmall | 批量识别模型 |
| `FUNASR_STREAM_MODEL` | paraformer-zh-streaming | 流式识别模型 |
| `FUNASR_SAMPLE_RATE` | 16000 | 采样率 |
| `FUNASR_STREAM_CHUNK_SIZE` | 0,10,5 | 流式 chunk_size（600ms lookahead） |
| `FUNASR_PRELOAD_MODEL` | false | 启动时预加载模型 |
| `FUNASR_DISABLE_UPDATE` | true | 禁用模型联网更新 |

## 项目结构

```
funasr_server/
├── __init__.py     # 版本号
├── __main__.py     # python -m funasr_server 入口
├── cli.py          # 命令行入口
├── config.py       # 配置（环境变量）
├── models.py       # 模型加载（懒加载 + 单例）
├── audio.py        # PCM/WAV 音频处理
├── recognize.py    # 批量识别（SenseVoice）
├── stream.py       # 流式识别（paraformer-streaming）
└── server.py       # HTTP 服务（路由分发）

deploy/
├── docker-compose.yml   # Docker Compose 编排
└── k8s/funasr.yaml      # Kubernetes 部署清单
```

## 依赖

- Python 3.9+
- funasr >= 1.2.0
- modelscope >= 1.18.0
- torch >= 2.0 / torchaudio >= 2.0

GPU 环境建议使用官方 CUDA 镜像加速推理。

## License

Apache-2.0
