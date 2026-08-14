#!/usr/bin/env python3
"""Evie 麦克风流式识别测试客户端。

从 Mac 麦克风实时采集音频（16kHz 单声道 int16），分片发送到 FunASR 流式服务，
实时打印增量识别文本。按 Ctrl+C 结束。

用法：
    /usr/local/Caskroom/miniconda/base/envs/3.10/bin/python mic_stream_test.py [时长秒]
"""
import base64
import json
import queue
import sys
import time
import urllib.request

import numpy as np
import sounddevice as sd

SAMPLE_RATE = 16000
CHUNK_SECONDS = 0.6
CHUNK_SAMPLES = int(SAMPLE_RATE * CHUNK_SECONDS)
STREAM_URL = "http://localhost:18001/stream/recognize"


def post_chunk(session_id, audio: np.ndarray, is_final: bool) -> str:
    payload = {
        "session_id": session_id,
        "audio": base64.b64encode(audio.tobytes()).decode(),
        "is_final": is_final,
    }
    req = urllib.request.Request(
        STREAM_URL,
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.load(resp).get("text", "")


def main():
    duration = float(sys.argv[1]) if len(sys.argv) > 1 else 15.0
    q: queue.Queue = queue.Queue()

    def callback(indata, frames, time_info, status):  # noqa: A002
        q.put(indata.copy())

    print(f"开始录音 {duration}s，请对着麦克风说话...（Ctrl+C 结束）", flush=True)
    session_id = f"mic-{int(time.time())}"
    final_text = ""

    try:
        with sd.InputStream(
            samplerate=SAMPLE_RATE,
            channels=1,
            dtype="int16",
            blocksize=CHUNK_SAMPLES,
            callback=callback,
        ):
            start = time.time()
            while time.time() - start < duration:
                try:
                    chunk = q.get(timeout=1.0)
                except queue.Empty:
                    continue
                text = post_chunk(session_id, chunk.flatten(), is_final=False)
                if text and text != final_text:
                    print(text, flush=True)
                    final_text = text
            # 结束时发送 is_final=True
            final = post_chunk(session_id, np.zeros(0, dtype=np.int16), is_final=True)
            if final:
                print(f"[最终] {final}", flush=True)
    except KeyboardInterrupt:
        print("\n已停止。", flush=True)


if __name__ == "__main__":
    main()
