package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"
)

type FileCenterServiceService struct {
	pb.UnimplementedFileCenterServiceServer
	uc  *biz.FileUsecase
	log *log.Helper
}

func NewFileCenterServiceService(uc *biz.FileUsecase, logger log.Logger) *FileCenterServiceService {
	return &FileCenterServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *FileCenterServiceService) CreateFileUploadSession(ctx context.Context, req *pbCore.CreateFileUploadSessionRequest) (*pbCore.CreateFileUploadSessionResponse, error) {
	return s.uc.CreateUploadSession(ctx, req)
}

func (s *FileCenterServiceService) UploadFileContent(ctx context.Context, req *pbCore.UploadFileContentRequest) (*pbCore.UploadFileContentResponse, error) {
	file, err := s.uc.UploadContent(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.UploadFileContentResponse{File: file}, nil
}

func (s *FileCenterServiceService) ConfirmFileUpload(ctx context.Context, req *pbCore.ConfirmFileUploadRequest) (*pbCore.ConfirmFileUploadResponse, error) {
	file, err := s.uc.ConfirmUpload(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.ConfirmFileUploadResponse{File: file}, nil
}

func (s *FileCenterServiceService) GetFileObject(ctx context.Context, req *pbCore.GetFileObjectRequest) (*pbCore.FileObject, error) {
	return s.uc.Get(ctx, req.GetId())
}

func (s *FileCenterServiceService) ListFileObjects(ctx context.Context, req *pbCore.ListFileObjectsRequest) (*pbCore.ListFileObjectsResponse, error) {
	params, err := listing.ParseParams(
		req,
		filtering.DeclareIdent("file_name", filtering.TypeString),
		filtering.DeclareIdent("business_type", filtering.TypeString),
		filtering.DeclareIdent("business_id", filtering.TypeString),
		filtering.DeclareIdent("status", filtering.TypeInt),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	req.PageSize = int32(params.PageSize)
	count, err := s.uc.Count(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListFileObjectsResponse{Total: count}
	resp.Items, err = s.uc.List(ctx,
		listing.FilterOption(params.Filter),
		listing.OrderByOption(params.OrderBy),
		listing.LimitOption(params.PageSize),
		listing.OffsetOption(int(params.PageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= params.PageSize {
		resp.NextPageToken = params.PageToken.Next(req).String()
	}
	return resp, nil
}

func (s *FileCenterServiceService) ListFileAccessLogs(ctx context.Context, req *pbCore.ListFileAccessLogsRequest) (*pbCore.ListFileAccessLogsResponse, error) {
	// file_id 是路径参数，作为必填筛选条件合并到 AIP filter 字符串，
	// 遵循 listing.Option 规范：所有查询条件统一通过 filter 传递。
	if req.GetFileId() > 0 {
		req.Filter = mergeFileIDFilter(req.Filter, req.GetFileId())
	}
	params, err := listing.ParseParams(
		req,
		filtering.DeclareIdent("file_id", filtering.TypeInt),
		filtering.DeclareIdent("action", filtering.TypeString),
		filtering.DeclareIdent("result", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	req.PageSize = int32(params.PageSize)
	count, err := s.uc.CountAccessLogs(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListFileAccessLogsResponse{Total: count}
	resp.Items, err = s.uc.ListAccessLogs(ctx,
		listing.FilterOption(params.Filter),
		listing.OrderByOption(params.OrderBy),
		listing.LimitOption(params.PageSize),
		listing.OffsetOption(int(params.PageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= params.PageSize {
		resp.NextPageToken = params.PageToken.Next(req).String()
	}
	return resp, nil
}

// mergeFileIDFilter 把 file_id 条件合并到已有 filter 字符串（AND 组合）。
func mergeFileIDFilter(existing *string, fileID uint32) *string {
	cond := fmt.Sprintf("file_id = %d", fileID)
	if existing != nil && strings.TrimSpace(*existing) != "" {
		merged := fmt.Sprintf("(%s) AND %s", strings.TrimSpace(*existing), cond)
		return &merged
	}
	return &cond
}

func (s *FileCenterServiceService) PresignFileDownload(ctx context.Context, req *pbCore.PresignFileDownloadRequest) (*pbCore.PresignFileDownloadResponse, error) {
	return s.uc.PresignDownload(ctx, req)
}

func (s *FileCenterServiceService) DeleteFileObject(ctx context.Context, req *pbCore.DeleteFileObjectRequest) (*pbCore.DeleteFileObjectResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId(), req.GetIdempotencyKey()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteFileObjectResponse{}, nil
}

func (s *FileCenterServiceService) UpdateFileObject(ctx context.Context, req *pbCore.UpdateFileObjectRequest) (*pbCore.UpdateFileObjectResponse, error) {
	file, err := s.uc.UpdateFileName(ctx, req.GetId(), req.GetFileName())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateFileObjectResponse{File: file}, nil
}

func (s *FileCenterServiceService) ReplaceFileContent(ctx context.Context, req *pbCore.ReplaceFileContentRequest) (*pbCore.ReplaceFileContentResponse, error) {
	file, err := s.uc.ReplaceContent(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.ReplaceFileContentResponse{File: file}, nil
}

func (s *FileCenterServiceService) DownloadFileContent(ctx context.Context, req *pbCore.DownloadFileContentRequest) (*pbCore.DownloadFileContentResponse, error) {
	return s.uc.DownloadContent(ctx, req.GetId())
}

func (s *FileCenterServiceService) UploadFilePart(ctx context.Context, req *pbCore.UploadFilePartRequest) (*pbCore.UploadFilePartResponse, error) {
	return s.uc.UploadFilePart(ctx, req)
}

func (s *FileCenterServiceService) ListFileParts(ctx context.Context, req *pbCore.ListFilePartsRequest) (*pbCore.ListFilePartsResponse, error) {
	return s.uc.ListFileParts(ctx, req.GetId())
}

func (s *FileCenterServiceService) CompleteFileUpload(ctx context.Context, req *pbCore.CompleteFileUploadRequest) (*pbCore.CompleteFileUploadResponse, error) {
	file, err := s.uc.CompleteFileUpload(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.CompleteFileUploadResponse{File: file}, nil
}

func (s *FileCenterServiceService) AbortFileUpload(ctx context.Context, req *pbCore.AbortFileUploadRequest) (*pbCore.AbortFileUploadResponse, error) {
	if err := s.uc.AbortFileUpload(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.AbortFileUploadResponse{}, nil
}
