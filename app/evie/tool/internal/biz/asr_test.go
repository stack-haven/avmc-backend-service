package biz_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	v1conf "backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"

	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"

	asrPkg "backend-service/pkg/asr"
)

// mockBatchProvider 整段识别 mock。
type mockBatchProvider struct {
	mu        sync.Mutex
	calls     int
	text      string
	confidence float64
	durationMs int64
	err       error
}

func (m *mockBatchProvider) Name() string { return "mock-batch" }
func (m *mockBatchProvider) Recognize(_ context.Context, _ []byte, _ asrPkg.RecognizeOptions) (*asrPkg.ASRResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &asrPkg.ASRResult{
		Text:         m.text,
		Confidence:   m.confidence,
		DurationMs:   m.durationMs,
		ProviderName: m.Name(),
	}, nil
}
func (m *mockBatchProvider) StreamRecognize(context.Context, <-chan asrPkg.PCMChunk, chan<- asrPkg.ASRStreamResult, asrPkg.RecognizeOptions) error {
	return nil
}
func (m *mockBatchProvider) Capabilities() asrPkg.ProviderCapabilities {
	return asrPkg.ProviderCapabilities{Name: m.Name(), Streaming: false, SupportedFormat: []string{"pcm", "wav"}}
}

// mockStreamProvider 流式识别 mock。
type mockStreamProvider struct {
	mu       sync.Mutex
	calls    int
	chunks   int
	finalText string
	err      error
}

func (m *mockStreamProvider) Name() string { return "mock-stream" }
func (m *mockStreamProvider) Recognize(context.Context, []byte, asrPkg.RecognizeOptions) (*asrPkg.ASRResult, error) {
	return &asrPkg.ASRResult{Text: "", ProviderName: m.Name()}, nil
}
func (m *mockStreamProvider) StreamRecognize(_ context.Context, in <-chan asrPkg.PCMChunk, out chan<- asrPkg.ASRStreamResult, _ asrPkg.RecognizeOptions) error {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	defer close(out) // per pkg/asr 契约：provider 需 close resultCh
	// 每收到一个 chunk 推一个非最终帧
	for chunk := range in {
		m.mu.Lock()
		m.chunks++
		m.mu.Unlock()
		out <- asrPkg.ASRStreamResult{
			Text:        "chunk:" + string(chunk.Data),
			IsFinal:     false,
			Confidence:  0.9,
			TimestampMs: time.Now().UnixMilli(),
		}
	}
	// 收完推最终帧
	out <- asrPkg.ASRStreamResult{
		Text:        m.finalText,
		IsFinal:     true,
		Confidence:  0.95,
		TimestampMs: time.Now().UnixMilli(),
	}
	return m.err
}
func (m *mockStreamProvider) Capabilities() asrPkg.ProviderCapabilities {
	return asrPkg.ProviderCapabilities{Name: m.Name(), Streaming: true, SupportedFormat: []string{"pcm"}}
}

func makeConf(t *testing.T, audioDir string) *conf.Asr {
	t.Helper()
	return &conf.Asr{
		DefaultBatchProvider:  "mock-batch",
		DefaultStreamProvider: "mock-stream",
		Upload: &conf.Asr_Upload{
			AudioDir:      audioDir,
			RetentionDays: 7,
		},
	}
}

func makeEnhancer(t *testing.T, dir string) *biz.EnhancementUsecase {
	t.Helper()
	dictPath := filepath.Join(dir, "system.json")
	if err := os.WriteFile(dictPath, []byte(`{"version":"t","entries":[]}`), 0644); err != nil {
		t.Fatalf("write dict: %v", err)
	}
	vb, err := biz.NewVocabularyBuilder(&v1conf.SystemDict{Path: dictPath})
	if err != nil {
		t.Fatalf("vocab builder: %v", err)
	}
	enhConf := &v1conf.Enhancement{
		Pipeline:              []string{"cleaning", "filler", "vocab_matching", "alias_resolution", "deterministic_replacement", "phrase_standardization", "pinyin_correction", "fuzzy_matching", "context_correction"},
		PinyinThreshold:       0.85,
		FuzzyAutoThreshold:    0.80,
		FuzzySuggestThreshold: 0.60,
		LlmEnabled:            false,
	}
	policy := biz.NewPolicyFromConf(enhConf)
	reg := builtins.NewDefaultRegistry()
	pipe, err := textenhance.BuildPipeline(reg, policy)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}
	return biz.NewEnhancementUsecase(pipe, vb, policy)
}

// TestASRUsecase_Recognize 测试整段识别：返回 raw/enhanced + 落盘 + ring buffer。
func TestASRUsecase_Recognize(t *testing.T) {
	dir := t.TempDir()
	mockBatch := &mockBatchProvider{
		text:       "你好世界",
		confidence: 0.95,
		durationMs: 1200,
	}
	mockStream := &mockStreamProvider{finalText: "ignored"}
	uc := biz.NewASRUsecase(
		&biz.ASRProviders{Batch: mockBatch, Stream: mockStream},
		makeEnhancer(t, dir),
		makeConf(t, "upload/audio"),
		log.DefaultLogger,
	)

	// PCM 16kHz 16bit 一段
	audio := make([]byte, 16000*2) // 1 秒静音
	res, err := uc.Recognize(context.Background(),
		"u1", "158", audio,
		biz.AudioFormat{Encoding: "pcm", SampleRate: 16000, BitDepth: 16},
		"", true,
	)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if res.RawText != "你好世界" {
		t.Errorf("RawText = %q, want %q", res.RawText, "你好世界")
	}
	if res.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
	if res.AudioPath == "" {
		t.Error("AudioPath should not be empty")
	}

	// 验证落盘
	if _, err := os.Stat(res.AudioPath); err != nil {
		t.Errorf("audio file not found: %v", err)
	}

	// 验证 ring buffer
	rec, ok := uc.GetRecord(context.Background(), res.SessionID)
	if !ok {
		t.Error("record not in ring buffer")
	}
	if rec.RawText != "你好世界" {
		t.Errorf("ring buffer RawText = %q, want %q", rec.RawText, "你好世界")
	}
}

// TestASRUsecase_StreamRecognize 测试流式识别：分片输入 + 最终帧带增强。
func TestASRUsecase_StreamRecognize(t *testing.T) {
	dir := t.TempDir()
	mockStream := &mockStreamProvider{finalText: "流式最终文本"}
	mockBatch := &mockBatchProvider{}
	uc := biz.NewASRUsecase(
		&biz.ASRProviders{Batch: mockBatch, Stream: mockStream},
		makeEnhancer(t, dir),
		makeConf(t, "upload/audio"),
		log.DefaultLogger,
	)

	audioCh := make(chan []byte, 8)
	resultCh := make(chan biz.StreamResult, 16)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 推 3 个 chunk
	go func() {
		for i := 0; i < 3; i++ {
			audioCh <- []byte{1, 2, 3}
		}
		close(audioCh)
	}()

	// 收 result（等到 resultCh 关闭）
	var results []biz.StreamResult
	done := make(chan struct{})
	go func() {
		for r := range resultCh {
			results = append(results, r)
		}
		close(done)
	}()

	_, err := uc.StreamRecognize(ctx, "u1", "158", audioCh, resultCh,
		biz.AudioFormat{Encoding: "pcm", SampleRate: 16000, BitDepth: 16},
		"", true)
	if err != nil {
		t.Fatalf("StreamRecognize: %v", err)
	}

	<-done

	// 应至少有 3 个非最终帧 + 1 个最终帧
	if len(results) < 4 {
		t.Errorf("expected at least 4 results, got %d", len(results))
	}
	// 最后一帧是 final + 携带 enhanced_text + audio_path
	last := results[len(results)-1]
	if !last.IsFinal {
		t.Error("last result should be IsFinal")
	}
	if last.AudioPath == "" {
		t.Error("final result should have AudioPath")
	}
	if last.EnhancedText == "" {
		t.Error("final result should have EnhancedText")
	}
}

// TestASRUsecase_RingBufferEviction 测试 ring buffer 淘汰。
func TestASRUsecase_RingBufferEviction(t *testing.T) {
	dir := t.TempDir()
	uc := biz.NewASRUsecase(
		&biz.ASRProviders{Batch: &mockBatchProvider{text: "x"}, Stream: &mockStreamProvider{}},
		makeEnhancer(t, dir),
		makeConf(t, "upload/audio"),
		log.DefaultLogger,
	)

	// 触发 maxRecords+10 次识别
	for i := 0; i < 1010; i++ {
		_, _ = uc.Recognize(context.Background(), "u", "t", []byte{1, 2, 3},
			biz.AudioFormat{Encoding: "pcm", SampleRate: 16000, BitDepth: 16},
			"", false)
	}

	page, total, _ := uc.ListRecords(context.Background(), 100, "")
	if total != 1000 {
		t.Errorf("total = %d, want 1000 (after eviction)", total)
	}
	if len(page) != 100 {
		t.Errorf("page len = %d, want 100", len(page))
	}
}

// TestASRUsecase_GetRecordAudio 测试读音频。
func TestASRUsecase_GetRecordAudio(t *testing.T) {
	dir := t.TempDir()
	uc := biz.NewASRUsecase(
		&biz.ASRProviders{Batch: &mockBatchProvider{text: "x"}, Stream: &mockStreamProvider{}},
		makeEnhancer(t, dir),
		makeConf(t, "upload/audio"),
		log.DefaultLogger,
	)

	res, err := uc.Recognize(context.Background(), "u", "158", []byte{1, 2, 3, 4},
		biz.AudioFormat{Encoding: "wav", SampleRate: 16000, BitDepth: 16},
		"", false)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}

	audio, ct, err := uc.GetRecordAudio(context.Background(), res.SessionID)
	if err != nil {
		t.Fatalf("GetRecordAudio: %v", err)
	}
	if len(audio) == 0 {
		t.Error("audio bytes empty")
	}
	if ct != "audio/wav" {
		t.Errorf("content-type = %q, want audio/wav", ct)
	}
}

// TestASRUsecase_ListRecords_Pagination 测试分页。
func TestASRUsecase_ListRecords_Pagination(t *testing.T) {
	dir := t.TempDir()
	uc := biz.NewASRUsecase(
		&biz.ASRProviders{Batch: &mockBatchProvider{text: "x"}, Stream: &mockStreamProvider{}},
		makeEnhancer(t, dir),
		makeConf(t, "upload/audio"),
		log.DefaultLogger,
	)

	for i := 0; i < 25; i++ {
		_, _ = uc.Recognize(context.Background(), "u", "t", []byte{1},
			biz.AudioFormat{Encoding: "pcm", SampleRate: 16000, BitDepth: 16}, "", false)
		time.Sleep(time.Millisecond) // 保证 CreatedAt 顺序
	}

	// 第一页
	page1, total1, next1 := uc.ListRecords(context.Background(), 10, "")
	if total1 != 25 {
		t.Errorf("total1 = %d, want 25", total1)
	}
	if len(page1) != 10 {
		t.Errorf("page1 len = %d, want 10", len(page1))
	}
	if next1 == "" {
		t.Error("next1 should not be empty")
	}

	// 第二页
	page2, _, next2 := uc.ListRecords(context.Background(), 10, next1)
	if len(page2) != 10 {
		t.Errorf("page2 len = %d, want 10", len(page2))
	}
	if next2 == "" {
		t.Error("next2 should not be empty")
	}

	// 第三页
	page3, _, next3 := uc.ListRecords(context.Background(), 10, next2)
	if len(page3) != 5 {
		t.Errorf("page3 len = %d, want 5", len(page3))
	}
	if next3 != "" {
		t.Errorf("next3 should be empty, got %q", next3)
	}
}

// 编译期：mock provider 满足 asr.ASRProvider。
var (
	_ asrPkg.ASRProvider = (*mockBatchProvider)(nil)
	_ asrPkg.ASRProvider = (*mockStreamProvider)(nil)
)