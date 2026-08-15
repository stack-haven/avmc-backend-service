// Package grpc 提供文件中心的 gRPC 客户端，供产品服务（evie 等）复用平台文件中心。
// 通过 gRPC metadata 转发原始 JWT（Authorization），实现跨服务认证与租户传递。
package grpc

import (
	"context"
	"fmt"

	corepb "backend-service/api/core/service/v1"
	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Client 封装文件中心的上传/下载，供产品服务复用。
type Client struct {
	conn       *grpc.ClientConn
	fileClient corepb.FileCenterServiceClient
}

// New 创建文件中心客户端，连接平台 gRPC 服务。
func New(ctx context.Context, endpoint string) (*Client, error) {
	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(forwardAuthToken),
	)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:       conn,
		fileClient: corepb.NewFileCenterServiceClient(conn),
	}, nil
}

// forwardAuthToken 将当前请求的 Authorization 头转发到出站 gRPC metadata，
// 实现跨服务的 JWT 认证与租户传递。
func forwardAuthToken(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if header, ok := transport.FromServerContext(ctx); ok {
		if auth := header.RequestHeader().Get("Authorization"); auth != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", auth)
		}
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

// Upload 完整上传：创建会话 → 上传内容 → 确认，返回文件对象。
func (c *Client) Upload(ctx context.Context, req *corepb.CreateFileUploadSessionRequest, content []byte) (*corepb.FileObject, error) {
	session, err := c.fileClient.CreateFileUploadSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create upload session: %w", err)
	}
	file := session.GetFile()
	if file == nil || file.GetId() == 0 {
		return nil, fmt.Errorf("create upload session: missing file id")
	}
	if _, err := c.fileClient.UploadFileContent(ctx, &corepb.UploadFileContentRequest{
		Id:          file.GetId(),
		Content:     content,
		ContentType: req.ContentType,
	}); err != nil {
		return nil, fmt.Errorf("upload file content: %w", err)
	}
	confirmed, err := c.fileClient.ConfirmFileUpload(ctx, &corepb.ConfirmFileUploadRequest{
		Id:   file.GetId(),
		Size: convert.ToPointer(int64(len(content))),
	})
	if err != nil {
		return nil, fmt.Errorf("confirm file upload: %w", err)
	}
	return confirmed.GetFile(), nil
}

// PresignDownload 获取文件的预签名下载 URL。
func (c *Client) PresignDownload(ctx context.Context, id uint32) (string, error) {
	resp, err := c.fileClient.PresignFileDownload(ctx, &corepb.PresignFileDownloadRequest{Id: id})
	if err != nil {
		return "", fmt.Errorf("presign download: %w", err)
	}
	return resp.GetDownloadUrl(), nil
}

// DownloadContent 通过文件中心代理下载文件内容（适用于 local 等不支持预签名的渠道）。
func (c *Client) DownloadContent(ctx context.Context, id uint32) ([]byte, string, error) {
	resp, err := c.fileClient.DownloadFileContent(ctx, &corepb.DownloadFileContentRequest{Id: id})
	if err != nil {
		return nil, "", fmt.Errorf("download content: %w", err)
	}
	return resp.GetContent(), resp.GetContentType(), nil
}

// Close 关闭 gRPC 连接。
func (c *Client) Close() error { return c.conn.Close() }
