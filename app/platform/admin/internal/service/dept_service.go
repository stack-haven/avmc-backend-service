package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/filtering"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"
)

// DeptServiceService 部门服务结构体
// 包含业务用例和日志记录器
type DeptServiceService struct {
	pb.UnimplementedDeptServiceServer
	duc *biz.DeptUsecase
	log *log.Helper
}

// NewDeptServiceService 创建新的部门服务实例
// 参数：duc 部门业务用例实例，logger 日志记录器
// 返回值：部门服务实例指针
func NewDeptServiceService(duc *biz.DeptUsecase, logger log.Logger) *DeptServiceService {
	return &DeptServiceService{
		duc: duc,
		log: log.NewHelper(logger),
	}
}

// ListDepts 处理部门列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：部门列表响应，错误信息
func (s *DeptServiceService) ListDepts(ctx context.Context, req *pbCore.ListDeptsRequest) (*pbCore.ListDeptsResponse, error) {
	s.log.Infof("查询部门列表分页，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	params, err := listing.ParseParams(
		req,
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	req.PageSize = int32(params.PageSize)
	count, err := s.duc.CountDepts(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := pbCore.ListDeptsResponse{
		Total: count,
	}
	resp.Items, err = s.duc.ListDepts(ctx,
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
	return &resp, nil
}

// GetDept 处理获取部门详情请求
// 参数：ctx 上下文，req 获取部门请求
// 返回值：部门详情，错误信息
func (s *DeptServiceService) GetDept(ctx context.Context, req *pbCore.GetDeptRequest) (*pbCore.Dept, error) {
	s.log.Infof("获取部门详情，部门ID：%v", req.GetId())
	return s.duc.Get(ctx, req.GetId())
}

// CreateDept 处理创建部门请求
// 参数：ctx 上下文，req 创建部门请求
// 返回值：创建部门响应，错误信息
func (s *DeptServiceService) CreateDept(ctx context.Context, req *pbCore.CreateDeptRequest) (*pbCore.CreateDeptResponse, error) {
	s.log.Infof("创建部门，部门名称：%s", req.GetDept().GetName())
	_, err := s.duc.Create(ctx, req.Dept)
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateDeptResponse{}, nil
}

// UpdateDept 处理更新部门请求
// 参数：ctx 上下文，req 更新部门请求
// 返回值：更新部门响应，错误信息
func (s *DeptServiceService) UpdateDept(ctx context.Context, req *pbCore.UpdateDeptRequest) (*pbCore.UpdateDeptResponse, error) {
	existing, err := s.GetDept(ctx, &pbCore.GetDeptRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	fieldmask.Update(req.UpdateMask, existing, req.Dept)
	s.log.Infof("更新部门，部门ID：%v", req.GetId())
	existing.Id = req.GetId()
	_, err = s.duc.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateDeptResponse{}, nil
}

// DeleteDept 处理删除部门请求
// 参数：ctx 上下文，req 删除部门请求
// 返回值：删除部门响应，错误信息
func (s *DeptServiceService) DeleteDept(ctx context.Context, req *pbCore.DeleteDeptRequest) (*pbCore.DeleteDeptResponse, error) {
	s.log.Infof("删除部门，部门ID：%v", req.Id)
	err := s.duc.Delete(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pbCore.DeleteDeptResponse{}, nil
}

func (s *DeptServiceService) GetDeptDeleteImpact(ctx context.Context, req *pbCore.GetDeptDeleteImpactRequest) (*pbCore.GetDeptDeleteImpactResponse, error) {
	return s.duc.GetDeleteImpact(ctx, req.GetId())
}

func (s *DeptServiceService) TransferAndDeleteDept(ctx context.Context, req *pbCore.TransferAndDeleteDeptRequest) (*pbCore.TransferAndDeleteDeptResponse, error) {
	count, err := s.duc.TransferAndDelete(ctx, req.GetId(), req.GetTargetDeptId())
	if err != nil {
		return nil, err
	}
	return &pbCore.TransferAndDeleteDeptResponse{TransferredUserCount: count}, nil
}

// ListDeptTree 处理部门树形列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：部门树形列表响应，错误信息
func (s *DeptServiceService) ListDeptsTree(ctx context.Context, req *pbCore.ListDeptsTreeRequest) (*pbCore.ListDeptsTreeResponse, error) {
	s.log.Infof("查询部门树，parent_id=%d", req.GetParentId())
	return s.duc.ListDeptsTree(ctx, req.GetParentId())
}
