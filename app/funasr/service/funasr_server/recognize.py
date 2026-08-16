"""批量识别：SenseVoice 模型的封装，返回清理后的文本。"""
from __future__ import annotations

import re

from . import audio, config
from .models import get_batch_model

# SenseVoice 特殊标签（语言/情感/事件/标点标记），转写后应清理
_SENSEVOICE_TAG = re.compile(r"<\s*\|[^|]*\|\s*>")


def clean_text(text: str) -> str:
    """移除 SenseVoice 特殊标签并归一化空白。"""
    text = _SENSEVOICE_TAG.sub("", text)
    return re.sub(r"\s+", " ", text).strip()


def recognize(audio_bytes: bytes, sample_rate: int = 0) -> str:
    """识别一段完整音频，返回文本。

    audio_bytes 为原始音频（PCM 或 WAV，PCM 自动封装为 WAV 供 ffmpeg 加载）。
    """
    if not audio_bytes:
        raise ValueError("empty audio")

    if sample_rate <= 0:
        sample_rate = config.SAMPLE_RATE
    wav = audio.ensure_wav(audio_bytes, sample_rate=sample_rate)

    model = get_batch_model()
    result = model.generate(input=wav, batch_size_s=300, language="zh")
    if not result:
        return ""
    return clean_text(result[0].get("text", "") or "")
