package service

import (
	"context"
	"io"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/pkg/asr"
	"backend-service/pkg/auth/authn"
)

// ASRServiceService ASR 语音识别服务。
type ASRServiceService struct {
	pb.UnimplementedASRServiceServer
	auc      *biz.ASRUsecase
	enhancer *biz.EnhancementEngine
	policyUc *biz.EnhancementPolicyUsecase
	log      *log.Helper
}

// NewASRServiceService 创建 ASR 服务实例。
func NewASRServiceService(auc *biz.ASRUsecase, enhancer *biz.EnhancementEngine, policyUc *biz.EnhancementPolicyUsecase, logger log.Logger) *ASRServiceService {
	return &ASRServiceService{auc: auc, enhancer: enhancer, policyUc: policyUc, log: log.NewHelper(logger)}
}

// Recognize 同步识别。
func (s *ASRServiceService) Recognize(ctx context.Context, req *pb.RecognizeRequest) (*pb.RecognizeResponse, error) {
	result, err := s.auc.Recognize(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.RecognizeResponse{
		RequestId:    result.RequestID,
		Text:         result.Text,
		Segments:     toPbSegments(result.Segments),
		Confidence:   float32(result.Confidence),
		DurationMs:   result.DurationMs,
		IsFinal:      true,
		ProviderName: result.ProviderName,
	}, nil
}

// StreamRecognize 流式识别（双向流）：接收音频分片，回传增量文本。
func (s *ASRServiceService) StreamRecognize(stream pb.ASRService_StreamRecognizeServer) error {
	ctx := stream.Context()

	// 先接收首个分片以获取 sessionID
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	sessionID := first.GetSessionId()

	audioCh := make(chan asr.PCMChunk, 16)
	resultCh := make(chan asr.ASRStreamResult, 16)

	outcomeCh := make(chan streamOutcome, 1)
	go func() {
		recordID, audioURL, serr := s.auc.StreamRecognize(ctx, sessionID, audioCh, resultCh)
		close(resultCh)
		outcomeCh <- streamOutcome{recordID: recordID, audioURL: audioURL, err: serr}
	}()

	// 发送首个分片
	audioCh <- asr.PCMChunk{Data: first.GetData(), Timestamp: first.GetTimestampMs()}

	// 接收剩余音频分片 → audioCh
	go func() {
		defer close(audioCh)
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
			audioCh <- asr.PCMChunk{Data: chunk.GetData(), Timestamp: chunk.GetTimestampMs()}
		}
	}()

	// 接收 Provider 增量结果 → 客户端
	for result := range resultCh {
		if err := stream.Send(&pb.StreamResult{
			Text:        result.Text,
			Confidence:  float32(result.Confidence),
			IsFinal:     result.IsFinal,
			TimestampMs: result.TimestampMs,
		}); err != nil {
			return err
		}
	}
	outcome := <-outcomeCh
	if outcome.err != nil {
		return outcome.err
	}
	return nil
}

// RecognizeAndCorrect 语音识别 + 文本增强：一步输出标准企业语言。
func (s *ASRServiceService) RecognizeAndCorrect(ctx context.Context, req *pb.RecognizeRequest) (*pb.RecognizeAndCorrectResponse, error) {
	result, err := s.auc.Recognize(ctx, req)
	if err != nil {
		return nil, err
	}
	corrected, err := s.enhance(ctx, result.Text, req.GetProfileId())
	if err != nil {
		return nil, err
	}
	return &pb.RecognizeAndCorrectResponse{
		OriginalText:  result.Text,
		CorrectedText: corrected.GetCorrectedText(),
		Changes:       corrected.GetChanges(),
		Confidence:    corrected.GetConfidence(),
		ProviderName:  result.ProviderName,
	}, nil
}

// ReRecognize 对已有记录重新识别 + 文本增强。
func (s *ASRServiceService) ReRecognize(ctx context.Context, req *pb.ReRecognizeRequest) (*pb.RecognizeAndCorrectResponse, error) {
	result, err := s.auc.ReRecognize(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	corrected, err := s.enhance(ctx, result.Text, 0)
	if err != nil {
		return nil, err
	}
	return &pb.RecognizeAndCorrectResponse{
		OriginalText:  result.Text,
		CorrectedText: corrected.GetCorrectedText(),
		Changes:       corrected.GetChanges(),
		Confidence:    corrected.GetConfidence(),
		ProviderName:  result.ProviderName,
	}, nil
}

// enhance 调用文本增强引擎，映射为 CorrectResponse。
func (s *ASRServiceService) enhance(ctx context.Context, text string, profileID uint32) (*pb.CorrectResponse, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	policy, err := resolveEnhancePolicy(ctx, s.policyUc, profileID)
	if err != nil {
		return nil, err
	}
	result, err := s.enhancer.EnhanceWithPolicy(ctx, tenantID, text, policy)
	if err != nil {
		return nil, err
	}
	changes := make([]*pb.CorrectionChange, 0, len(result.Changes))
	needConfirm := false
	for _, ch := range result.Changes {
		changes = append(changes, &pb.CorrectionChange{
			From:       ch.OriginalText,
			To:         ch.ResultText,
			Type:       ch.Type,
			Confidence: float32(ch.Confidence),
		})
		if ch.Action == biz.ActionSuggest {
			needConfirm = true
		}
	}
	return &pb.CorrectResponse{
		OriginalText:  result.RawText,
		CorrectedText: result.EnhancedText,
		Changes:       changes,
		NeedConfirm:   needConfirm,
	}, nil
}

// ListAsrRecords 查询识别记录列表。
func (s *ASRServiceService) ListAsrRecords(ctx context.Context, req *pb.ListAsrRecordsRequest) (*pb.ListAsrRecordsResponse, error) {
	records, total, err := s.auc.ListRecords(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListAsrRecordsResponse{Records: records, Total: total}, nil
}

// GetAsrRecord 查询识别记录详情。
func (s *ASRServiceService) GetAsrRecord(ctx context.Context, req *pb.GetAsrRecordRequest) (*pb.AsrRecord, error) {
	return s.auc.GetRecord(ctx, req.GetId())
}

// GetAsrRecordAudio 获取识别记录音频内容。
func (s *ASRServiceService) GetAsrRecordAudio(ctx context.Context, req *pb.GetAsrRecordAudioRequest) (*pb.GetAsrRecordAudioResponse, error) {
	audio, contentType, err := s.auc.GetRecordAudio(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.GetAsrRecordAudioResponse{AudioData: audio, ContentType: contentType}, nil
}

func toPbSegments(segments []asr.Segment) []*pb.Segment {
	result := make([]*pb.Segment, 0, len(segments))
	for _, seg := range segments {
		result = append(result, &pb.Segment{
			StartMs:    seg.StartMs,
			EndMs:      seg.EndMs,
			Text:       seg.Text,
			Confidence: float32(seg.Confidence),
		})
	}
	return result
}
