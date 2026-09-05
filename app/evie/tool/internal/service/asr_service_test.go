package service_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/go-kratos/kratos/v2/log"

	v1 "backend-service/api/evie/tool/v1"
	v1conf "backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/service"

	asrPkg "backend-service/pkg/asr"
)

// mockBatchProvider service 测试用 mock。
type mockBatchProvider2 struct {
	mu         sync.Mutex
	calls      int
	text       string
	confidence float64
}

func (m *mockBatchProvider2) Name() string { return "mock-batch" }
func (m *mockBatchProvider2) Recognize(_ context.Context, _ []byte, _ asrPkg.RecognizeOptions) (*asrPkg.ASRResult, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return &asrPkg.ASRResult{Text: m.text, Confidence: m.confidence, ProviderName: m.Name()}, nil
}
func (m *mockBatchProvider2) StreamRecognize(context.Context, <-chan asrPkg.PCMChunk, chan<- asrPkg.ASRStreamResult, asrPkg.RecognizeOptions) error {
	return nil
}
func (m *mockBatchProvider2) Capabilities() asrPkg.ProviderCapabilities {
	return asrPkg.ProviderCapabilities{Name: m.Name()}
}

// mockStreamProvider2 流式识别 mock（per pkg/asr 契约：close out channel）。
type mockStreamProvider2 struct {
	finalText string
}

func (m *mockStreamProvider2) Name() string { return "mock-stream" }
func (m *mockStreamProvider2) Recognize(context.Context, []byte, asrPkg.RecognizeOptions) (*asrPkg.ASRResult, error) {
	return &asrPkg.ASRResult{ProviderName: m.Name()}, nil
}
func (m *mockStreamProvider2) StreamRecognize(_ context.Context, in <-chan asrPkg.PCMChunk, out chan<- asrPkg.ASRStreamResult, _ asrPkg.RecognizeOptions) error {
	defer close(out)
	for range in {
		out <- asrPkg.ASRStreamResult{Text: "partial", IsFinal: false, Confidence: 0.9}
	}
	out <- asrPkg.ASRStreamResult{Text: m.finalText, IsFinal: true, Confidence: 0.95}
	return nil
}
func (m *mockStreamProvider2) Capabilities() asrPkg.ProviderCapabilities {
	return asrPkg.ProviderCapabilities{Name: m.Name(), Streaming: true}
}

func makeService(t *testing.T) *service.ASRService {
	t.Helper()
	dir := t.TempDir()
	dictPath := filepath.Join(dir, "system.json")
	if err := os.WriteFile(dictPath, []byte(`{"version":"t","entries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	providers := &biz.ASRProviders{
		Batch:  &mockBatchProvider2{text: "识别结果", confidence: 0.9},
		Stream: &mockStreamProvider2{finalText: "流式最终"},
	}
	uc := biz.NewASRUsecase(providers, nil, &v1conf.Asr{
		Upload: &v1conf.Asr_Upload{AudioDir: dir + "/audio"},
	}, log.DefaultLogger)
	return service.NewASRService(uc)
}

func makeCtxWithAuth(userID, tenantID string) context.Context {
	auth := &data.AuthInfo{UserID: userID, TenantID: tenantID, AccessToken: "test"}
	return data.WithAuthInfo(context.Background(), auth)
}

// TestASRService_Recognize 测试 sync 识别。
func TestASRService_Recognize(t *testing.T) {
	svc := makeService(t)
	ctx := makeCtxWithAuth("u1", "158")

	resp, err := svc.Recognize(ctx, &v1.RecognizeRequest{
		Format:             &v1.AudioFormat{Encoding: "wav", SampleRate: 16000, BitDepth: 16},
		AudioData:          []byte("fake-wav-bytes"),
		EnableEnhancement: false,
	})
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if resp.RawText != "识别结果" {
		t.Errorf("RawText = %q, want %q", resp.RawText, "识别结果")
	}
	if !resp.IsFinal {
		t.Error("IsFinal should be true")
	}
	if resp.SessionId == "" {
		t.Error("SessionId empty")
	}
	if resp.AudioPath == "" {
		t.Error("AudioPath empty")
	}
}

// TestASRService_Recognize_NoAuth 测试无 auth 时拒绝。
func TestASRService_Recognize_NoAuth(t *testing.T) {
	svc := makeService(t)
	_, err := svc.Recognize(context.Background(), &v1.RecognizeRequest{
		Format:    &v1.AudioFormat{Encoding: "wav"},
		AudioData: []byte("x"),
	})
	if err == nil {
		t.Error("expected error without auth")
	}
	if st, ok := status.FromError(err); !ok || st.Code().String() != "Unauthenticated" {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

// TestASRService_ListRecords 测试分页列表。
func TestASRService_ListRecords(t *testing.T) {
	svc := makeService(t)
	ctx := makeCtxWithAuth("u1", "158")

	// 插入 3 条
	for i := 0; i < 3; i++ {
		_, err := svc.Recognize(ctx, &v1.RecognizeRequest{
			Format:    &v1.AudioFormat{Encoding: "wav", SampleRate: 16000, BitDepth: 16},
			AudioData: []byte("x"),
		})
		if err != nil {
			t.Fatalf("Recognize #%d: %v", i, err)
		}
	}

	resp, err := svc.ListRecords(ctx, &v1.ListAsrRecordsRequest{PageSize: 2})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(resp.Records) != 2 {
		t.Errorf("page1 len = %d, want 2", len(resp.Records))
	}
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
	if resp.NextPageToken == "" {
		t.Error("next token should be set")
	}
}

// TestASRService_GetRecord_GetAudio 测试单条 + 音频读。
func TestASRService_GetRecord_GetAudio(t *testing.T) {
	svc := makeService(t)
	ctx := makeCtxWithAuth("u1", "158")

	recResp, err := svc.Recognize(ctx, &v1.RecognizeRequest{
		Format:    &v1.AudioFormat{Encoding: "wav", SampleRate: 16000, BitDepth: 16},
		AudioData: []byte("hello-audio"),
	})
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}

	// GetRecord
	got, err := svc.GetRecord(ctx, &v1.GetRecordRequest{Id: recResp.SessionId})
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if got.Id != recResp.SessionId {
		t.Errorf("GetRecord id = %q, want %q", got.Id, recResp.SessionId)
	}

	// GetRecordAudio
	audio, err := svc.GetRecordAudio(ctx, &v1.GetRecordAudioRequest{Id: recResp.SessionId})
	if err != nil {
		t.Fatalf("GetRecordAudio: %v", err)
	}
	if len(audio.AudioData) == 0 {
		t.Error("AudioData empty")
	}
	if audio.ContentType != "audio/wav" {
		t.Errorf("ContentType = %q, want audio/wav", audio.ContentType)
	}
}

// TestASRService_StreamRecognize 测试双向流（gRPC）。
func TestASRService_StreamRecognize(t *testing.T) {
	svc := makeService(t)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-session-id", "test-session-1",
		"x-sample-rate", "16000",
	))
	ctx = data.WithAuthInfo(ctx, &data.AuthInfo{UserID: "u1", TenantID: "158"})

	// 用一个内部 mock stream server 模拟 gRPC
	stream := newFakeStreamServer(ctx)

	// 启动 send goroutine 模拟客户端发 3 个 chunk 然后 close
	go func() {
		for i := 0; i < 3; i++ {
			stream.sendRecv(&v1.AudioChunk{Data: []byte{1, 2, 3}})
		}
		stream.closeRecv()
	}()

	err := svc.StreamRecognize(stream)
	if err != nil {
		t.Fatalf("StreamRecognize: %v", err)
	}
	if len(stream.sent) == 0 {
		t.Error("no results sent")
	}
	// 最后一个应该是 IsFinal
	last := stream.sent[len(stream.sent)-1]
	if !last.IsFinal {
		t.Error("last should be IsFinal")
	}
	if last.AudioPath == "" {
		t.Error("last should have AudioPath")
	}
}

// fakeStreamServer 模拟 v1.ASRService_StreamRecognizeServer。
type fakeStreamServer struct {
	ctx    context.Context
	recvCh chan *v1.AudioChunk
	sent   []*v1.StreamResult
}

func newFakeStreamServer(ctx context.Context) *fakeStreamServer {
	return &fakeStreamServer{
		ctx:    ctx,
		recvCh: make(chan *v1.AudioChunk, 8),
	}
}

func (f *fakeStreamServer) Context() context.Context { return f.ctx }
func (f *fakeStreamServer) Recv() (*v1.AudioChunk, error) {
	c, ok := <-f.recvCh
	if !ok {
		return nil, io.EOF
	}
	return c, nil
}
func (f *fakeStreamServer) Send(r *v1.StreamResult) error {
	f.sent = append(f.sent, r)
	return nil
}
func (f *fakeStreamServer) sendRecv(c *v1.AudioChunk) {
	f.recvCh <- c
}
func (f *fakeStreamServer) closeRecv() { close(f.recvCh) }
func (f *fakeStreamServer) SetHeader(metadata.MD) error  { return nil }
func (f *fakeStreamServer) SendHeader(metadata.MD) error { return nil }
func (f *fakeStreamServer) SetTrailer(metadata.MD)       {}
func (f *fakeStreamServer) SendMsg(interface{}) error    { return nil }
func (f *fakeStreamServer) RecvMsg(interface{}) error    { return nil }
