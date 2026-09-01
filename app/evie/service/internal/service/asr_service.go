package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc/metadata"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	entviewer "backend-service/app/evie/service/internal/data/ent/viewer"
	"backend-service/pkg/asr"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/session"
)

// ASRServiceService ASR 语音识别 transport 适配层：所有 session_id 生成 + 增强 + 落库 都在 biz usecase 完成。
// service 不持有 data 层依赖，遵循 user 要求"service 只做 transport 适配"。
type ASRServiceService struct {
	pb.UnimplementedASRServiceServer
	uc            *biz.ASRUsecase
	euc           *biz.EnhancementUsecase
	authenticator *session.Manager
	log           *log.Helper
}

// NewASRServiceService 创建 ASR 服务实例。
func NewASRServiceService(uc *biz.ASRUsecase, euc *biz.EnhancementUsecase, authenticator *session.Manager, logger log.Logger) *ASRServiceService {
	return &ASRServiceService{uc: uc, euc: euc, authenticator: authenticator, log: log.NewHelper(logger)}
}

// Recognize 语音识别 + 文本增强（识别、增强、session_id、落库均在 biz ASRUsecase 内完成）。
func (s *ASRServiceService) Recognize(ctx context.Context, req *pb.RecognizeRequest) (*pb.RecognizeResponse, error) {
	return s.uc.Recognize(ctx, req)
}

// StreamRecognize 流式识别（双向流）：后端统一生成 session_id。
func (s *ASRServiceService) StreamRecognize(stream pb.ASRService_StreamRecognizeServer) error {
	ctx := stream.Context()

	first, err := stream.Recv()
	if err != nil {
		return err
	}

	audioCh := make(chan asr.PCMChunk, 16)
	resultCh := make(chan asr.ASRStreamResult, 16)

	outcomeCh := make(chan asrStreamOutcome, 1)
	go func() {
		recordID, audioURL, serr := s.uc.StreamRecognize(ctx, audioCh, resultCh)
		close(resultCh)
		outcomeCh <- asrStreamOutcome{recordID: recordID, audioURL: audioURL, err: serr}
	}()

	audioCh <- asr.PCMChunk{Data: first.GetData(), Timestamp: first.GetTimestampMs()}

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
	return outcome.err
}

// asrStreamOutcome 内部用：流式识别最终落库结果。
type asrStreamOutcome struct {
	recordID uint32
	audioURL string
	err      error
}

// ReRecognize 重新识别 + 增强（biz ASRUsecase 内部复用原 session_id 关联增强日志）。
func (s *ASRServiceService) ReRecognize(ctx context.Context, req *pb.ReRecognizeRequest) (*pb.RecognizeResponse, error) {
	return s.uc.ReRecognize(ctx, req.GetId())
}

// GetAsrRecordDetail 完整详情（含增强轨迹 + 步骤快照），从 enhancement_logs 取最近 log 拼接。
func (s *ASRServiceService) GetAsrRecordDetail(ctx context.Context, req *pb.GetAsrRecordRequest) (*pb.AsrRecordDetailResponse, error) {
	return s.uc.GetRecordDetail(ctx, req.GetId())
}

// ListAsrRecords 查询列表。
func (s *ASRServiceService) ListAsrRecords(ctx context.Context, req *pb.ListAsrRecordsRequest) (*pb.ListAsrRecordsResponse, error) {
	records, total, err := s.uc.ListRecords(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListAsrRecordsResponse{Records: records, Total: total}, nil
}

// GetAsrRecord 查询单条。
func (s *ASRServiceService) GetAsrRecord(ctx context.Context, req *pb.GetAsrRecordRequest) (*pb.AsrRecord, error) {
	return s.uc.GetRecord(ctx, req.GetId())
}

// GetAsrRecordAudio 获取音频。
func (s *ASRServiceService) GetAsrRecordAudio(ctx context.Context, req *pb.GetAsrRecordAudioRequest) (*pb.GetAsrRecordAudioResponse, error) {
	audio, contentType, err := s.uc.GetRecordAudio(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.GetAsrRecordAudioResponse{AudioData: audio, ContentType: contentType}, nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// streamMessage 客户端发送的音频分片消息。
type streamMessage struct {
	Audio   string `json:"audio"`    // base64 PCM 16kHz
	IsFinal bool   `json:"is_final"` // 是否最后一帧
}

// streamResult 服务端回传的增量结果。
type streamResult struct {
	Text     string `json:"text"`
	IsFinal  bool   `json:"is_final"`
	RecordID uint32 `json:"record_id,omitempty"` // 最终结果：识别记录 ID
	AudioURL string `json:"audio_url,omitempty"` // 最终结果：音频文件 ID
}

// streamOutcome 流式识别的业务结果（记录 ID + 音频文件 ID）。
type streamOutcome struct {
	recordID uint32
	audioURL string
	err      error
}

// ServeHTTP 处理 WebSocket 连接：鉴权 + 流式识别。
func (s *ASRServiceService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. 鉴权（JWT 从 query 参数提取）
	ctx := r.Context()
	tenantID, token, err := s.authenticate(ctx, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx = entviewer.NewTenantContext(ctx, tenantID)
	// WebSocket 用 query 传 token（非 Authorization 头），注入出站 metadata 供跨服务 gRPC 转发
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

	// 2. 升级 WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Errorf("upgrade websocket: %v", err)
		return
	}
	defer conn.Close()

	// 3. 流式识别
	audioCh := make(chan asr.PCMChunk, 32)
	resultCh := make(chan asr.ASRStreamResult, 32)

	outcomeCh := make(chan streamOutcome, 1)
	go func() {
		recordID, audioURL, serr := s.uc.StreamRecognize(ctx, audioCh, resultCh)
		close(resultCh)
		outcomeCh <- streamOutcome{recordID: recordID, audioURL: audioURL, err: serr}
	}()

	// 读取客户端音频分片 → audioCh
	go func() {
		defer close(audioCh)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m streamMessage
			if json.Unmarshal(msg, &m) != nil {
				continue
			}
			if m.IsFinal {
				return // 结束信号，关闭 audioCh
			}
			if m.Audio == "" {
				continue
			}
			pcm, err := base64.StdEncoding.DecodeString(m.Audio)
			if err != nil {
				continue
			}
			audioCh <- asr.PCMChunk{Data: pcm}
		}
	}()

	// 读取增量结果 → 客户端（provider 的 final 暂存，待合并 record 信息后回传）
	var finalText string
	for result := range resultCh {
		if result.Text != "" {
			finalText = result.Text
		}
		if result.IsFinal {
			continue
		}
		out, _ := json.Marshal(streamResult{Text: result.Text, IsFinal: false})
		if err := conn.WriteMessage(websocket.TextMessage, out); err != nil {
			return
		}
	}

	// 流结束：回传最终结果（附带记录 ID 与音频文件 ID）
	outcome := <-outcomeCh
	if outcome.err != nil {
		s.log.Warnf("stream recognize: %v", outcome.err)
	}
	final, _ := json.Marshal(streamResult{
		Text:     finalText,
		IsFinal:  true,
		RecordID: outcome.recordID,
		AudioURL: outcome.audioURL,
	})
	if err := conn.WriteMessage(websocket.TextMessage, final); err != nil {
		return
	}
}

// authenticate 校验 query 中的 JWT，返回 tenant ID 与原始 token。
func (s *ASRServiceService) authenticate(ctx context.Context, r *http.Request) (uint32, string, error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		return 0, "", authn.NewAuthError(authn.ErrCodeMissingToken, "missing token", nil)
	}
	claims, err := s.authenticator.ValidateToken(ctx, token)
	if err != nil {
		return 0, "", err
	}
	tenantID, _ := strconv.ParseUint(claims.GetTenant(), 10, 32)
	if tenantID == 0 {
		return 0, "", authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid tenant", nil)
	}
	return uint32(tenantID), token, nil
}
