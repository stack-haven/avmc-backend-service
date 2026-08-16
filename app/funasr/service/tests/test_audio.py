"""音频处理函数测试（纯函数，无模型依赖）。"""
from funasr_server import audio


def test_pcm_to_wav_and_is_wav():
    pcm = bytes(range(64))
    wav = audio.pcm_to_wav(pcm, sample_rate=16000)
    assert audio.is_wav(wav)
    assert len(wav) == 44 + len(pcm)


def test_ensure_wav_wraps_pcm():
    pcm = bytes(128)
    result = audio.ensure_wav(pcm, sample_rate=16000)
    assert audio.is_wav(result)


def test_ensure_wav_passthrough():
    wav = audio.pcm_to_wav(bytes(32), sample_rate=16000)
    assert audio.ensure_wav(wav) == wav


def test_pcm_bytes_to_float32():
    # int16 的 32767 应归一化为接近 1.0
    import struct
    pcm = struct.pack("<h", 32767) + struct.pack("<h", -32768)
    arr = audio.pcm_bytes_to_float32(pcm)
    assert arr.shape == (2,)
    assert abs(arr[0] - 32767 / 32768.0) < 1e-6
    assert arr[1] == -1.0
