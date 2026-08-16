"""命令行入口：启动批量/流式识别服务。

用法示例：
    funasr-server                        # 同时启动批量(18000) + 流式(18001)
    funasr-server --batch-only           # 仅批量识别
    funasr-server --stream-only          # 仅流式识别
    funasr-server --batch-port 9000 --stream-port 9001
"""
from __future__ import annotations

import argparse
import threading

from . import config, server


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="funasr-server",
        description="FunASR 语音识别 HTTP 服务（批量 + 流式）",
    )
    parser.add_argument("--batch-port", type=int, default=config.BATCH_PORT,
                        help=f"批量识别端口（默认 {config.BATCH_PORT}）")
    parser.add_argument("--stream-port", type=int, default=config.STREAM_PORT,
                        help=f"流式识别端口（默认 {config.STREAM_PORT}）")
    parser.add_argument("--batch-only", action="store_true", help="仅启动批量识别服务")
    parser.add_argument("--stream-only", action="store_true", help="仅启动流式识别服务")
    parser.add_argument("--preload", action="store_true",
                        help="启动时预加载模型（启动慢但首请求快）")
    return parser


def main(argv: list[str] | None = None) -> None:
    args = _build_parser().parse_args(argv)

    if args.batch_only and args.stream_only:
        raise SystemExit("--batch-only 与 --stream-only 不能同时指定")

    if args.preload:
        from . import models
        models.preload()

    services: list[tuple[str, int]] = []
    if not args.stream_only:
        services.append(("batch", args.batch_port))
    if not args.batch_only:
        services.append(("stream", args.stream_port))

    for name, port in services:
        handler = server.BatchHandler if name == "batch" else server.StreamHandler
        t = threading.Thread(target=server.serve, args=(handler, port), daemon=True, name=f"funasr-{name}")
        t.start()
        print(f"funasr {name} service listening on :{port}", flush=True)

    # 保持主线程存活，等待信号退出
    try:
        threading.Event().wait()
    except KeyboardInterrupt:
        print("\nfunasr-server stopped")
