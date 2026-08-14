package biz

import (
	"testing"

	pb "backend-service/api/evie/service/v1"
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

func TestToAsrHotwords(t *testing.T) {
	hotwords := []*pb.Hotword{
		{Word: "金种子", Target: "金种籽", Weight: 5},
		{Word: "田华", Weight: 3},
	}
	got := toAsrHotwords(hotwords)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Word != "金种子" || got[0].Target != "金种籽" || got[0].Weight != 5 {
		t.Errorf("unexpected first hotword: %+v", got[0])
	}
	if got[1].Target != "" {
		t.Errorf("expected empty target, got %q", got[1].Target)
	}
}

func TestFormatEncodingString(t *testing.T) {
	if got := formatEncodingString(pb.AudioEncoding_AUDIO_ENCODING_WAV); got != "wav" {
		t.Errorf("expected wav, got %q", got)
	}
	if got := formatEncodingString(pb.AudioEncoding_AUDIO_ENCODING_UNSPECIFIED); got != "pcm" {
		t.Errorf("expected pcm default, got %q", got)
	}
}
