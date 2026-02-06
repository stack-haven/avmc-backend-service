package server

import (
	"backend-service/app/avmc/ai/internal/conf"
	"backend-service/app/avmc/ai/internal/service"
	"backend-service/pkg/kratos/transport/sse"

	"github.com/go-kratos/kratos/v2/log"
)

// NewSSEServer creates a new SSE server.
func NewSSEServer(
	c *conf.Server,
	logger log.Logger,
	chat *service.ChatServiceService,
) *sse.Server {
	// 使用 HTTP 配置作为默认值
	var opts = []sse.ServerOption{
		sse.WithLogger(logger),
	}
	if c.Sse.Network != "" {
		opts = append(opts, sse.WithNetwork(c.Sse.Network))
	}
	if c.Sse.Addr != "" {
		opts = append(opts, sse.WithAddress(c.Sse.Addr))
	}
	if c.Sse.Path != "" {
		opts = append(opts, sse.WithPath(c.Sse.Path))
	}
	if c.Sse.Timeout != nil {
		opts = append(opts, sse.WithTimeout(c.Sse.Timeout.AsDuration()))
	}
	// 设置 SSE 服务器
	srv := sse.NewServer(opts...)

	// 注册多个 SSE 路径，支持不同的业务接口
	// 1. 聊天接口
	// chatPath := "/api/v1/chat/stream"
	// if c.Sse.Path != "" {
	// 	chatPath = c.Sse.Path
	// }
	// srv.RegisterSSEPath(chatPath, "chat")

	// // 2. 其他业务接口（示例）
	// srv.RegisterSSEPath("/api/v1/ai/stream", "ai")

	// 设置对话服务的 SSE 服务器
	chat.SetSSEServer(srv)
	return srv
}
