package audio

import (
	"bytes"
	"testing"
)

func TestPCMToWAVRoundTrip(t *testing.T) {
	pcm := make([]byte, 640) // 20ms @ 16kHz 16bit mono
	for i := range pcm {
		pcm[i] = byte(i)
	}

	wav := PCMToWAV(pcm, 16000, 1, 16)
	if !IsWAV(wav) {
		t.Fatal("PCMToWAV 结果应为合法 WAV")
	}
	if len(wav) != 44+len(pcm) {
		t.Fatalf("WAV 长度 = %d, 期望 %d", len(wav), 44+len(pcm))
	}
	// 往返：WAV 头剥离后应还原 PCM
	if got := WAVToPCM(wav); !bytes.Equal(got, pcm) {
		t.Fatal("WAVToPCM 未能还原原始 PCM")
	}
}

func TestWAVToPCMNonWAV(t *testing.T) {
	raw := []byte{1, 2, 3, 4}
	if got := WAVToPCM(raw); !bytes.Equal(got, raw) {
		t.Fatal("非 WAV 输入应原样返回")
	}
}

func TestIsWAV(t *testing.T) {
	if IsWAV([]byte("RIFFxxxxWAVE")) {
		t.Fatal("长度不足 44 字节不应判定为 WAV")
	}
	wav := PCMToWAV(make([]byte, 64), 16000, 1, 16)
	if !IsWAV(wav) {
		t.Fatal("合法 WAV 应判定为 WAV")
	}
}

func TestPCMToWAVDefaults(t *testing.T) {
	// 零值参数应回退默认（16kHz/mono/16bit）
	wav := PCMToWAV(make([]byte, 32), 0, 0, 0)
	if len(wav) != 44+32 {
		t.Fatalf("长度 = %d", len(wav))
	}
	if wav[22] != 1 || wav[23] != 0 { // channels = 1 (little-endian uint16)
		t.Fatal("默认声道数应为 1")
	}
}
