# FunASR 语音识别服务镜像（CPU 版）
#
# GPU 版：将 base 镜像替换为 CUDA 镜像（如 nvidia/cuda:12.1-runtime），
# 并安装对应 CUDA 版本的 torch。示例见 deploy/docker-compose.yml 的 GPU profile。
FROM python:3.10-slim

# ffmpeg 用于音频解码（funasr 依赖 ffmpeg 加载多种音频格式）
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 先安装依赖（利用 Docker 层缓存，源码变化不触发重装 torch）
# 注意：torch/torchaudio 体积较大，生产建议使用官方 funasr 镜像或自建 base 镜像加速。
COPY pyproject.toml ./
# 使用 --extra-index-url 追加 pytorch CPU 源，保留 PyPI 主源，
# 否则 torch 的构建依赖（flit_core/typing_extensions/sympy）在 pytorch 源里找不到。
RUN pip install --no-cache-dir \
    --extra-index-url https://download.pytorch.org/whl/cpu \
    torch torchaudio funasr modelscope numpy

# 复制源码
COPY funasr_server ./funasr_server
COPY README.md ./

# 模型缓存卷（首次启动从 ModelScope 下载）
VOLUME ["/root/.cache/modelscope"]

EXPOSE 18000 18001

# 启动批量 + 流式两个服务
CMD ["python", "-m", "funasr_server"]
