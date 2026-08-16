package xunfei

import (
	"context"
	"os"
	"testing"
	"time"

	"backend-service/pkg/asr"
)

// TestRecognizeReal 真实调用讯飞 IAT（需外网 + 有效配置，手动运行）。
func TestRecognizeReal(t *testing.T) {
	if os.Getenv("XUNFEI_TEST") != "1" {
		t.Skip("设置 XUNFEI_TEST=1 才运行真实讯飞测试")
	}
	cfg := Config{
		Host:      "iat-api.xfyun.cn",
		AppID:     "2d8c3dd7",
		APISecret: "MWEyMzg2ODhiMThhY2E2MWJmZWQ4YjQ4",
		APIKey:    "641cdc25afd21421685c6ec2ce24149c",
		URI:       "/v2/iat",
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	audio, err := os.ReadFile("/tmp/xunfei_10s.pcm")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := p.Recognize(ctx, audio, asr.RecognizeOptions{})
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	t.Logf("识别结果: %s", result.Text)
}
