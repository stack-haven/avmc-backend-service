#!/usr/bin/env python3
"""Evie FunASR 流式识别 HTTP 服务。

封装 FunASR 流式模型（paraformer-zh-streaming），提供会话式增量识别。

启动：
    /usr/local/Caskroom/miniconda/base/envs/3.10/bin/python funasr_stream_server.py

接口：
    POST /stream/recognize  body=JSON {"session_id","audio"(base64 int16 PCM),"is_final"}
                            返回 {"text": 增量文本, "is_final": bool}
    POST /stream/end         body=JSON {"session_id"} 清理会话
    GET  /health

音频格式：16kHz / 16bit / 单声道 PCM（int16）
"""
import base64
import json

from http.server import BaseHTTPRequestHandler, HTTPServer

import numpy as np

from funasr import AutoModel

DEFAULT_PORT = int(__import__("os").environ.get("FUNASR_STREAM_PORT", "18001"))

MODEL = None
CACHES = {}  # session_id -> asr cache


def load_model():
    """懒加载流式模型（首次从 ModelScope 下载 paraformer-zh-streaming）。"""
    global MODEL
    if MODEL is None:
        MODEL = AutoModel(model="paraformer-zh-streaming", disable_update=True)
    return MODEL


class Handler(BaseHTTPRequestHandler):
    def _json(self, code, payload):
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
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)
        try:
            data = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            self._json(400, {"error": "invalid json"})
            return

        if self.path == "/stream/end":
            CACHES.pop(data.get("session_id", ""), None)
            self._json(200, {"status": "ok"})
            return

        if self.path != "/stream/recognize":
            self._json(404, {"error": "not found"})
            return

        session_id = data.get("session_id", "")
        audio_b64 = data.get("audio", "")
        is_final = bool(data.get("is_final", False))
        if not session_id or not audio_b64:
            self._json(400, {"error": "session_id and audio required"})
            return

        try:
            audio = np.frombuffer(base64.b64decode(audio_b64), dtype=np.int16)
        except Exception as e:  # noqa: BLE001
            self._json(400, {"error": f"invalid audio: {e}"})
            return

        if audio.size == 0:
            self._json(400, {"error": "empty audio"})
            return

        cache = CACHES.get(session_id, {})
        try:
            result = load_model().generate(
                input=audio,
                cache=cache,
                is_final=is_final,
                chunk_size=[0, 10, 5],  # 600ms lookahead
            )
        except Exception as e:  # noqa: BLE001
            CACHES.pop(session_id, None)
            self._json(500, {"error": str(e)})
            return

        CACHES[session_id] = cache
        text = ""
        if result:
            text = result[0].get("text", "") or ""
        if is_final:
            CACHES.pop(session_id, None)
        self._json(200, {"text": text, "is_final": is_final})

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    print("loading funasr streaming model (first run downloads model)...", flush=True)
    load_model()
    print(f"FunASR stream service listening on :{DEFAULT_PORT}", flush=True)
    HTTPServer(("0.0.0.0", DEFAULT_PORT), Handler).serve_forever()
