package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"go.einride.tech/aip/pagination"
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
	pageSize := biz.NormalizePageSize(req.GetPageSize())
	req.PageSize = int32(pageSize)

	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return nil, err
	}

	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, err
	}
	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return nil, err
	}
	count, err := s.duc.CountDepts(ctx, biz.ListFilter(filter))
	if err != nil {
		return nil, err
	}
	resp := pbCore.ListDeptsResponse{
		Total: count,
	}
	resp.Items, err = s.duc.ListDepts(ctx,
		biz.ListFilter(filter),
		biz.ListOrderBy(orderBy),
		biz.ListLimit(pageSize),
		biz.ListOffset(int(pageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= pageSize {
		resp.NextPageToken = pageToken.Next(req).String()
	}
	return &resp, nil
}

// GetDept 处理获取部门详情请求
// 参数：ctx 上下文，req 获取部门请求
// 返回值：部门详情，错误信息
func (s *DeptServiceService) GetDept(ctx context.Context, req *pbCore.GetDeptRequest) (*pbCore.Dept, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorDeptInvalidId("部门ID不能为空")
	}
	s.log.Infof("获取部门详情，部门ID：%v", req.GetId())
	return s.duc.Get(ctx, req.GetId())
}

// CreateDept 处理创建部门请求
// 参数：ctx 上下文，req 创建部门请求
// 返回值：创建部门响应，错误信息
func (s *DeptServiceService) CreateDept(ctx context.Context, req *pbCore.CreateDeptRequest) (*pbCore.CreateDeptResponse, error) {
	if req.GetDept() == nil {
		return nil, pb.ErrorDeptInvalidId("部门信息不能为空")
	}
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
	if req.GetId() == 0 {
		return nil, pb.ErrorDeptInvalidId("部门ID不能为空")
	}
	if req.GetDept() == nil {
		return nil, pb.ErrorDeptInvalidId("部门信息不能为空")
	}
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
	if req.Id == 0 {
		return nil, pb.ErrorDeptInvalidId("部门ID不能为空")
	}
	s.log.Infof("删除部门，部门ID：%v", req.Id)
	err := s.duc.Delete(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pbCore.DeleteDeptResponse{}, nil
}

// ListDeptTree 处理部门树形列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：部门树形列表响应，错误信息
func (s *DeptServiceService) ListDeptsTree(ctx context.Context, req *pbCore.ListDeptsTreeRequest) (*pbCore.ListDeptsTreeResponse, error) {
	s.log.Infof("查询部门树，parent_id=%d", req.GetParentId())
	return s.duc.ListDeptsTree(ctx, req.GetParentId())
}
