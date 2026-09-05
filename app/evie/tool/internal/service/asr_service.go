// Package service · asr_service.go
// ASRService proto 实现：同步识别 + 流式识别 + 记录查询。
//
// 依赖：
//   - *biz.ASRUsecase（M7 编排）
//   - data.AuthInfoFromContext（取 user_id/tenant_id）
package service

import (
	"context"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	v1 "backend-service/api/evie/tool/v1"
	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/data"
)

// ASRService 同步 + 流式识别。
type ASRService struct {
	v1.UnimplementedASRServiceServer
	uc *biz.ASRUsecase
}

// NewASRService 构造。
func NewASRService(uc *biz.ASRUsecase) *ASRService {
	return &ASRService{uc: uc}
}

// Recognize 同步识别。
func (s *ASRService) Recognize(ctx context.Context, req *v1.RecognizeRequest) (*v1.RecognizeResponse, error) {
	auth, ok := data.AuthInfoFromContext(ctx)
	if !ok || auth == nil {
		return nil, status.Error(codes.Unauthenticated, "missing auth info")
	}

	format := protoAudioToBiz(req.GetFormat())

	res, err := s.uc.Recognize(
		ctx,
		auth.UserID, auth.TenantID,
		req.GetAudioData(),
		format,
		req.GetSessionId(),
		req.GetEnableEnhancement(),
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	changes := res.EnhanceChanges
	if changes == nil {
		changes = []*v1.EnhanceChange{}
	}

	return &v1.RecognizeResponse{
		RequestId:       res.RequestID,
		SessionId:       res.SessionID,
		RawText:         res.RawText,
		EnhancedText:    res.EnhancedText,
		Confidence:      float32(res.Confidence),
		DurationMs:      res.DurationMs,
		IsFinal:         true,
		ProviderName:    res.ProviderName,
		AudioPath:       res.AudioPath,
		Changes:         changes,
		Status:          res.EnhanceStatus,
		ProcessingTimeMs: res.ProcessingMs,
		ErrorMessage:    res.ErrorMessage,
	}, nil
}

// StreamRecognize 流式识别（gRPC bidirectional stream）。
//
// 约定：session_id / format 通过 gRPC metadata 传递（避免污染 AudioChunk 协议）：
//   - x-session-id        (string)
//   - x-sample-rate       (int)
//   - x-bit-depth         (int, optional)
//   - x-encoding          (string, optional: pcm/wav)
func (s *ASRService) StreamRecognize(stream v1.ASRService_StreamRecognizeServer) error {
	ctx := stream.Context()

	auth, ok := data.AuthInfoFromContext(ctx)
	if !ok || auth == nil {
		return status.Error(codes.Unauthenticated, "missing auth info")
	}

	// 从 metadata 取 session_id / format
	md, _ := metadata.FromIncomingContext(ctx)
	sessionID := firstMeta(md, "x-session-id")
	sampleRate := intMeta(md, "x-sample-rate", 16000)
	bitDepth := intMeta(md, "x-bit-depth", 16)
	encoding := firstMeta(md, "x-encoding")
	if encoding == "" {
		encoding = "pcm"
	}
	format := biz.AudioFormat{
		Encoding:   encoding,
		SampleRate: sampleRate,
		BitDepth:   bitDepth,
	}

	audioCh := make(chan []byte, 64)
	resultCh := make(chan biz.StreamResult, 64)

	// 收 audio goroutine
	go func() {
		defer close(audioCh)
		for {
			chunk, err := stream.Recv()
			if err != nil {
				return // io.EOF / ctx cancel
			}
			if len(chunk.GetData()) > 0 {
				select {
				case audioCh <- chunk.GetData():
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// 推 result goroutine
	sendDone := make(chan error, 1)
	go func() {
		for r := range resultCh {
			sr := &v1.StreamResult{
				RequestId:    r.RequestID,
				SessionId:    r.SessionID,
				Text:         r.Text,
				IsFinal:      r.IsFinal,
				Confidence:   float32(r.Confidence),
				TimestampMs:  r.TimestampMs,
				AudioPath:    r.AudioPath,
				EnhancedText: r.EnhancedText,
			}
			if err := stream.Send(sr); err != nil {
				sendDone <- err
				return
			}
		}
		sendDone <- nil
	}()

	_, recErr := s.uc.StreamRecognize(
		ctx,
		auth.UserID, auth.TenantID,
		audioCh, resultCh,
		format, sessionID,
		true, // 流式默认开启增强（仅最终帧）
	)

	sErr := <-sendDone
	if recErr != nil && sErr == nil {
		return status.Error(codes.Internal, recErr.Error())
	}
	if sErr != nil {
		return status.Error(codes.Internal, sErr.Error())
	}
	return nil
}

// ListRecords 分页列出记录。
func (s *ASRService) ListRecords(ctx context.Context, req *v1.ListAsrRecordsRequest) (*v1.ListAsrRecordsResponse, error) {
	_, ok := data.AuthInfoFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing auth info")
	}
	page, total, next := s.uc.ListRecords(ctx, req.GetPageSize(), req.GetPageToken())
	out := make([]*v1.AsrRecord, 0, len(page))
	for _, r := range page {
		out = append(out, recordToProto(r))
	}
	return &v1.ListAsrRecordsResponse{
		Records:       out,
		Total:         total,
		NextPageToken: next,
	}, nil
}

// GetRecord 取单条记录。
func (s *ASRService) GetRecord(ctx context.Context, req *v1.GetRecordRequest) (*v1.AsrRecord, error) {
	_, ok := data.AuthInfoFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing auth info")
	}
	rec, found := s.uc.GetRecord(ctx, req.GetId())
	if !found {
		return nil, status.Error(codes.NotFound, "record not found")
	}
	return recordToProto(rec), nil
}

// GetRecordAudio 取原始音频字节。
func (s *ASRService) GetRecordAudio(ctx context.Context, req *v1.GetRecordAudioRequest) (*v1.GetRecordAudioResponse, error) {
	_, ok := data.AuthInfoFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "record not found")
	}
	audio, ct, err := s.uc.GetRecordAudio(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &v1.GetRecordAudioResponse{
		AudioData:   audio,
		ContentType: ct,
	}, nil
}

// protoAudioToBiz proto.AudioFormat → biz.AudioFormat。
func protoAudioToBiz(f *v1.AudioFormat) biz.AudioFormat {
	if f == nil {
		return biz.AudioFormat{Encoding: "pcm", SampleRate: 16000, BitDepth: 16}
	}
	return biz.AudioFormat{
		Encoding:   f.GetEncoding(),
		SampleRate: int(f.GetSampleRate()),
		BitDepth:   int(f.GetBitDepth()),
	}
}

// recordToProto biz.ASRRecord → proto.AsrRecord。
func recordToProto(r *biz.ASRRecord) *v1.AsrRecord {
	return &v1.AsrRecord{
		Id:           r.ID,
		UserId:       r.UserID,
		TenantId:     r.TenantID,
		RawText:      r.RawText,
		EnhancedText: r.EnhancedText,
		AudioPath:    r.AudioPath,
		ProviderName: r.ProviderName,
		CreatedAt:    r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// firstMeta 从 metadata 取第一个值。
func firstMeta(md metadata.MD, key string) string {
	if len(md) == 0 {
		return ""
	}
	if vals := md.Get(key); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// intMeta 从 metadata 取 int。
func intMeta(md metadata.MD, key string, def int) int {
	if v := firstMeta(md, key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}