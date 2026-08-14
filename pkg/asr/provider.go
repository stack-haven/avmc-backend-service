// Package asr 定义 ASR 供应商抽象，供 Evie 等需要语音识别的服务复用。
// 供应商实现（FunASR/Whisper/讯飞/阿里云）放在本包的子目录，业务层通过
// ProviderRegistry 按租户配置路由到具体实现，实现可替换、可组合。
package asr

import (
	"context"
	"fmt"
)

// ASRProvider ASR 引擎供应商抽象。
// 所有 ASR 实现（私有部署/云 API）必须实现此接口。
type ASRProvider interface {
	// Name 供应商标识，全局唯一。
	Name() string

	// Recognize 同步识别（短音频 ≤60s）。
	Recognize(ctx context.Context, audio []byte, opts RecognizeOptions) (*ASRResult, error)

	// StreamRecognize 流式识别，实时返回中间结果。
	StreamRecognize(ctx context.Context, audioCh <-chan PCMChunk, resultCh chan<- ASRStreamResult, opts RecognizeOptions) error

	// Capabilities 返回该供应商的能力集。
	Capabilities() ProviderCapabilities
}

// ProviderCapabilities 供应商能力声明。
type ProviderCapabilities struct {
	Streaming       bool     // 支持流式识别
	MaxDurationMs   int64    // 单次最长音频时长（0=无限制）
	SupportedFormat []string // 支持的音频格式: pcm/wav/mp3/opus
	SampleRates     []int    // 支持的采样率
	HotwordSupport  bool     // 是否支持热词增强
	DeploymentMode  string   // self_hosted / cloud_api
}

// RecognizeOptions 识别参数（各 Provider 各自提取需要字段）。
type RecognizeOptions struct {
	TenantID   string
	UserID     string
	Hotwords   []Hotword
	SampleRate int
	Language   string // zh/en/auto
}

// Hotword 热词（传给 ASR 引擎）。
type Hotword struct {
	Word   string
	Target string
	Weight float64
}

// ASRResult 同步识别结果。
type ASRResult struct {
	RequestID    string
	Text         string
	Segments     []Segment
	Confidence   float64
	DurationMs   int64
	ProviderName string
}

// Segment 识别分段。
type Segment struct {
	StartMs    int64
	EndMs      int64
	Text       string
	Confidence float64
}

// ASRStreamResult 流式识别中间结果。
type ASRStreamResult struct {
	RequestID   string
	Text        string
	Confidence  float64
	IsFinal     bool
	TimestampMs int64
}

// PCMChunk PCM 音频分片。
type PCMChunk struct {
	Data        []byte
	Timestamp   int64
	VoiceActive bool
}

// ErrProviderNotConfigured 表示租户尚未配置可用的 ASR Provider。
func ErrProviderNotConfigured(name string) error {
	return fmt.Errorf("asr provider %q is not configured", name)
}
