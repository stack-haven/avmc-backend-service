"""音频格式处理：PCM/WAV 转换与格式探测。

FunASR 的 AutoModel 通过 ffmpeg 加载音频，raw PCM（无容器头）无法被
ffmpeg 自动识别，需封装为 WAV；而流式模型直接接收 numpy int16 样本。
"""
from __future__ import annotations

import io
import wave

import numpy as np


def is_wav(data: bytes) -> bool:
    """判断字节是否为 RIFF/WAVE 格式（至少含 44 字节头）。"""
    return len(data) > 44 and data[:4] == b"RIFF" and data[8:12] == b"WAVE"


def pcm_to_wav(pcm: bytes, sample_rate: int = 16000, channels: int = 1, bits: int = 16) -> bytes:
    """将 raw PCM（16-bit little-endian）封装为 WAV。"""
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(channels)
        w.setsampwidth(bits // 8)
        w.setframerate(sample_rate)
        w.writeframes(pcm)
    return buf.getvalue()


def pcm_bytes_to_float32(pcm: bytes) -> np.ndarray:
    """将 raw PCM 字节转换为归一化 float32 数组（-1~1），供模型输入。"""
    samples = np.frombuffer(pcm, dtype=np.int16)
    return samples.astype(np.float32) / 32768.0


def ensure_wav(audio: bytes, sample_rate: int = 16000) -> bytes:
    """若非 WAV，则封装为 WAV；否则原样返回（用于批量识别的 ffmpeg 加载）。"""
    if is_wav(audio):
        return audio
    return pcm_to_wav(audio, sample_rate=sample_rate)
