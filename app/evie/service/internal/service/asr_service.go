package service

import (
	"context"
	"encoding/json"
	"io"
	"time"

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
	logUc    *biz.EnhancementLogUsecase
	log      *log.Helper
}

// NewASRServiceService 创建 ASR 服务实例。
func NewASRServiceService(auc *biz.ASRUsecase, enhancer *biz.EnhancementEngine, policyUc *biz.EnhancementPolicyUsecase, logUc *biz.EnhancementLogUsecase, logger log.Logger) *ASRServiceService {
	return &ASRServiceService{auc: auc, enhancer: enhancer, policyUc: policyUc, logUc: logUc, log: log.NewHelper(logger)}
}

// Recognize 语音识别 + 文本增强（profile_id 决定策略）。
// 增强策略统一通过场景 Profile 关联：profile_id>0 时按场景关联策略；=0 时按租户默认。
func (s *ASRServiceService) Recognize(ctx context.Context, req *pb.RecognizeRequest) (*pb.RecognizeResponse, error) {
	result, err := s.auc.Recognize(ctx, req)
	if err != nil {
		return nil, err
	}

	return s.buildEnhancedResponse(ctx, result, req.GetProfileId(), req.GetSessionId())
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

// ReRecognize 对已有记录重新识别（纯 ASR，不增强）。
// 如需增强请调用 Recognize。
func (s *ASRServiceService) ReRecognize(ctx context.Context, req *pb.ReRecognizeRequest) (*pb.RecognizeResponse, error) {
	result, err := s.auc.ReRecognize(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	return s.buildEnhancedResponse(ctx, result, req.GetProfileId(), result.RequestID)

}

// buildEnhancedResponse 构造 RecognizeResponse 并落增强日志。
// profileID=0 时按租户默认策略；无策略返回 corrected==original（changes=空）。
func (s *ASRServiceService) buildEnhancedResponse(ctx context.Context, result *asr.ASRResult, profileID uint32, sessionID string) (*pb.RecognizeResponse, error) {
	corrected, err := s.enhance(ctx, result.Text, profileID, sessionID)
	if err != nil {
		return nil, err
	}
	if corrected == nil {
		return &pb.RecognizeResponse{
			RawText:      result.Text,
			Changes:      nil,
			Confidence:   float32(result.Confidence),
			ProviderName: result.ProviderName,
		}, nil
	}
	return &pb.RecognizeResponse{
		RawText:      result.Text,
		EnhancedText: corrected.EnhancedText,
		Changes:      corrected.GetChanges(),
		Confidence:   float32(0),
		ProviderName: result.ProviderName,
	}, nil
}

// enhance 调用文本增强引擎，映射为 CorrectResponse。
// policyID > 0 时按指定策略增强；profileID > 0 时按场景增强（场景关联策略）。
// enhance 调用文本增强引擎，映射为 CorrectResponse。
// 增强策略统一通过 profile 路径：profileID>0 时按场景关联策略；profileID=0 时按租户默认。
// 找不到任何策略时返回 nil，由 buildRecognizeResponse 退化为只返回原文。
func (s *ASRServiceService) enhance(ctx context.Context, text string, profileID uint32, sessionID string) (*pb.EnhanceTextResponse, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	policy, err := resolveEnhancePolicy(ctx, s.policyUc, profileID)
	if err != nil {
		return nil, err
	}
	result, err := s.enhancer.EnhanceWithPolicy(ctx, tenantID, text, policy)
	if err != nil {
		return nil, err
	}
	changes := make([]*pb.EnhanceChange, 0, len(result.Changes))
	for _, ch := range result.Changes {
		changes = append(changes, &pb.EnhanceChange{
			From:       ch.OriginalText,
			To:         ch.ResultText,
			Type:       ch.Type,
			Confidence: float32(ch.Confidence),
		})
	}

	// 保存增强日志（含步骤快照，供记录详情步骤图展示）
	if s.logUc != nil && result.StepTimings != nil {
		changesJSON, _ := json.Marshal(result.Changes)
		snapshotsJSON, _ := json.Marshal(result.StepSnapshots)
		err = s.logUc.Save(ctx, &biz.EnhancementLogData{
			SessionID:           sessionID,
			RawText:             result.RawText,
			EnhancedText:        result.EnhancedText,
			ChangesJSON:         string(changesJSON),
			StepSnapshotsJSON:   string(snapshotsJSON),
			ProcessingTimeMs:    result.TotalTime.Milliseconds(),
			CleaningTimeMs:      stepMs(result.StepTimings, "cleaning"),
			FillerTimeMs:        stepMs(result.StepTimings, "filler"),
			VocabMatchTimeMs:    stepMs(result.StepTimings, "vocabulary_matching"),
			AliasTimeMs:         stepMs(result.StepTimings, "alias_resolution"),
			DeterministicTimeMs: stepMs(result.StepTimings, "deterministic_replacement"),
			PinyinTimeMs:        stepMs(result.StepTimings, "pinyin_correction"),
			FuzzyTimeMs:         stepMs(result.StepTimings, "fuzzy_matching"),
			ContextTimeMs:       stepMs(result.StepTimings, "context_correction"),
			Status:              1,
		})
		if err != nil {
			s.log.Errorf("failed to save enhancement log: %v", err)
		}
	}

	return &pb.EnhanceTextResponse{
		OriginalText: result.RawText,
		EnhancedText: result.EnhancedText,
		Changes:      changes,
		Status:       1,
	}, nil
}

// GetAsrRecordDetail 查询识别记录完整详情（音频 + 原始/增强文本 + 增强步骤快照）。
func (s *ASRServiceService) GetAsrRecordDetail(ctx context.Context, req *pb.GetAsrRecordRequest) (*pb.AsrRecordDetail, error) {
	record, err := s.auc.GetRecord(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	detail := &pb.AsrRecordDetail{Record: record}
	if s.logUc != nil {
		logs, _, err := s.logUc.List(ctx, &pb.ListEnhancementLogsRequest{SessionId: record.GetSessionId(), PageSize: 1})
		if err == nil && len(logs) > 0 {
			log := logs[0]
			detail.EnhancedText = log.GetEnhancedText()
			detail.PolicyName = log.GetPolicyMode()
			if log.GetStepSnapshotsJson() != "" {
				var snapshots []*biz.StepSnapshot
				if err := json.Unmarshal([]byte(log.GetStepSnapshotsJson()), &snapshots); err == nil {
					for _, snap := range snapshots {
						pbSnap := &pb.EnhancementStepSnapshot{
							Step: snap.Step, Before: snap.Before, After: snap.After,
							DurationMs: snap.DurationMs, Skipped: snap.Skipped,
						}
						for _, ch := range snap.Changes {
							pbSnap.Changes = append(pbSnap.Changes, &pb.EnhanceChange{
								From: ch.OriginalText, To: ch.ResultText,
								Type: ch.Type, Confidence: float32(ch.Confidence),
							})
						}
						detail.StepSnapshots = append(detail.StepSnapshots, pbSnap)
					}
				}
			}
			if log.GetChangesJson() != "" {
				var changes []*biz.EnhancementChange
				if err := json.Unmarshal([]byte(log.GetChangesJson()), &changes); err == nil {
					for _, ch := range changes {
						detail.Changes = append(detail.Changes, &pb.EnhanceChange{
							From: ch.OriginalText, To: ch.ResultText,
							Type: ch.Type, Confidence: float32(ch.Confidence),
						})
					}
				}
			}
		}
	}
	return detail, nil
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

// stepMs 从 stepTimings map 中取步骤耗时，找不到返回 0。
func stepMs(m map[string]time.Duration, key string) int64 {
	if d, ok := m[key]; ok {
		return d.Milliseconds()
	}
	return 0
}

// toPbSegments 死代码已删除（Segment 类型已从 proto 中移除；如需按时间片展示识别片段可临时在调用处构造）
