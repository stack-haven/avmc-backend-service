"""流式识别：paraformer-zh-streaming 模型的封装，带会话级缓存。"""
from __future__ import annotations

import threading

import numpy as np

from . import audio, config
from .models import get_stream_model

# 会话缓存：session_id -> FunASR cache dict
_sessions: dict[str, dict] = {}
_lock = threading.Lock()


def stream_recognize(session_id: str, audio_bytes: bytes, is_final: bool) -> str:
    """处理一段流式音频，返回本段的增量文本。

    audio_bytes 为 raw PCM（16kHz 16bit 单声道）。is_final 标记是否为最后一段，
    命中后清理会话缓存。
    """
    if not session_id:
        raise ValueError("session_id required")

    # FunASR 流式模型要求每次喂入非空样本；final 时允许以静音样本收尾。
    if not audio_bytes:
        samples = np.zeros(1, dtype=np.float32)
    else:
        samples = audio.pcm_bytes_to_float32(audio_bytes)

    with _lock:
        cache = _sessions.get(session_id, {})

    model = get_stream_model()
    result = model.generate(
        input=samples,
        cache=cache,
        is_final=is_final,
        chunk_size=config.stream_chunk_size(),
    )

    with _lock:
        if is_final:
            _sessions.pop(session_id, None)
        else:
            _sessions[session_id] = cache

    if not result:
        return ""
    return result[0].get("text", "") or ""


def end_session(session_id: str) -> None:
    """主动清理某个会话的缓存（断连/超时时调用）。"""
    with _lock:
        _sessions.pop(session_id, None)
