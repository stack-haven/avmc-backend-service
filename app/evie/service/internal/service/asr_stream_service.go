package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc/metadata"

	"backend-service/app/evie/service/internal/biz"
	entviewer "backend-service/app/evie/service/internal/data/ent/viewer"
	"backend-service/pkg/asr"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/session"
)

// ASRStreamService 流式语音识别服务（WebSocket 传输层）。
// 负责：JWT 鉴权 → WebSocket 升级 → 音频转发 → 增量结果回传。
type ASRStreamService struct {
	auc           *biz.ASRUsecase
	authenticator *session.Manager
	log           *log.Helper
}

// NewASRStreamService 创建流式识别服务实例。
func NewASRStreamService(auc *biz.ASRUsecase, authenticator *session.Manager, logger log.Logger) *ASRStreamService {
	return &ASRStreamService{auc: auc, authenticator: authenticator, log: log.NewHelper(logger)}
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
func (s *ASRStreamService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	sessionID := r.URL.Query().Get("session_id")
	audioCh := make(chan asr.PCMChunk, 32)
	resultCh := make(chan asr.ASRStreamResult, 32)

	outcomeCh := make(chan streamOutcome, 1)
	go func() {
		recordID, audioURL, serr := s.auc.StreamRecognize(ctx, sessionID, audioCh, resultCh)
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
func (s *ASRStreamService) authenticate(ctx context.Context, r *http.Request) (uint32, string, error) {
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
