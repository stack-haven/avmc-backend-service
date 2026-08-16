"""模型加载：懒加载 + 进程级单例，避免重复加载大型模型。"""
from __future__ import annotations

import threading

from funasr import AutoModel

from . import config

_lock = threading.Lock()
_models: dict[str, "AutoModel"] = {}


def get_model(name: str, **kwargs) -> "AutoModel":
    """按名称获取模型实例，首次调用时加载并缓存（线程安全）。"""
    with _lock:
        model = _models.get(name)
        if model is None:
            model = AutoModel(model=name, disable_update=config.DISABLE_UPDATE, **kwargs)
            _models[name] = model
        return model


def get_batch_model() -> "AutoModel":
    """获取批量识别模型（SenseVoice，带 VAD + 标点）。"""
    return get_model(
        config.BATCH_MODEL,
        vad_model="fsmn-vad",
        punc_model="ct-punc",
    )


def get_stream_model() -> "AutoModel":
    """获取流式识别模型（paraformer-zh-streaming）。"""
    return get_model(config.STREAM_MODEL)


def preload() -> None:
    """预加载全部模型（供启动时调用，避免首请求等待）。"""
    get_batch_model()
    get_stream_model()
