package service

import (
	"context"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
)

// scanTaskSortableFields 任务列表允许的排序字段。
var scanTaskSortableFields = []string{
	"status",
	"created_at",
}

// scanTaskFilterDeclarations 任务列表允许出现在 filter 里的字段。
// 必须与 data 层 scanTaskFilterFields 保持一致。
var scanTaskFilterDeclarations = []filtering.DeclarationOption{
	filtering.DeclareIdent("status", filtering.TypeString),
	filtering.DeclareIdent("source_id", filtering.TypeInt),
	filtering.DeclareIdent("full_scan", filtering.TypeBool),
	filtering.DeclareIdent("failed", filtering.TypeInt),
	filtering.DeclareIdent("discovered", filtering.TypeInt),
	filtering.DeclareIdent("started_at", filtering.TypeTimestamp),
	filtering.DeclareIdent("finished_at", filtering.TypeTimestamp),
	filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
}

// ScanServiceService 扫描任务服务。
type ScanServiceService struct {
	pb.UnimplementedScanServiceServer
	uc  *biz.ScanUsecase
	log *log.Helper
}

// NewScanServiceService 创建扫描任务服务。
func NewScanServiceService(uc *biz.ScanUsecase, logger log.Logger) *ScanServiceService {
	return &ScanServiceService{uc: uc, log: log.NewHelper(log.With(logger, "module", "edai/service/scan"))}
}

// StartScan 发起扫描（异步）。
func (s *ScanServiceService) StartScan(ctx context.Context, req *pb.StartScanRequest) (*pb.StartScanResponse, error) {
	task, err := s.uc.Start(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.StartScanResponse{Task: task}, nil
}

// ListScanTasks 查询任务列表。
func (s *ScanServiceService) ListScanTasks(ctx context.Context, req *pb.ListScanTasksRequest) (*pb.ListScanTasksResponse, error) {
	params, err := parseListParams(req, scanTaskSortableFields, scanTaskFilterDeclarations...)
	if err != nil {
		return nil, err
	}
	items, total, err := s.uc.List(ctx, biz.ScanTaskListQuery{
		Filter:   params.Filter,
		OrderBy:  params.OrderBy,
		Offset:   params.Offset,
		Limit:    params.PageSize,
		SourceID: req.GetSourceId(),
	})
	if err != nil {
		return nil, err
	}
	resp := &pb.ListScanTasksResponse{Items: items, Total: total}
	if len(items) >= params.PageSize {
		resp.NextPageToken = formatOffset(params.Offset + params.PageSize)
	}
	return resp, nil
}

// GetScanTask 查询任务详情。
func (s *ScanServiceService) GetScanTask(ctx context.Context, req *pb.GetScanTaskRequest) (*pb.GetScanTaskResponse, error) {
	task, err := s.uc.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.GetScanTaskResponse{Task: task}, nil
}

// CancelScanTask 请求取消任务。
func (s *ScanServiceService) CancelScanTask(ctx context.Context, req *pb.CancelScanTaskRequest) (*pb.CancelScanTaskResponse, error) {
	task, err := s.uc.Cancel(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.CancelScanTaskResponse{Task: task}, nil
}
