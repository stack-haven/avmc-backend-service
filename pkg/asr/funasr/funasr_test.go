package funasr

import (
	"testing"

	"backend-service/pkg/asr"
)

func TestParseConfig(t *testing.T) {
	cfg, err := ParseConfig(`{"addr":"http://localhost:18000"}`)
	if err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}
	if cfg.Addr != "http://localhost:18000" {
		t.Errorf("expected addr http://localhost:18000, got %q", cfg.Addr)
	}
}

func TestParseConfigEmpty(t *testing.T) {
	if _, err := ParseConfig(""); err == nil {
		t.Fatal("expected error for empty config_json")
	}
	if _, err := ParseConfig(`{"sample_rate":16000}`); err == nil {
		t.Fatal("expected error for missing addr")
	}
}

func TestProviderNameAndCapabilities(t *testing.T) {
	p, err := New(Config{Addr: "http://localhost:18000"})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()

	if p.Name() != "funasr" {
		t.Errorf("expected name funasr, got %q", p.Name())
	}
	caps := p.Capabilities()
	if caps.Streaming || caps.HotwordSupport {
		t.Errorf("HTTP 服务暂不支持流式/热词，got %+v", caps)
	}
	if caps.DeploymentMode != "self_hosted" {
		t.Errorf("expected self_hosted, got %q", caps.DeploymentMode)
	}
}

func TestNewDefaults(t *testing.T) {
	p, err := New(Config{Addr: "http://localhost:18000"})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer p.Close()
	if p.cfg.SampleRate != 16000 {
		t.Errorf("expected default sample_rate 16000, got %d", p.cfg.SampleRate)
	}
	if p.cfg.Language != "zh" {
		t.Errorf("expected default language zh, got %q", p.cfg.Language)
	}
}

func TestRecognizeEmptyAudio(t *testing.T) {
	p, _ := New(Config{Addr: "http://localhost:18000"})
	defer p.Close()
	_, err := p.Recognize(t.Context(), nil, asr.RecognizeOptions{})
	if err == nil {
		t.Fatal("expected error for empty audio")
	}
}
