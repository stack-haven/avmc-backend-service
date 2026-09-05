// Package biz · asr.go
// ASR 语音识别业务编排（同步 + 流式）。
//
// 职责：
//   - 路由到合适 ASR provider（batch / stream）
//   - 落盘原始音频到 upload/audio/<tenant>/<session>.<ext>
//   - 集成文本增强（EnhancementUsecase.EnhanceText）
//   - 维护 in-memory ring buffer 记录最近 N 条识别（不依赖 DB）
//
// 设计原则：
//   - usecase 不感知 transport（HTTP / gRPC）；service 层做 proto 转换
//   - 流式识别通过 channel 解耦音频输入/结果输出，usecase 不阻塞调用方
//   - 音频保存路径相对工作目录；最终通过 audio_path 字段返回
//   - 识别记录 ring buffer 上限（默认 1000），超出淘汰最旧
package biz

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	v1 "backend-service/api/evie/tool/v1"
	"backend-service/app/evie/tool/internal/conf"
	asrPkg "backend-service/pkg/asr"
	asrAudio "backend-service/pkg/asr/audio"
	pid "backend-service/pkg/utils/id"
)

// ASRRecord 识别记录（内存版，不落库）。
type ASRRecord struct {
	ID           string    // = session_id
	UserID       string
	TenantID     string
	RawText      string
	EnhancedText string
	AudioPath    string
	AudioFormat  string
	ProviderName string
	Confidence   float64
	DurationMs   int64
	CreatedAt    time.Time
}

// ASRUsecase ASR 业务编排。
type ASRUsecase struct {
	batchProvider  asrPkg.ASRProvider
	streamProvider asrPkg.ASRProvider
	enhancer       *EnhancementUsecase
	conf           *conf.Asr
	log            *log.Helper

	recordsMu  sync.RWMutex
	records    []*ASRRecord
	maxRecords int
}

// NewASRUsecase 构造 ASR usecase。
//
// providers 来自 data.ASRProviders（同时包含 batch + stream）。
func NewASRUsecase(
	providers *ASRProviders,
	enhancer *EnhancementUsecase,
	c *conf.Asr,
	logger log.Logger,
) *ASRUsecase {
	var batch asrPkg.ASRProvider
	var stream asrPkg.ASRProvider
	if providers != nil {
		batch = providers.Batch
		stream = providers.Stream
	}
	if batch == nil && stream == nil {
		panic("biz.NewASRUsecase: at least one ASR provider required")
	}
	return &ASRUsecase{
		batchProvider:  batch,
		streamProvider: stream,
		enhancer:       enhancer,
		conf:           c,
		log:            log.NewHelper(log.With(logger, "module", "biz/asr")),
		records:        make([]*ASRRecord, 0, 1024),
		maxRecords:     1000,
	}
}

// ASRProviders 整段 + 流式 provider 聚合（wire 友好的包装类型）。
//
// 定义在 biz 包；data 层构造它（data → biz 是已存在的合法方向，data 不依赖 ASRProviders 类型结构）。
type ASRProviders struct {
	Batch  asrPkg.ASRProvider
	Stream asrPkg.ASRProvider
}

// RecognizeResult 业务结果（service 层转 proto）。
type RecognizeResult struct {
	RequestID      string
	SessionID      string
	RawText        string
	EnhancedText   string
	Confidence     float64
	DurationMs     int64
	ProviderName   string
	AudioPath      string
	AudioFormat    string
	EnhanceChanges []*v1.EnhanceChange
	EnhanceStatus  int32
	ProcessingMs   int64
	ErrorMessage   string
}

// StreamResult 业务流式结果（service 层转 proto）。
type StreamResult struct {
	RequestID    string
	SessionID    string
	Text         string
	IsFinal      bool
	Confidence   float64
	TimestampMs  int64
	AudioPath    string
	EnhancedText string
}

// AudioFormat 音频格式（与 proto 对齐）。
type AudioFormat struct {
	Encoding   string // "pcm"/"wav"/"mp3"/"opus"
	SampleRate int    // Hz
	BitDepth   int    // 16/24/32
	Language   string // "zh-CN" / "en-US"
}

// Recognize 同步识别一段音频（≤60s）。
//
// 流程：
//  1. 规范化音频（WAV/PCM → WAV 容器，便于播放器与存档）
//  2. 调 batchProvider.Recognize
//  3. （可选）调 Enhancer 跑 8 层增强
//  4. 落盘原始音频到 upload/audio/<tenant>/<session>.<ext>
//  5. 写 ring buffer，返回 RecognizeResult
func (uc *ASRUsecase) Recognize(
	ctx context.Context,
	userID, tenantID string,
	rawAudio []byte,
	format AudioFormat,
	sessionID string,
	enableEnhancement bool,
) (*RecognizeResult, error) {
	if len(rawAudio) == 0 {
		return nil, fmt.Errorf("biz.asr: empty audio data")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("biz.asr: tenantID required (auth missing)")
	}
	if uc.batchProvider == nil {
		return nil, fmt.Errorf("biz.asr: no batch provider configured")
	}
	if sessionID == "" {
		sessionID = pid.NewSessionID(pid.SessionIDPrefixASR)
	}

	// 1. 规范化音频（保留原始 ext 用于落盘）
	audioBytes, ext := normalizeAudio(rawAudio, format)

	// 2. ASR 识别
	opts := asrPkg.RecognizeOptions{
		SampleRate: format.SampleRate,
		Language:   format.Language,
	}
	asrResult, err := uc.batchProvider.Recognize(ctx, audioBytes, opts)
	if err != nil {
		uc.log.Warnf("recognize failed: %v", err)
		return nil, fmt.Errorf("biz.asr: recognize failed: %w", err)
	}
	if asrResult == nil {
		return nil, fmt.Errorf("biz.asr: empty result")
	}

	// 3. 文本增强（可选）
	var (
		enhText      = asrResult.Text
		enhChanges   []*v1.EnhanceChange
		enhStatus    int32 = 1
		processingMs int64
		errMsg       string
	)
	if enableEnhancement && uc.enhancer != nil && asrResult.Text != "" {
		start := time.Now()
		enh, enhErr := uc.enhancer.EnhanceText(ctx, asrResult.Text, tenantID)
		processingMs = time.Since(start).Milliseconds()
		if enhErr != nil {
			uc.log.Warnf("enhance text failed (raw kept): %v", enhErr)
			errMsg = enhErr.Error()
			enhStatus = 2 // DEGRADED
		} else {
			enhText = enh.EnhancedText
			enhStatus = enh.Status
			enhChanges = enh.Changes
			errMsg = enh.ErrorMessage
		}
	}

	// 4. 保存音频 + 写记录
	audioPath, saveErr := uc.saveAudio(tenantID, sessionID, ext, audioBytes)
	if saveErr != nil {
		uc.log.Warnf("save audio failed: %v", saveErr)
		// 不阻断主流程
	}
	rec := &ASRRecord{
		ID:           sessionID,
		UserID:       userID,
		TenantID:     tenantID,
		RawText:      asrResult.Text,
		EnhancedText: enhText,
		AudioPath:    audioPath,
		AudioFormat:  ext,
		ProviderName: asrResult.ProviderName,
		Confidence:   asrResult.Confidence,
		DurationMs:   asrResult.DurationMs,
		CreatedAt:    time.Now(),
	}
	uc.appendRecord(rec)

	return &RecognizeResult{
		RequestID:      uuid.NewString(),
		SessionID:      sessionID,
		RawText:        asrResult.Text,
		EnhancedText:   enhText,
		Confidence:     asrResult.Confidence,
		DurationMs:     asrResult.DurationMs,
		ProviderName:   asrResult.ProviderName,
		AudioPath:      audioPath,
		AudioFormat:    ext,
		EnhanceChanges: enhChanges,
		EnhanceStatus:  enhStatus,
		ProcessingMs:   processingMs,
		ErrorMessage:   errMsg,
	}, nil
}

// StreamRecognize 流式识别（音频分片输入 → 增量结果输出 + 最终增强）。
//
// audioCh：调用方推送 PCM/encoded 分片，完成后 close
// resultCh：usecase 推送 StreamResult；最终 is_final=true 帧携带 enhanced_text + audio_path
//
// 流程：
//   - 1 个 collect goroutine：消费 audioCh 累计字节 + 转发到 providerIn
//   - 1 个 provider goroutine：跑 provider.StreamRecognize(providerIn, providerOut)
//   - 主循环：消费 providerOut → 非最终帧透传到 resultCh；最终帧暂存
//   - audioCh 关闭后等 provider 跑完，对最终文本跑一次增强 + 落盘 WAV
//   - 推送 is_final=true 帧（带 enhanced_text + audio_path）
func (uc *ASRUsecase) StreamRecognize(
	ctx context.Context,
	userID, tenantID string,
	audioCh <-chan []byte,
	resultCh chan<- StreamResult,
	format AudioFormat,
	sessionID string,
	enableEnhancement bool,
) (*ASRRecord, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("biz.asr: tenantID required (auth missing)")
	}
	if uc.streamProvider == nil {
		return nil, fmt.Errorf("biz.asr: no stream provider configured")
	}
	if sessionID == "" {
		sessionID = pid.NewSessionID(pid.SessionIDPrefixASR)
	}

	providerIn := make(chan asrPkg.PCMChunk, 64)
	providerOut := make(chan asrPkg.ASRStreamResult, 64)

	// 收集完整音频（用于最终落盘 + 增强）
	var audioBuf []byte
	collectDone := make(chan struct{})

	// collect goroutine：消费 audioCh 累计字节 + 转发到 providerIn
	go func() {
		defer close(collectDone)
		defer close(providerIn)
		for chunk := range audioCh {
			audioBuf = append(audioBuf, chunk...)
			// provider 可能处理慢；阻塞到 provider 消费
			select {
			case providerIn <- asrPkg.PCMChunk{Data: chunk}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// provider goroutine
	providerDone := make(chan error, 1)
	go func() {
		opts := asrPkg.RecognizeOptions{
			SampleRate: format.SampleRate,
			Language:   format.Language,
		}
		providerDone <- uc.streamProvider.StreamRecognize(ctx, providerIn, providerOut, opts)
	}()

	// 主循环：消费 providerOut → 透传非最终帧 + 收集最终帧
	var finalText string
	var finalConfidence float64
	for r := range providerOut {
		if r.IsFinal {
			// 最终帧：先暂存，等 audioCh 关闭后再跑增强 + 落盘
			if r.Text != "" {
				finalText = r.Text
			}
			if r.Confidence > 0 {
				finalConfidence = r.Confidence
			}
			// 仍要透传（让客户端提前知道最终结果到达）
			select {
			case resultCh <- StreamResult{
				RequestID:   uuid.NewString(),
				SessionID:   sessionID,
				Text:        r.Text,
				IsFinal:     true,
				Confidence:  r.Confidence,
				TimestampMs: r.TimestampMs,
			}:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		} else {
			if r.Text != "" {
				finalText = r.Text // 增量累计
			}
			if r.Confidence > 0 {
				finalConfidence = r.Confidence
			}
			select {
			case resultCh <- StreamResult{
				RequestID:   uuid.NewString(),
				SessionID:   sessionID,
				Text:        r.Text,
				IsFinal:     false,
				Confidence:  r.Confidence,
				TimestampMs: r.TimestampMs,
			}:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	// 等待 collect 完成 + provider 完成
	<-collectDone
	provErr := <-providerDone

	// 最终结果：跑增强（一次）+ 落盘音频
	var (
		enhText   = finalText
		errMsg    string
		enhStatus int32 = 1
	)
	if enableEnhancement && uc.enhancer != nil && finalText != "" {
		enh, enhErr := uc.enhancer.EnhanceText(ctx, finalText, tenantID)
		if enhErr != nil {
			uc.log.Warnf("stream enhance failed: %v", enhErr)
			errMsg = enhErr.Error()
			enhStatus = 2
		} else {
			enhText = enh.EnhancedText
			enhStatus = enh.Status
			errMsg = enh.ErrorMessage
		}
	}

	// 落盘音频（流式默认 PCM → WAV）
	sr := format.SampleRate
	if sr <= 0 {
		sr = 16000
	}
	bd := format.BitDepth
	if bd <= 0 {
		bd = 16
	}
	audioPath, _ := uc.saveAudio(tenantID, sessionID, "wav",
		asrAudio.PCMToWAV(audioBuf, sr, 1, bd))

	rec := &ASRRecord{
		ID:           sessionID,
		UserID:       userID,
		TenantID:     tenantID,
		RawText:      finalText,
		EnhancedText: enhText,
		AudioPath:    audioPath,
		AudioFormat:  "wav",
		ProviderName: uc.streamProvider.Name(),
		Confidence:   finalConfidence,
		DurationMs:   0,
		CreatedAt:    time.Now(),
	}
	uc.appendRecord(rec)

	// 最终帧（携带增强文本 + audio_path）
	select {
	case resultCh <- StreamResult{
		RequestID:    uuid.NewString(),
		SessionID:    sessionID,
		Text:         finalText,
		IsFinal:      true,
		Confidence:   finalConfidence,
		AudioPath:    audioPath,
		EnhancedText: enhText,
	}:
	case <-ctx.Done():
	}
	// 关闭 resultCh 告知消费方识别结束
	close(resultCh)

	if provErr != nil && !errors.Is(provErr, context.Canceled) {
		return rec, fmt.Errorf("biz.asr: stream provider: %w", provErr)
	}
	_ = errMsg
	_ = enhStatus
	return rec, nil
}

// normalizeAudio 根据 encoding 统一为 WAV（落盘用），返回 audio bytes + ext。
func normalizeAudio(audio []byte, f AudioFormat) (out []byte, ext string) {
	switch f.Encoding {
	case "wav":
		return audio, "wav"
	case "mp3":
		return audio, "mp3"
	case "opus":
		return audio, "opus"
	default: // pcm / "" / unknown
		sr := f.SampleRate
		if sr <= 0 {
			sr = 16000
		}
		bd := f.BitDepth
		if bd <= 0 {
			bd = 16
		}
		return asrAudio.PCMToWAV(audio, sr, 1, bd), "wav"
	}
}

// saveAudio 把音频写到 <audio_dir>/<tenant_id>/<session_id>.<ext>。
//
// 落盘失败不阻断主流程（仅 warn）。
func (uc *ASRUsecase) saveAudio(tenantID, sessionID, ext string, audio []byte) (string, error) {
	if uc.conf == nil || uc.conf.Upload == nil || uc.conf.Upload.AudioDir == "" {
		return "", fmt.Errorf("biz.asr: audio_dir not configured")
	}
	relDir := filepath.Join(uc.conf.Upload.AudioDir, tenantID)
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		return "", fmt.Errorf("biz.asr: mkdir %s: %w", relDir, err)
	}
	relPath := filepath.Join(relDir, sessionID+"."+ext)
	if err := os.WriteFile(relPath, audio, 0o644); err != nil {
		return "", fmt.Errorf("biz.asr: write %s: %w", relPath, err)
	}
	return relPath, nil
}

// appendRecord 写入 ring buffer（FIFO；超 maxRecords 淘汰最旧）。
func (uc *ASRUsecase) appendRecord(rec *ASRRecord) {
	uc.recordsMu.Lock()
	defer uc.recordsMu.Unlock()
	uc.records = append(uc.records, rec)
	if len(uc.records) > uc.maxRecords {
		excess := len(uc.records) - uc.maxRecords
		uc.records = uc.records[excess:]
	}
}

// ListRecords 分页列出记录（按时间倒序）。
func (uc *ASRUsecase) ListRecords(_ context.Context, pageSize int32, pageToken string) ([]*ASRRecord, int32, string) {
	uc.recordsMu.RLock()
	defer uc.recordsMu.RUnlock()

	// 时间倒序
	sorted := make([]*ASRRecord, len(uc.records))
	copy(sorted, uc.records)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	// 简化分页：page_token = offset 字符串
	offset := 0
	if pageToken != "" {
		_, _ = fmt.Sscanf(pageToken, "%d", &offset)
	}
	if offset > len(sorted) {
		offset = len(sorted)
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	end := offset + int(pageSize)
	if end > len(sorted) {
		end = len(sorted)
	}
	page := sorted[offset:end]
	next := ""
	if end < len(sorted) {
		next = fmt.Sprintf("%d", end)
	}
	return page, int32(len(sorted)), next
}

// GetRecord 按 ID 取记录。
func (uc *ASRUsecase) GetRecord(_ context.Context, id string) (*ASRRecord, bool) {
	uc.recordsMu.RLock()
	defer uc.recordsMu.RUnlock()
	for _, r := range uc.records {
		if r.ID == id {
			return r, true
		}
	}
	return nil, false
}

// GetRecordAudio 读原始音频 bytes + content-type。
func (uc *ASRUsecase) GetRecordAudio(_ context.Context, id string) ([]byte, string, error) {
	rec, ok := uc.GetRecord(context.Background(), id)
	if !ok {
		return nil, "", fmt.Errorf("biz.asr: record not found: %s", id)
	}
	if rec.AudioPath == "" {
		return nil, "", fmt.Errorf("biz.asr: record has no audio")
	}
	data, err := os.ReadFile(rec.AudioPath)
	if err != nil {
		return nil, "", fmt.Errorf("biz.asr: read audio: %w", err)
	}
	ct := "audio/" + rec.AudioFormat
	switch rec.AudioFormat {
	case "mp3":
		ct = "audio/mpeg"
	}
	return data, ct, nil
}