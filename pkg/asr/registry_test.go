package asr

import (
	"context"
	"sync"
	"testing"
)

// mockProvider 是测试用的最小 Provider 实现。
type mockProvider struct {
	name string
	cap  ProviderCapabilities
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Recognize(context.Context, []byte, RecognizeOptions) (*ASRResult, error) {
	return &ASRResult{Text: "ok", ProviderName: m.name}, nil
}

func (m *mockProvider) StreamRecognize(context.Context, <-chan PCMChunk, chan<- ASRStreamResult, RecognizeOptions) error {
	return nil
}

func (m *mockProvider) Capabilities() ProviderCapabilities {
	m.cap.Name = m.name
	return m.cap
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewProviderRegistry()
	r.Register(&mockProvider{name: "funasr", cap: ProviderCapabilities{Streaming: true}})

	p, err := r.Get("funasr")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.Name() != "funasr" {
		t.Fatalf("Name = %q", p.Name())
	}

	if _, err := r.Get("unknown"); err != ErrProviderNotFound {
		t.Fatalf("未知供应商应返回 ErrProviderNotFound, got %v", err)
	}
}

func TestRegistryListIncludesName(t *testing.T) {
	r := NewProviderRegistry()
	r.Register(&mockProvider{name: "funasr"})
	r.Register(&mockProvider{name: "xunfei"})

	caps := r.List()
	if len(caps) != 2 {
		t.Fatalf("List 长度 = %d", len(caps))
	}
	seen := map[string]bool{}
	for _, c := range caps {
		seen[c.Name] = true
	}
	if !seen["funasr"] || !seen["xunfei"] {
		t.Fatalf("能力列表缺少 Name: %v", seen)
	}
}

func TestRegistryConcurrent(t *testing.T) {
	r := NewProviderRegistry()
	r.Register(&mockProvider{name: "funasr"})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Get("funasr")
			_ = r.Names()
		}()
	}
	wg.Wait()
}
