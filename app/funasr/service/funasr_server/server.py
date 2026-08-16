"""HTTP 服务：批量识别与流式识别的 HTTP 接口。

接口约定：
    批量服务（默认 :18000）
        POST /recognize    body=音频字节，返回 {"text": "..."}
        GET  /health       健康检查

    流式服务（默认 :18001）
        POST /stream/recognize  body=JSON {"session_id","audio"(base64 PCM),"is_final"}
                                返回 {"text": 增量文本, "is_final": bool}
        POST /stream/end        body=JSON {"session_id"}，清理会话
        GET  /health            健康检查
"""
from __future__ import annotations

import base64
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Any

from . import config, recognize, stream


class _JSONHandler(BaseHTTPRequestHandler):
    """提供 JSON 响应与通用工具方法的基类。"""

    def _json(self, code: int, payload: dict[str, Any]) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_body(self) -> bytes:
        length = int(self.headers.get("Content-Length", "0"))
        return self.rfile.read(length) if length else b""

    def _read_json(self) -> dict[str, Any]:
        raw = self._read_body()
        if not raw:
            return {}
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return {}

    def log_message(self, *args: Any) -> None:
        """关闭默认访问日志，避免刷屏。"""
        pass


class BatchHandler(_JSONHandler):
    """批量识别：POST /recognize。"""

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/health":
            self._json(200, {"status": "ok"})
        else:
            self._json(404, {"error": "not found"})

    def do_POST(self) -> None:  # noqa: N802
        if self.path != "/recognize":
            self._json(404, {"error": "not found"})
            return
        audio = self._read_body()
        if not audio:
            self._json(400, {"error": "empty audio"})
            return
        try:
            text = recognize.recognize(audio)
            self._json(200, {"text": text})
        except Exception as e:  # noqa: BLE001
            self._json(500, {"error": str(e)})


class StreamHandler(_JSONHandler):
    """流式识别：POST /stream/recognize 与 /stream/end。"""

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/health":
            self._json(200, {"status": "ok"})
        else:
            self._json(404, {"error": "not found"})

    def do_POST(self) -> None:  # noqa: N802
        data = self._read_json()

        if self.path == "/stream/end":
            stream.end_session(data.get("session_id", ""))
            self._json(200, {"status": "ok"})
            return

        if self.path != "/stream/recognize":
            self._json(404, {"error": "not found"})
            return

        session_id = data.get("session_id", "")
        is_final = bool(data.get("is_final", False))
        if not session_id:
            self._json(400, {"error": "session_id required"})
            return

        audio_b64 = data.get("audio", "")
        try:
            audio = base64.b64decode(audio_b64) if audio_b64 else b""
        except ValueError:
            self._json(400, {"error": "invalid audio base64"})
            return

        try:
            text = stream.stream_recognize(session_id, audio, is_final)
            self._json(200, {"text": text, "is_final": is_final})
        except Exception as e:  # noqa: BLE001
            stream.end_session(session_id)
            self._json(500, {"error": str(e)})


def make_server(handler: type[BaseHTTPRequestHandler], port: int) -> HTTPServer:
    """创建并绑定一个 HTTP 服务实例。"""
    return HTTPServer(("0.0.0.0", port), handler)


def serve(handler: type[BaseHTTPRequestHandler], port: int) -> None:
    """启动并阻塞运行一个 HTTP 服务。"""
    server = make_server(handler, port)
    server.serve_forever()
