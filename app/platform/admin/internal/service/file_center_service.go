package service

import (
	"context"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
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

func (s *FileCenterServiceService) PresignFileDownload(ctx context.Context, req *pbCore.PresignFileDownloadRequest) (*pbCore.PresignFileDownloadResponse, error) {
	return s.uc.PresignDownload(ctx, req)
}

func (s *FileCenterServiceService) DeleteFileObject(ctx context.Context, req *pbCore.DeleteFileObjectRequest) (*pbCore.DeleteFileObjectResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteFileObjectResponse{}, nil
}
