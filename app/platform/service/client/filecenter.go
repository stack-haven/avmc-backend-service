// Package client 提供 Ark Platform 服务的 gRPC 客户端 SDK，供产品服务（evie 等）复用。
// 通过 gRPC metadata 转发原始 JWT（Authorization），实现跨服务认证与租户传递。
package client

import (
	"context"
	"fmt"

	corepb "backend-service/api/platform/service/v1"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// FileCenterClient 封装文件中心的上传/下载，供产品服务复用。
type FileCenterClient struct {
	conn       *grpc.ClientConn
	fileClient corepb.FileCenterServiceClient
}

// NewFileCenterClient 创建文件中心客户端，连接平台 gRPC 服务。
func NewFileCenterClient(ctx context.Context, endpoint string) (*FileCenterClient, error) {
	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(forwardAuthToken),
	)
	if err != nil {
		return nil, err
	}
	return &FileCenterClient{
		conn:       conn,
		fileClient: corepb.NewFileCenterServiceClient(conn),
	}, nil
}

// forwardAuthToken 将当前请求的 Authorization 头（含 "Bearer " 前缀）原样转发到出站 gRPC metadata，
// 实现跨服务的 JWT 认证与租户传递。
func forwardAuthToken(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	ctx = authn.ForwardAuthToken(ctx)
	return invoker(ctx, method, req, reply, cc, opts...)
}

// Upload 完整上传：创建会话 → 上传内容 → 确认，返回文件对象。
func (c *FileCenterClient) Upload(ctx context.Context, req *corepb.CreateFileUploadSessionRequest, content []byte) (*corepb.FileObject, error) {
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
func (c *FileCenterClient) PresignDownload(ctx context.Context, id uint32) (string, error) {
	resp, err := c.fileClient.PresignFileDownload(ctx, &corepb.PresignFileDownloadRequest{Id: id})
	if err != nil {
		return "", fmt.Errorf("presign download: %w", err)
	}
	return resp.GetDownloadUrl(), nil
}

// DownloadContent 通过文件中心代理下载文件内容（适用于 local 等不支持预签名的渠道）。
func (c *FileCenterClient) DownloadContent(ctx context.Context, id uint32) ([]byte, string, error) {
	resp, err := c.fileClient.DownloadFileContent(ctx, &corepb.DownloadFileContentRequest{Id: id})
	if err != nil {
		return nil, "", fmt.Errorf("download content: %w", err)
	}
	return resp.GetContent(), resp.GetContentType(), nil
}

// Close 关闭 gRPC 连接。
func (c *FileCenterClient) Close() error { return c.conn.Close() }
