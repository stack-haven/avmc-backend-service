package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/errors"

	pb "backend-service/api/ai/service/v1"
	"backend-service/app/ai/service/internal/biz"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/session"
	"backend-service/pkg/kratos/transport/sse"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"go.einride.tech/aip/pagination"
)

// ChatServiceService 对话服务结构体
// 包含业务用例和日志记录器
type ChatServiceService struct {
	pb.UnimplementedChatServiceServer
	ucc           *biz.ChatUsecase
	sse           *sse.Server
	authenticator *session.Manager
	log           *log.Helper
	wsUpgrader    websocket.Upgrader
	router        *mux.Router
}

// RegisterRoutes 注册路由
func (s *ChatServiceService) RegisterRoutes() *mux.Router {
	s.router.HandleFunc("/ws/chat", s.WebsocketHandler)
	s.router.HandleFunc("/sse/chat", s.SSEHandler)
	return s.router
}

// NewChatServiceService 创建新的对话服务实例
func NewChatServiceService(ucc *biz.ChatUsecase, authenticator *session.Manager, logger log.Logger) *ChatServiceService {
	return &ChatServiceService{
		ucc:           ucc,
		authenticator: authenticator,
		log:           log.NewHelper(logger),
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// 关键：允许跨域
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		router: mux.NewRouter(),
	}
}

// SetSSEServer 设置 SSE 服务器
func (s *ChatServiceService) SetSSEServer(srv *sse.Server) {
	s.sse = srv
}

// PublishSSEData 根据业务类型发布 SSE 数据
func (s *ChatServiceService) PublishSSEData(businessType string, streamID string, data map[string]string) error {
	// 根据业务类型处理不同的逻辑
	switch businessType {
	case "chat":
		// 聊天业务逻辑
		s.log.Infof("Publishing chat SSE data to stream: %s", streamID)
	case "ai":
		// AI 业务逻辑
		s.log.Infof("Publishing AI SSE data to stream: %s", streamID)
	default:
		s.log.Warnf("Unknown business type: %s", businessType)
	}

	// 发布数据
	return s.sse.PublishData(context.Background(), sse.StreamID(streamID), data)
}

// ListChatSimple 处理对话简单列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：对话列表响应，错误信息
func (s *ChatServiceService) ListChatsSimple(ctx context.Context, req *pb.ListChatsRequest) (*pb.ListChatsResponse, error) {
	s.log.Infof("查询对话简单列表分页，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return nil, err
	}

	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, err
	}
	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return nil, err
	}
	count, err := s.ucc.CountChats(ctx, biz.ListFilter(filter))
	if err != nil {
		return nil, err
	}
	resp := pb.ListChatsResponse{
		Total: count,
	}
	resp.Items, err = s.ucc.ListPageSimple(ctx,
		biz.ListFilter(filter),
		biz.ListOrderBy(orderBy),
		biz.ListLimit(int(req.PageSize)),
		biz.ListOffset(int(pageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= int(req.PageSize) {
		resp.NextPageToken = pageToken.Next(req).String()
	}
	return &resp, nil
}

// GetChat 处理获取对话详情请求
// 参数：ctx 上下文，req 获取对话请求
// 返回值：对话详情响应，错误信息
func (s *ChatServiceService) GetChat(ctx context.Context, req *pb.GetChatRequest) (*pb.Chat, error) {
	if req.GetId() == 0 {
		return nil, errors.New(1001, "对话ID不能为空", "chat id is required")
	}
	s.log.Infof("获取对话详情，对话ID：%v", req.GetId())
	return s.ucc.Get(ctx, req.GetId())
}

// CreateChat 处理创建对话请求
// 参数：ctx 上下文，req 创建对话请求
// 返回值：创建对话响应，错误信息
func (s *ChatServiceService) CreateChat(ctx context.Context, req *pb.CreateChatRequest) (*pb.CreateChatResponse, error) {
	if req.GetChat() == nil {
		return nil, pb.ErrorChatInvalidId("对话信息不能为空")
	}
	s.log.Infof("创建对话，对话名称：%s", req.GetChat().GetName())
	_, err := s.ucc.Create(ctx, req.Chat)
	if err != nil {
		return nil, err
	}
	// s.wsUpgrader.
	return &pb.CreateChatResponse{}, nil
}

// UpdateChat 处理更新对话请求
// 参数：ctx 上下文，req 更新对话请求
// 返回值：更新对话响应，错误信息
func (s *ChatServiceService) UpdateChat(ctx context.Context, req *pb.UpdateChatRequest) (*pb.UpdateChatResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorChatInvalidId("对话ID不能为空")
	}
	if req.GetChat() == nil {
		return nil, pb.ErrorChatInvalidId("对话信息不能为空")
	}
	if req.GetOperatorId() == 0 {
		// return nil, pb.ErrorChatInvalidOperatorId("操作人ID不能为空")
	}
	existing, err := s.GetChat(ctx, &pb.GetChatRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	fieldmask.Update(req.UpdateMask, existing, req.Chat)
	s.log.Infof("更新对话，对话ID：%v", req.GetId())
	existing.Id = req.GetId()
	_, err = s.ucc.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateChatResponse{}, nil
}

// DeleteChat 处理删除对话请求
// 参数：ctx 上下文，req 删除对话请求
// 返回值：删除对话响应，错误信息
func (s *ChatServiceService) DeleteChat(ctx context.Context, req *pb.DeleteChatRequest) (*pb.DeleteChatResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorChatInvalidId("对话ID不能为空")
	}
	s.log.Infof("删除对话，对话ID：%v", req.GetId())
	err := s.ucc.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.DeleteChatResponse{}, nil
}

// UpdateChatByStatus 处理更新对话状态请求
// 参数：ctx 上下文，req 更新对话状态请求
// 返回值：更新对话状态响应，错误信息
func (s *ChatServiceService) UpdateChatByStatus(ctx context.Context, req *pb.UpdateChatByStatusRequest) (*pb.UpdateChatByStatusResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorChatInvalidId("对话ID不能为空")
	}
	if req.GetStatus() == 0 {
		return nil, pb.ErrorChatStatusCannotBeEmpty("对话状态不能为空")
	}
	s.log.Infof("更新对话状态，对话ID：%v，对话状态：%v", req.GetId(), req.GetStatus())
	_, err := s.ucc.UpdateStatus(ctx, req.GetId(), req.GetStatus())
	if err != nil {
		return nil, err
	}
	return &pb.UpdateChatByStatusResponse{}, nil
}

// ListChat 处理对话列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：对话列表响应，错误信息
func (s *ChatServiceService) ListChats(ctx context.Context, req *pb.ListChatsRequest) (*pb.ListChatsResponse, error) {
	s.log.Infof("查询对话列表分页，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("email", filtering.TypeString),
		filtering.DeclareIdent("phone", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return nil, err
	}

	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, err
	}
	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return nil, err
	}
	count, err := s.ucc.CountChats(ctx, biz.ListFilter(filter))
	if err != nil {
		return nil, err
	}
	resp := pb.ListChatsResponse{
		Total: count,
	}
	resp.Items, err = s.ucc.ListChats(ctx,
		biz.ListFilter(filter),
		biz.ListOrderBy(orderBy),
		biz.ListLimit(int(req.PageSize)),
		biz.ListOffset(int(pageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= int(req.PageSize) {
		resp.NextPageToken = pageToken.Next(req).String()
	}
	return &resp, nil
}

// StreamChat 处理 AI 聊天的流式响应
// 参数：ctx 上下文，req 聊天请求
// 返回值：流式聊天响应，错误信息
func (s *ChatServiceService) StreamChat(ctx context.Context, req *pb.StreamChatRequest) (*pb.StreamChatResponse, error) {
	if req.GetMessage() == "" {
		return nil, errors.New(1001, "消息内容不能为空", "message content is required")
	}
	s.log.Infof("处理 AI 聊天流式请求，chat_id=%d", req.GetChatId())
	// 生成唯一的流ID
	streamID := sse.StreamID(fmt.Sprintf("chat_%d_%d", req.GetChatId(), time.Now().UnixNano()))
	// 异步处理AI响应并通过SSE发送
	go func() {
		// 模拟AI响应
		responses := []string{
			"你好！我是AI助手。",
			"我可以帮你做什么？",
			"请告诉我你的需求。",
		}
		for _, response := range responses {
			time.Sleep(1 * time.Second)
			// 通过SSE发送数据
			s.sse.PublishData(context.Background(), streamID, map[string]string{
				"message": response,
				"chat_id": fmt.Sprintf("%d", req.GetChatId()),
			})
		}
	}()
	return &pb.StreamChatResponse{StreamId: string(streamID)}, nil
}

// authenticateRequest 显式验证 HTTP 请求中的 Token（用于绕过 Kratos 中间件的 SSE/WS 端点）
func (s *ChatServiceService) authenticateRequest(r *http.Request) (context.Context, error) {
	token := r.Header.Get("Authorization")
	if token == "" {
		// 也尝试从 query 参数获取
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		return nil, errors.New(401, "UNAUTHORIZED", "missing token")
	}

	// 支持 Bearer scheme
	const bearerPrefix = "Bearer "
	if len(token) > len(bearerPrefix) && token[:len(bearerPrefix)] == bearerPrefix {
		token = token[len(bearerPrefix):]
	}

	claims, err := s.authenticator.ValidateToken(r.Context(), token)
	if err != nil {
		return nil, errors.New(401, "UNAUTHORIZED", "invalid token")
	}

	ctx := authn.ContextWithAuthClaims(r.Context(), claims)
	return ctx, nil
}

// WebsocketHandler 处理 WebSocket 连接
func (s *ChatServiceService) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := s.authenticateRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	uid := authn.GetAuthUserID(ctx)
	s.log.Infof("websocket authenticated user: %d", uid)
	c, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Infof("upgrade:", err)
		return
	}
	defer c.Close()
	// 1. 发送欢迎消息 (发送字符串)
	c.WriteMessage(websocket.TextMessage, []byte("连接已建立，你可以开始提问了"))
	type Response struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	c.WriteJSON(Response{Type: "status", Text: "AI 正在思考..."})
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			s.log.Error("read:", err)
			break
		}
		s.log.Infof("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			s.log.Errorf("write:", err)
			break
		}
	}
}

// SSEHandler 处理 SSE 连接
func (s *ChatServiceService) SSEHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := s.authenticateRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")          // 禁用 Nginx 缓存
	w.Header().Set("Access-Control-Allow-Origin", "*") // 视情况开启跨域
	uid := authn.GetAuthUserID(ctx)
	s.log.Infof("sse authenticated user: %d", uid)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	// 2. 使用专门的定时器或 Channel
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			s.log.Info("SSE Client disconnected")
			return
		case t := <-ticker.C:
			// 这里替换为你的业务逻辑，比如从 biz 层获取 AI Token
			_, err := fmt.Fprintf(w, "data: %s\n\n", t.Format(time.RFC3339))
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
