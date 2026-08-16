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
		Name:            p.Name(),
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

	// 读取结果协程：讯飞按帧异步返回（rpl 文本为累积完整句）
	resultCh := make(chan string, 32)
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				close(resultCh)
				return
			}
			if r := parseIatResult(msg); r.Text != "" {
				resultCh <- r.Text
			}
		}
	}()

	// 分帧发送音频。讯飞 IAT 为实时流式转写，帧间需保持节奏，但短音频（≤60s）
	// 可适当加快发送；vad_eos 调小以减少音频结束后的静默等待。
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
		frame := map[string]any{
			"common":   map[string]any{"app_id": p.cfg.AppID},
			"business": map[string]any{"language": "zh_cn", "domain": "iat", "accent": "mandarin", "vad_eos": 1000},
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
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// 发送结束帧（空音频，status=2）
	endFrame := map[string]any{
		"common":   map[string]any{"app_id": p.cfg.AppID},
		"business": map[string]any{"language": "zh_cn", "domain": "iat", "accent": "mandarin", "vad_eos": 1000},
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

// buildFrame 构建讯飞 IAT 的音频帧。
func (p *Provider) buildFrame(status int, audio []byte) map[string]any {
	return map[string]any{
		"common":   map[string]any{"app_id": p.cfg.AppID},
		"business": map[string]any{"language": "zh_cn", "domain": "iat", "accent": "mandarin", "vad_eos": 3000, "dwa": "wpgs"},
		"data": map[string]any{
			"status":   status,
			"format":   "audio/L16;rate=16000",
			"encoding": "raw",
			"audio":    base64.StdEncoding.EncodeToString(audio),
		},
	}
}

// StreamRecognize 流式识别：实时接收音频分片转发到讯飞 IAT，增量回传文本。
func (p *Provider) StreamRecognize(ctx context.Context, audioCh <-chan asr.PCMChunk, resultCh chan<- asr.ASRStreamResult, _ asr.RecognizeOptions) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, p.buildAuthURL(), nil)
	if err != nil {
		return fmt.Errorf("xunfei stream dial: %w", err)
	}
	defer conn.Close()

	// 读取结果协程：增量回传文本（rpl=替换累积完整句，apd=追加片段）
	resultDone := make(chan struct{})
	var finalText string
	go func() {
		defer close(resultDone)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			r := parseIatResult(msg)
			if r.Text == "" {
				continue
			}
			if r.Pgs == "apd" {
				finalText += r.Text
			} else {
				finalText = r.Text
			}
			resultCh <- asr.ASRStreamResult{Text: finalText, IsFinal: false}
		}
	}()

	// 发送音频分片（前端按实时节奏发送，后端直接转发）
	first := true
	for chunk := range audioCh {
		status := 1
		if first {
			status = 0
			first = false
		}
		if err := conn.WriteJSON(p.buildFrame(status, chunk.Data)); err != nil {
			return fmt.Errorf("xunfei stream write: %w", err)
		}
	}
	if first {
		if err := conn.WriteJSON(p.buildFrame(0, nil)); err != nil {
			return err
		}
	}
	if err := conn.WriteJSON(p.buildFrame(2, nil)); err != nil {
		return err
	}

	// 等待结果读取结束
	select {
	case <-resultDone:
	case <-time.After(15 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}
	if finalText != "" {
		resultCh <- asr.ASRStreamResult{Text: finalText, IsFinal: true}
	}
	return nil
}

func (p *Provider) Close() error { return nil }

// iatResult 讯飞 IAT 返回的一条结果。
type iatResult struct {
	Text string // 本句文本
	Pgs  string // rpl=替换（文本为累积完整句），apd=追加片段
	Ls   bool   // 是否最后一句
}

// parseIatResult 解析讯飞 IAT 返回的 JSON 结果。
func parseIatResult(msg []byte) iatResult {
	var resp struct {
		Data struct {
			Result struct {
				Pgs string `json:"pgs"`
				Ls  bool   `json:"ls"`
				Ws  []struct {
					Cw []struct {
						W string `json:"w"`
					} `json:"cw"`
				} `json:"ws"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return iatResult{}
	}
	var b strings.Builder
	for _, ws := range resp.Data.Result.Ws {
		for _, cw := range ws.Cw {
			b.WriteString(cw.W)
		}
	}
	return iatResult{Text: b.String(), Pgs: resp.Data.Result.Pgs, Ls: resp.Data.Result.Ls}
}

// parseResultText 兼容旧调用：仅返回文本（批量识别场景）。
func parseResultText(msg []byte) string {
	return parseIatResult(msg).Text
}
