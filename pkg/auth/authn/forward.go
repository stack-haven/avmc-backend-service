package authn

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc/metadata"
)

// ForwardAuthToken 将当前请求的 Authorization 头（含 "Bearer " 前缀）原样转发到
// 出站 gRPC metadata（key 为小写 "authorization"），实现跨服务的 JWT 认证与租户传递。
//
// 约定：与服务端 ParseContextToken(HeaderAuthorize, BearerWord) 保持同一解析链路，
// 不做 split、不拆 token、不改变大小写，原样透传 "Bearer <token>"。
// kratos gRPC transport 的 headerCarrier 即 metadata.MD，Get 大小写不敏感，
// 服务端仍可用 Get(HeaderAuthorize) 取到。
//
// 注意：仅适用于同步调用（ctx 仍是 server context）。异步场景（如操作审计中间件
// 用 context.Background() 启动 goroutine）请先用 ForwardAuthHeader 在同步阶段显式
// 提取 header 后再注入。
func ForwardAuthToken(ctx context.Context) context.Context {
	if tr, ok := transport.FromServerContext(ctx); ok {
		if auth := tr.RequestHeader().Get(HeaderAuthorize); auth != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", auth)
		}
	}
	return ctx
}

// ForwardAuthHeader 将调用方在同步阶段显式提取的 Authorization 头（含 "Bearer " 前缀）
// 转发到出站 gRPC metadata。适用于 ctx 已不是 server context 的异步场景（如操作审计），
// 此时 ForwardAuthToken 无法从 server context 提取 header。
func ForwardAuthHeader(ctx context.Context, authHeader string) context.Context {
	if authHeader != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authHeader)
	}
	return ctx
}
