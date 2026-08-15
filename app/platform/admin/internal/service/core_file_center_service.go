package service

import (
	"context"

	corepb "backend-service/api/core/service/v1"
)

// CoreFileCenterService 为产品服务（evie 等）提供跨服务的文件中心委托。
// 它实现 core.service.v1.FileCenterService（gRPC-only），供 pkg/filecenter/grpc
// 客户端调用，复用平台文件中心的上传/下载能力。认证由跨服务 JWT 转发中间件完成。
type CoreFileCenterService struct {
	corepb.UnimplementedFileCenterServiceServer
	inner *FileCenterServiceService
}

// NewCoreFileCenterService 创建核心文件中心服务实例。
func NewCoreFileCenterService(inner *FileCenterServiceService) *CoreFileCenterService {
	return &CoreFileCenterService{inner: inner}
}

// CreateFileUploadSession 创建上传会话。
func (s *CoreFileCenterService) CreateFileUploadSession(ctx context.Context, req *corepb.CreateFileUploadSessionRequest) (*corepb.CreateFileUploadSessionResponse, error) {
	return s.inner.CreateFileUploadSession(ctx, req)
}

// UploadFileContent 上传文件内容。
func (s *CoreFileCenterService) UploadFileContent(ctx context.Context, req *corepb.UploadFileContentRequest) (*corepb.UploadFileContentResponse, error) {
	return s.inner.UploadFileContent(ctx, req)
}

// ConfirmFileUpload 确认上传。
func (s *CoreFileCenterService) ConfirmFileUpload(ctx context.Context, req *corepb.ConfirmFileUploadRequest) (*corepb.ConfirmFileUploadResponse, error) {
	return s.inner.ConfirmFileUpload(ctx, req)
}

// PresignFileDownload 预签名下载。
func (s *CoreFileCenterService) PresignFileDownload(ctx context.Context, req *corepb.PresignFileDownloadRequest) (*corepb.PresignFileDownloadResponse, error) {
	return s.inner.PresignFileDownload(ctx, req)
}

// DownloadFileContent 代理下载文件内容（local 渠道）。
func (s *CoreFileCenterService) DownloadFileContent(ctx context.Context, req *corepb.DownloadFileContentRequest) (*corepb.DownloadFileContentResponse, error) {
	return s.inner.DownloadFileContent(ctx, req)
}
