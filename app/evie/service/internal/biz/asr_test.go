package biz

import (
	"bytes"
	"testing"

	pb "backend-service/api/evie/service/v1"
	asraudio "backend-service/pkg/asr/audio"
)

func TestNewProviderFunasr(t *testing.T) {
	p, err := newProvider("funasr", `{"addr":"localhost:10095"}`)
	if err != nil {
		t.Fatalf("newProvider error: %v", err)
	}
	if p.Name() != "funasr" {
		t.Errorf("expected funasr, got %q", p.Name())
	}
}

func TestNewProviderUnsupported(t *testing.T) {
	if _, err := newProvider("whisper", "{}"); err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestNewProviderBadConfig(t *testing.T) {
	if _, err := newProvider("funasr", ""); err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestNormalizeAudio(t *testing.T) {
	// raw PCM → 封装为 WAV（可播放）
	req := &pb.RecognizeRequest{
		AudioData: make([]byte, 64),
		Format:    &pb.AudioFormat{Encoding: pb.AudioEncoding_AUDIO_ENCODING_PCM, SampleRate: 16000},
	}
	audio, ext, contentType := normalizeAudio(req)
	if ext != "wav" || contentType != "audio/wav" {
		t.Errorf("expected wav/audio/wav, got %q/%q", ext, contentType)
	}
	if !asraudio.IsWAV(audio) || len(audio) != 44+64 {
		t.Errorf("PCM 应封装为 WAV, len=%d", len(audio))
	}

	// 已是 WAV → 原样返回
	wav := asraudio.PCMToWAV(make([]byte, 32), 16000, 1, 16)
	req2 := &pb.RecognizeRequest{
		AudioData: wav,
		Format:    &pb.AudioFormat{Encoding: pb.AudioEncoding_AUDIO_ENCODING_WAV},
	}
	audio2, ext2, _ := normalizeAudio(req2)
	if ext2 != "wav" || !bytes.Equal(audio2, wav) {
		t.Error("WAV 应原样返回")
	}
}
