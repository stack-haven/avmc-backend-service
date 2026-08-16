// Package funasr 实现 FunASR 的 HTTP 客户端，满足 pkg/asr.ASRProvider 接口。
//
// 后端是一个 Python HTTP 服务（见 app/evie/service/deploy/funasr_http_server.py），
// 封装 funasr AutoModel + SenseVoice 模型，提供 POST /recognize 同步识别。
// 这样无需 FunASR Runtime Server 的 gRPC proto，即可对接自建 FunASR。
package funasr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"backend-service/pkg/asr"
	asraudio "backend-service/pkg/asr/audio"
)

// Config 是租户 asr_provider_config.config_json 的解析结果。
// 示例：{"addr":"http://localhost:18000","stream_addr":"http://localhost:18001"}
type Config struct {
	// Addr 是 Python FunASR 同步识别服务地址（含 http:// 前缀）。
	Addr string `json:"addr"`
	// StreamAddr 是 Python FunASR 流式识别服务地址（可选，空则不支持流式）。
	StreamAddr string `json:"stream_addr,omitempty"`
	// SampleRate 可选，默认 16000。
	SampleRate int `json:"sample_rate,omitempty"`
	// Language 可选，默认 zh。
	Language string `json:"language,omitempty"`
}

// ParseConfig 解析 config_json 字符串为 Config。
func ParseConfig(configJSON string) (*Config, error) {
	if strings.TrimSpace(configJSON) == "" {
		return nil, fmt.Errorf("funasr config_json is empty")
	}
	var cfg Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("parse funasr config: %w", err)
	}
	if cfg.Addr == "" {
		return nil, fmt.Errorf("funasr addr is required in config_json")
	}
	return &cfg, nil
}

// Provider 实现 asr.ASRProvider，对接 Python FunASR HTTP 服务。
type Provider struct {
	cfg    Config
	client *http.Client
}

var _ asr.ASRProvider = (*Provider)(nil)

// New 创建 FunASR Provider。
func New(cfg Config) (*Provider, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("funasr addr is required")
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 16000
	}
	if cfg.Language == "" {
		cfg.Language = "zh"
	}
	return &Provider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 120 * time.Second, // ASR 推理可能较慢
		},
	}, nil
}

// Name 返回供应商唯一标识。
func (p *Provider) Name() string { return "funasr" }

// Capabilities 返回 FunASR 的能力声明。
func (p *Provider) Capabilities() asr.ProviderCapabilities {
	return asr.ProviderCapabilities{
		Name:            p.Name(),
		Streaming:       p.cfg.StreamAddr != "",
		MaxDurationMs:   0,
		SupportedFormat: []string{"pcm", "wav", "mp3", "opus"},
		SampleRates:     []int{8000, 16000},
		HotwordSupport:  false, // 当前 HTTP 服务未接热词，后续可扩展
		DeploymentMode:  "self_hosted",
	}
}

// Recognize 同步识别：POST 音频到 Python 服务，解析返回文本。
func (p *Provider) Recognize(ctx context.Context, audio []byte, opts asr.RecognizeOptions) (*asr.ASRResult, error) {
	if len(audio) == 0 {
		return nil, fmt.Errorf("funasr recognize: empty audio")
	}
	sampleRate := opts.SampleRate
	if sampleRate == 0 {
		sampleRate = p.cfg.SampleRate
	}
	// 后端 funasr 服务依赖 ffmpeg 探测音频格式，raw PCM 需封装为 WAV 才能识别。
	if !asraudio.IsWAV(audio) {
		audio = asraudio.PCMToWAV(audio, sampleRate, 1, 16)
	}
	url := strings.TrimRight(p.cfg.Addr, "/") + "/recognize"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(audio))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "audio/wav")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("funasr recognize: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("funasr read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("funasr recognize http %d: %s", resp.StatusCode, string(body))
	}
	var parsed recognizeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("funasr parse response: %w", err)
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("funasr recognize: %s", parsed.Error)
	}
	return &asr.ASRResult{
		Text:         parsed.Text,
		Confidence:   1.0,
		ProviderName: p.Name(),
	}, nil
}

// StreamRecognize 流式识别：逐分片发送到 Python 流式服务，回传增量文本。
func (p *Provider) StreamRecognize(ctx context.Context, audioCh <-chan asr.PCMChunk, resultCh chan<- asr.ASRStreamResult, _ asr.RecognizeOptions) error {
	if p.cfg.StreamAddr == "" {
		return fmt.Errorf("funasr stream_addr is not configured")
	}
	sessionID := fmt.Sprintf("stream-%d", time.Now().UnixNano())
	for {
		select {
		case chunk, ok := <-audioCh:
			if !ok {
				// 流结束，发送 is_final 获取最终结果
				text, err := p.streamChunk(ctx, sessionID, nil, true)
				if err != nil {
					return err
				}
				resultCh <- asr.ASRStreamResult{Text: text, IsFinal: true}
				return nil
			}
			text, err := p.streamChunk(ctx, sessionID, chunk.Data, false)
			if err != nil {
				return err
			}
			resultCh <- asr.ASRStreamResult{Text: text, IsFinal: false, TimestampMs: chunk.Timestamp}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// streamChunk 发送一个音频分片到流式服务并返回增量文本。
func (p *Provider) streamChunk(ctx context.Context, sessionID string, audio []byte, isFinal bool) (string, error) {
	payload := map[string]any{
		"session_id": sessionID,
		"audio":      base64.StdEncoding.EncodeToString(audio),
		"is_final":   isFinal,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	url := strings.TrimRight(p.cfg.StreamAddr, "/") + "/stream/recognize"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("funasr stream: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("funasr stream http %d: %s", resp.StatusCode, string(respBody))
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	return parsed.Text, nil
}

// Close 关闭（HTTP 客户端无需释放资源）。
func (p *Provider) Close() error { return nil }

// recognizeResponse 是 Python 服务的 /recognize 响应体。
type recognizeResponse struct {
	Text  string `json:"text"`
	Error string `json:"error"`
}
