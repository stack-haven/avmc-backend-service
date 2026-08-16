package asr_test

import (
	"context"
	"fmt"

	"backend-service/pkg/asr"
)

// Example 演示如何实现一个 ASR Provider 并通过注册中心调用。
func Example() {
	// 1. 实现 ASRProvider（这里是简化的假实现）
	reg := asr.NewProviderRegistry()
	reg.Register(&fakeProvider{})

	// 2. 按名称路由到具体引擎
	p, err := reg.Get("fake")
	if err != nil {
		fmt.Println("not found")
		return
	}

	// 3. 同步识别
	result, err := p.Recognize(context.Background(), []byte("pcm"), asr.RecognizeOptions{
		SampleRate: 16000,
		Language:   "zh",
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(result.ProviderName, result.Text)
	// Output: fake hello
}

// fakeProvider 是最小 Provider 实现示例。
type fakeProvider struct{}

func (fakeProvider) Name() string { return "fake" }

func (fakeProvider) Recognize(_ context.Context, _ []byte, _ asr.RecognizeOptions) (*asr.ASRResult, error) {
	return &asr.ASRResult{Text: "hello", ProviderName: "fake"}, nil
}

func (fakeProvider) StreamRecognize(_ context.Context, _ <-chan asr.PCMChunk, _ chan<- asr.ASRStreamResult, _ asr.RecognizeOptions) error {
	return nil
}

func (fakeProvider) Capabilities() asr.ProviderCapabilities {
	return asr.ProviderCapabilities{
		Name:            "fake",
		Streaming:       false,
		SupportedFormat: []string{"pcm"},
		SampleRates:     []int{16000},
		DeploymentMode:  "self_hosted",
	}
}
