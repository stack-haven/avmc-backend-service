// Package asr 定义语音识别（ASR）供应商抽象层。
//
// 本包以「供应商可替换、可组合」为核心，提供三部分能力：
//
//  1. ASRProvider 接口与数据类型：定义所有 ASR 引擎（私有部署/云 API）
//     必须实现的最小契约，以及识别参数、结果、流式结果、能力声明等公共类型。
//  2. ProviderRegistry 注册中心：在服务启动时注册各供应商实现，运行时按名称
//     路由，避免业务层硬编码具体引擎。
//  3. 音频格式工具（子包 audio）：提供 PCM/WAV 互转等 ASR 常用格式处理。
//
// 供应商实现（FunASR/Whisper/讯飞/阿里云）位于本包的子目录，各自独立，
// 仅依赖本包定义的接口与类型，可单独发布或替换。
package asr

import (
	"context"
	"errors"
)

// ErrProviderNotFound 表示注册中心未找到指定名称的供应商。
var ErrProviderNotFound = errors.New("asr provider not found")

// ASRProvider 是 ASR 引擎供应商的最小契约。
// 所有实现（私有部署/云 API）必须满足该接口，从而能被业务层统一调用。
type ASRProvider interface {
	// Name 返回供应商标识，全局唯一（如 "funasr"、"xunfei"）。
	Name() string

	// Recognize 同步识别一段完整音频（短音频，通常 ≤60s）。
	// audio 为原始音频字节；具体格式（PCM/WAV 等）由各实现解释，
	// 也可参考 opts.SampleRate 判断采样率。
	Recognize(ctx context.Context, audio []byte, opts RecognizeOptions) (*ASRResult, error)

	// StreamRecognize 流式识别：从 audioCh 持续读取音频分片，将增量识别结果
	// 写入 resultCh，直到 audioCh 关闭。实现需保证 resultCh 最终被关闭或
	// 发送 IsFinal 结果，以便调用方感知结束。
	StreamRecognize(ctx context.Context, audioCh <-chan PCMChunk, resultCh chan<- ASRStreamResult, opts RecognizeOptions) error

	// Capabilities 返回该供应商的能力声明，用于路由决策与 UI 展示。
	Capabilities() ProviderCapabilities
}

// ProviderCapabilities 声明供应商的能力边界，供调用方在路由时决策
// （例如：整段批量优先本地引擎、流式优先支持流式的引擎）。
type ProviderCapabilities struct {
	Name            string   // 供应商标识（与 ASRProvider.Name 一致）
	Streaming       bool     // 是否支持流式识别
	MaxDurationMs   int64    // 单次最长音频时长（0=无限制）
	SupportedFormat []string // 支持的音频格式：pcm/wav/mp3/opus
	SampleRates     []int    // 支持的采样率（Hz）
	HotwordSupport  bool     // 是否支持热词增强
	DeploymentMode  string   // self_hosted / cloud_api
}

// RecognizeOptions 携带一次识别的可选参数。
// 各实现按需取用；字段为可选，零值表示「未指定、使用默认」。
// 业务身份（租户/用户等）不在此定义，应由调用方通过 context.Context 传递。
type RecognizeOptions struct {
	Hotwords   []Hotword // 热词增强（仅 HotwordSupport 的实现消费）
	SampleRate int       // 采样率 Hz（0=使用实现默认值）
	Language   string    // 语言：zh/en/auto
}

// Hotword 表示一条热词增强规则。
type Hotword struct {
	Word   string  // 原文
	Target string  // 期望识别结果
	Weight float64 // 权重
}

// ASRResult 是同步识别的完整结果。
type ASRResult struct {
	RequestID    string    // 请求标识（可选）
	Text         string    // 完整识别文本
	Segments     []Segment // 分段（时间戳对齐，可选）
	Confidence   float64   // 整体置信度 0~1
	DurationMs   int64     // 音频时长（毫秒）
	ProviderName string    // 实际处理的引擎名
}

// Segment 是带时间戳的识别分段。
type Segment struct {
	StartMs    int64   // 起始时间（毫秒）
	EndMs      int64   // 结束时间（毫秒）
	Text       string  // 分段文本
	Confidence float64 // 分段置信度 0~1
}

// ASRStreamResult 是流式识别的一条增量结果。
type ASRStreamResult struct {
	RequestID   string  // 请求标识（可选）
	Text        string  // 当前累积文本（或本段增量，由实现约定）
	Confidence  float64 // 置信度 0~1
	IsFinal     bool    // 是否为最终结果
	TimestampMs int64   // 时间戳（毫秒，可选）
}

// PCMChunk 是一块 PCM 音频分片。
type PCMChunk struct {
	Data        []byte // PCM 样本数据（16-bit little-endian）
	Timestamp   int64  // 时间戳（毫秒，可选）
	VoiceActive bool   // 是否检测到人声（VAD，可选）
}
