"""配置：集中管理服务配置，支持环境变量覆盖。

所有配置项均可通过环境变量覆盖（便于 Docker/K8s 部署），
默认值与本地开发约定一致。
"""
from __future__ import annotations

import os


def _env_int(name: str, default: int) -> int:
    raw = os.environ.get(name, "")
    try:
        return int(raw) if raw else default
    except ValueError:
        return default


def _env_bool(name: str, default: bool) -> bool:
    raw = os.environ.get(name, "")
    if not raw:
        return default
    return raw.strip().lower() in ("1", "true", "yes", "on")


# ---- 服务端口 ----
BATCH_PORT = _env_int("FUNASR_BATCH_PORT", 18000)
STREAM_PORT = _env_int("FUNASR_STREAM_PORT", 18001)

# ---- 模型 ----
# 批量识别模型（SenseVoice：中文最优，带标点/情感/语气）
BATCH_MODEL = os.environ.get("FUNASR_BATCH_MODEL", "iic/SenseVoiceSmall")
# 流式识别模型（paraformer-zh-streaming：实时逐帧）
STREAM_MODEL = os.environ.get("FUNASR_STREAM_MODEL", "paraformer-zh-streaming")

# ---- 音频 ----
SAMPLE_RATE = _env_int("FUNASR_SAMPLE_RATE", 16000)
# 流式识别的 chunk_size（[front, rear, chunk]，单位 60ms）
# [0, 10, 5] 表示 600ms lookahead，是 FunASR 流式模型的默认值。
STREAM_CHUNK_SIZE = os.environ.get("FUNASR_STREAM_CHUNK_SIZE", "0,10,5")

# ---- 运行时 ----
# 是否在启动时预加载模型（True 启动慢但首请求快；False 懒加载）
PRELOAD_MODEL = _env_bool("FUNASR_PRELOAD_MODEL", False)
# 禁用模型更新（生产环境建议 True，避免启动时联网检查）
DISABLE_UPDATE = _env_bool("FUNASR_DISABLE_UPDATE", True)


def stream_chunk_size() -> list[int]:
    """解析流式 chunk_size 配置为整数列表。"""
    parts = STREAM_CHUNK_SIZE.split(",")
    try:
        return [int(p.strip()) for p in parts]
    except ValueError:
        return [0, 10, 5]
