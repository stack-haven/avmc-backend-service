package service

import (
	"context"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
)

// fileSortableFields 文件列表允许的排序字段（AIP order_by 白名单）。
var fileSortableFields = []string{
	"file_name",
	"file_size",
	"fs_modified_at",
	"created_at",
}

// fileFilterDeclarations 文件列表允许出现在 filter 里的字段。
//
// 必须与 data 层 file_filter.go 的翻译表保持一致：声明了但翻译表里没有的字段
// 会在执行时报 400，而不是被静默忽略。
var fileFilterDeclarations = []filtering.DeclarationOption{
	filtering.DeclareIdent("file_name", filtering.TypeString),
	filtering.DeclareIdent("raw_path", filtering.TypeString),
	filtering.DeclareIdent("raw_parent_path", filtering.TypeString),
	filtering.DeclareIdent("extension", filtering.TypeString),
	filtering.DeclareIdent("file_status", filtering.TypeString),
	filtering.DeclareIdent("source_id", filtering.TypeInt),
	filtering.DeclareIdent("file_size", filtering.TypeInt),
	filtering.DeclareIdent("is_duplicate", filtering.TypeBool),
	filtering.DeclareIdent("fs_created_at", filtering.TypeTimestamp),
	filtering.DeclareIdent("fs_modified_at", filtering.TypeTimestamp),
	filtering.DeclareIdent("first_seen_at", filtering.TypeTimestamp),
	filtering.DeclareIdent("last_scanned_at", filtering.TypeTimestamp),
}

// FileServiceService 文件记录查询服务。
type FileServiceService struct {
	pb.UnimplementedFileServiceServer
	uc  *biz.FileUsecase
	log *log.Helper
}

// NewFileServiceService 创建文件记录服务。
func NewFileServiceService(uc *biz.FileUsecase, logger log.Logger) *FileServiceService {
	return &FileServiceService{uc: uc, log: log.NewHelper(log.With(logger, "module", "edai/service/file"))}
}

// ListFiles 查询文件记录列表。
func (s *FileServiceService) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	params, err := parseListParams(req, fileSortableFields, fileFilterDeclarations...)
	if err != nil {
		return nil, err
	}
	items, total, err := s.uc.List(ctx, biz.FileListQuery{
		Filter:   params.Filter,
		OrderBy:  params.OrderBy,
		Offset:   params.Offset,
		Limit:    params.PageSize,
		SourceID: req.GetSourceId(),
	})
	if err != nil {
		return nil, err
	}
	resp := &pb.ListFilesResponse{Items: items, Total: total}
	if len(items) >= params.PageSize {
		resp.NextPageToken = formatOffset(params.Offset + params.PageSize)
	}
	return resp, nil
}

// GetFile 查询单个文件记录。
func (s *FileServiceService) GetFile(ctx context.Context, req *pb.GetFileRequest) (*pb.GetFileResponse, error) {
	rec, err := s.uc.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.GetFileResponse{File: rec}, nil
}

// GetFileStats 返回统计。
func (s *FileServiceService) GetFileStats(ctx context.Context, req *pb.GetFileStatsRequest) (*pb.GetFileStatsResponse, error) {
	return s.uc.Stats(ctx, req.GetSourceId())
}
