"""funasr-server：工程化的 FunASR 语音识别 HTTP 服务。

提供两个服务端口：
- 批量识别（SenseVoice，默认 :18000）
- 流式识别（paraformer-zh-streaming，默认 :18001）

用法：
    python -m funasr_server            # 启动批量 + 流式两个服务
    funasr-server --help               # 查看命令行选项
"""

__version__ = "0.1.0"
