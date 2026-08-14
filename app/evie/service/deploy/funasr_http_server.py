#!/usr/bin/env python3
"""Evie FunASR HTTP 服务：封装 funasr AutoModel，提供同步识别接口。

启动（需在已安装 funasr 的 python 环境，如 miniconda 3.10 env）：
    /usr/local/Caskroom/miniconda/base/envs/3.10/bin/python funasr_http_server.py

接口：
    POST /recognize    body=音频字节，返回 {"text": "..."}
    GET  /health       健康检查

依赖：pip install funasr modelscope torchaudio（模型首次启动从 ModelScope 下载）
"""
import json
import os
import re
import tempfile

from http.server import BaseHTTPRequestHandler, HTTPServer

from funasr import AutoModel

# SenseVoice 特殊标签（语言/情感/事件/标点标记），转写后应清理
_SENSEVOICE_TAG = re.compile(r"<\s*\|[^|]*\|\s*>")


def clean_text(text: str) -> str:
    """移除 SenseVoice 的特殊标签并去多余空格。"""
    text = _SENSEVOICE_TAG.sub("", text)
    return re.sub(r"\s+", " ", text).strip()

# 默认端口（Go 端 config_json 的 addr 与此对应）
DEFAULT_PORT = int(os.environ.get("FUNASR_HTTP_PORT", "18000"))

MODEL = None


def load_model():
    """懒加载模型，仅加载一次（模型约 2GB）。"""
    global MODEL
    if MODEL is None:
        MODEL = AutoModel(
            model="iic/SenseVoiceSmall",
            vad_model="fsmn-vad",
            punc_model="ct-punc",
            disable_update=True,
        )
    return MODEL


def _audio_suffix(content_type: str) -> str:
    ct = (content_type or "").lower()
    if "wav" in ct:
        return ".wav"
    if "pcm" in ct:
        return ".pcm"
    # 通用扩展名，让 ffmpeg 自动探测真实格式（mp3/webm/opus 等）
    return ".bin"


class Handler(BaseHTTPRequestHandler):
    def _json(self, code: int, payload: dict):
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/health":
            self._json(200, {"status": "ok"})
        else:
            self._json(404, {"error": "not found"})

    def do_POST(self):
        if self.path != "/recognize":
            self._json(404, {"error": "not found"})
            return
        length = int(self.headers.get("Content-Length", "0"))
        audio = self.rfile.read(length)
        if not audio:
            self._json(400, {"error": "empty audio"})
            return

        suffix = _audio_suffix(self.headers.get("Content-Type", ""))
        fd, tmp = tempfile.mkstemp(suffix=suffix)
        try:
            with os.fdopen(fd, "wb") as f:
                f.write(audio)
            result = load_model().generate(input=tmp, batch_size_s=300, language="zh")
            text = ""
            if result:
                text = clean_text(result[0].get("text", "") or "")
            self._json(200, {"text": text})
        except Exception as e:  # noqa: BLE001
            self._json(500, {"error": str(e)})
        finally:
            os.unlink(tmp)

    def log_message(self, *args):  # 关闭默认访问日志，避免刷屏
        pass


if __name__ == "__main__":
    print("loading funasr model (first run may take a while)...", flush=True)
    load_model()
    print(f"FunASR HTTP service listening on :{DEFAULT_PORT}", flush=True)
    HTTPServer(("0.0.0.0", DEFAULT_PORT), Handler).serve_forever()
