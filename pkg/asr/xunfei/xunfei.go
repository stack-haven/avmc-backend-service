// Package xunfei 实现讯飞语音转写（IAT）的 WebSocket 客户端，满足 pkg/asr.ASRProvider。
// 讯飞 IAT 是实时流式转写 API，音频格式为 raw PCM 16kHz 16bit 单声道。
package xunfei

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"backend-service/pkg/asr"
)

// Config 是讯飞 IAT 的配置（租户 config_json 解析结果）。
type Config struct {
	Host      string `json:"host"`       // iat-api.xfyun.cn
	AppID     string `json:"app_id"`     // 应用 APPID
	APISecret string `json:"api_secret"` // API Secret
	APIKey    string `json:"api_key"`    // API Key
	URI       string `json:"uri"`        // /v2/iat
}

// ParseConfig 解析 config_json。
func ParseConfig(configJSON string) (*Config, error) {
	if strings.TrimSpace(configJSON) == "" {
		return nil, fmt.Errorf("xunfei config_json is empty")
	}
	var cfg Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("parse xunfei config: %w", err)
	}
	if cfg.Host == "" || cfg.AppID == "" || cfg.APISecret == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("xunfei config requires host/app_id/api_secret/api_key")
	}
	if cfg.URI == "" {
		cfg.URI = "/v2/iat"
	}
	return &cfg, nil
}

// Provider 实现 asr.ASRProvider，对接讯飞 IAT WebSocket。
type Provider struct{ cfg Config }

var _ asr.ASRProvider = (*Provider)(nil)

// New 创建讯飞 Provider。
func New(cfg Config) (*Provider, error) {
	if cfg.Host == "" || cfg.AppID == "" || cfg.APISecret == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("xunfei config requires host/app_id/api_secret/api_key")
	}
	if cfg.URI == "" {
		cfg.URI = "/v2/iat"
	}
	return &Provider{cfg: cfg}, nil
}

func (p *Provider) Name() string { return "xunfei" }

func (p *Provider) Capabilities() asr.ProviderCapabilities {
	return asr.ProviderCapabilities{
		Streaming:       true,
		MaxDurationMs:   3600000, // 1小时
		SupportedFormat: []string{"pcm"},
		SampleRates:     []int{16000},
		HotwordSupport:  false,
		DeploymentMode:  "cloud_api",
	}
}

// buildAuthURL 构建带鉴权签名的 WebSocket URL。
func (p *Provider) buildAuthURL() string {
	date := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05") + " GMT"
	signatureOrigin := fmt.Sprintf("host: %s\ndate: %s\nGET %s HTTP/1.1", p.cfg.Host, date, p.cfg.URI)
	mac := hmac.New(sha256.New, []byte(p.cfg.APISecret))
	mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	authorizationOrigin := fmt.Sprintf(
		`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`,
		p.cfg.APIKey, signature,
	)
	authorization := base64.StdEncoding.EncodeToString([]byte(authorizationOrigin))

	return fmt.Sprintf("wss://%s%s?authorization=%s&date=%s&host=%s",
		p.cfg.Host, p.cfg.URI,
		url.QueryEscape(authorization), url.QueryEscape(date), p.cfg.Host)
}

// Recognize 同步识别：将音频按帧发送到讯飞 IAT，累加返回文本。
// audio 需为 raw PCM 16kHz 16bit 单声道。
func (p *Provider) Recognize(ctx context.Context, audio []byte, _ asr.RecognizeOptions) (*asr.ASRResult, error) {
	if len(audio) == 0 {
		return nil, fmt.Errorf("xunfei recognize: empty audio")
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, p.buildAuthURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("xunfei dial: %w", err)
	}
	defer conn.Close()

	// 读取结果协程：讯飞按帧异步返回
	resultCh := make(chan string, 32)
	go func() {
		var text strings.Builder
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				close(resultCh)
				return
			}
			segment := parseResultText(msg)
			if segment != "" {
				text.WriteString(segment)
				resultCh <- text.String()
			}
		}
	}()

	// 分帧发送音频（40ms 一帧）
	chunkSize := 1280
	for i := 0; i < len(audio); i += chunkSize {
		end := i + chunkSize
		if end > len(audio) {
			end = len(audio)
		}
		status := 1
		if i == 0 {
			status = 0
		}
		if end >= len(audio) {
			status = 2
		}
		frame := map[string]any{
			"common":   map[string]any{"app_id": p.cfg.AppID},
			"business": map[string]any{"language": "zh_cn", "domain": "iat", "accent": "mandarin", "vad_eos": 10000},
			"data": map[string]any{
				"status":   status,
				"format":   "audio/L16;rate=16000",
				"encoding": "raw",
				"audio":    base64.StdEncoding.EncodeToString(audio[i:end]),
			},
		}
		if err := conn.WriteJSON(frame); err != nil {
			return nil, fmt.Errorf("xunfei write frame: %w", err)
		}
	}

	// 发送结束帧（空音频，status=2）
	endFrame := map[string]any{
		"common":   map[string]any{"app_id": p.cfg.AppID},
		"business": map[string]any{"language": "zh_cn", "domain": "iat", "accent": "mandarin", "vad_eos": 10000},
		"data": map[string]any{
			"status":   2,
			"format":   "audio/L16;rate=16000",
			"encoding": "raw",
			"audio":    "",
		},
	}
	if err := conn.WriteJSON(endFrame); err != nil {
		return nil, fmt.Errorf("xunfei write end frame: %w", err)
	}

	// 等待最终结果（超时 60s）
	timeout := time.After(60 * time.Second)
	var finalText string
	for {
		select {
		case t, ok := <-resultCh:
			if !ok {
				if finalText == "" {
					return nil, fmt.Errorf("xunfei recognize: connection closed before result")
				}
				return &asr.ASRResult{Text: finalText, Confidence: 1.0, ProviderName: p.Name()}, nil
			}
			finalText = t
		case <-timeout:
			if finalText == "" {
				return nil, fmt.Errorf("xunfei recognize: timeout waiting result")
			}
			return &asr.ASRResult{Text: finalText, Confidence: 1.0, ProviderName: p.Name()}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// StreamRecognize 流式识别（讯飞 IAT 本身即流式，后续接入）。
func (p *Provider) StreamRecognize(context.Context, <-chan asr.PCMChunk, chan<- asr.ASRStreamResult, asr.RecognizeOptions) error {
	return fmt.Errorf("xunfei StreamRecognize not implemented")
}

func (p *Provider) Close() error { return nil }

// parseResultText 解析讯飞 IAT 返回的 JSON 中的文本片段。
func parseResultText(msg []byte) string {
	var resp struct {
		Data struct {
			Result struct {
				Ws []struct {
					Cw []struct {
						W string `json:"w"`
					} `json:"cw"`
				} `json:"ws"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return ""
	}
	var b strings.Builder
	for _, ws := range resp.Data.Result.Ws {
		for _, cw := range ws.Cw {
			b.WriteString(cw.W)
		}
	}
	return b.String()
}
