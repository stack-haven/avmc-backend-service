package service

import (
	"context"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
)

// sourceSortableFields 数据源列表允许的排序字段（AIP order_by 白名单）。
var sourceSortableFields = []string{
	"name",
	"updated_at",
	"created_at",
}

// sourceFilterDeclarations 数据源列表允许出现在 filter 里的字段。
// 必须与 data 层 dataSourceFilterFields 保持一致。
var sourceFilterDeclarations = []filtering.DeclarationOption{
	filtering.DeclareIdent("name", filtering.TypeString),
	filtering.DeclareIdent("type", filtering.TypeString),
	filtering.DeclareIdent("root_path", filtering.TypeString),
	filtering.DeclareIdent("connection_status", filtering.TypeString),
	filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	filtering.DeclareIdent("updated_at", filtering.TypeTimestamp),
}

// SourceServiceService 数据源管理服务。
type SourceServiceService struct {
	pb.UnimplementedSourceServiceServer
	uc  *biz.SourceUsecase
	log *log.Helper
}

// NewSourceServiceService 创建数据源服务。
func NewSourceServiceService(uc *biz.SourceUsecase, logger log.Logger) *SourceServiceService {
	return &SourceServiceService{uc: uc, log: log.NewHelper(log.With(logger, "module", "edai/service/source"))}
}

// ListSources 查询数据源列表。
func (s *SourceServiceService) ListSources(ctx context.Context, req *pb.ListSourcesRequest) (*pb.ListSourcesResponse, error) {
	params, err := parseListParams(req, sourceSortableFields, sourceFilterDeclarations...)
	if err != nil {
		return nil, err
	}
	items, total, err := s.uc.List(ctx, biz.DataSourceListQuery{
		Filter:  params.Filter,
		OrderBy: params.OrderBy,
		Offset:  params.Offset,
		Limit:   params.PageSize,
	})
	if err != nil {
		return nil, err
	}
	resp := &pb.ListSourcesResponse{Items: items, Total: total}
	if len(items) >= params.PageSize {
		resp.NextPageToken = formatOffset(params.Offset + params.PageSize)
	}
	return resp, nil
}

// GetSource 查询单个数据源。
func (s *SourceServiceService) GetSource(ctx context.Context, req *pb.GetSourceRequest) (*pb.GetSourceResponse, error) {
	src, err := s.uc.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.GetSourceResponse{Source: src}, nil
}

// CreateSource 创建数据源。
func (s *SourceServiceService) CreateSource(ctx context.Context, req *pb.CreateSourceRequest) (*pb.CreateSourceResponse, error) {
	src, err := s.uc.Create(ctx, req.GetSource())
	if err != nil {
		return nil, err
	}
	return &pb.CreateSourceResponse{Source: src}, nil
}

// UpdateSource 更新数据源。
func (s *SourceServiceService) UpdateSource(ctx context.Context, req *pb.UpdateSourceRequest) (*pb.UpdateSourceResponse, error) {
	src, err := s.uc.Update(ctx, req.GetId(), req.GetSource())
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, pb.ErrorResourceNotFound("数据源不存在")
	}
	return &pb.UpdateSourceResponse{Source: src}, nil
}

// DeleteSource 删除数据源（仅配置，不触碰原始文件）。
func (s *SourceServiceService) DeleteSource(ctx context.Context, req *pb.DeleteSourceRequest) (*pb.DeleteSourceResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteSourceResponse{}, nil
}

// TestSourceConnection 连接测试。
func (s *SourceServiceService) TestSourceConnection(ctx context.Context, req *pb.TestSourceConnectionRequest) (*pb.TestSourceConnectionResponse, error) {
	return s.uc.TestConnection(ctx, req.GetId())
}
