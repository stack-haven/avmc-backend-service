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

// MenuServiceService 菜单服务结构体
// 包含业务用例和日志记录器
type MenuServiceService struct {
	pb.UnimplementedMenuServiceServer
	muc *biz.MenuUsecase
	log *log.Helper
}

// NewMenuServiceService 创建新的菜单服务实例
// 参数：muc 菜单业务用例实例，logger 日志记录器
// 返回值：菜单服务实例指针
func NewMenuServiceService(muc *biz.MenuUsecase, logger log.Logger) *MenuServiceService {
	return &MenuServiceService{
		muc: muc,
		log: log.NewHelper(logger),
	}
}

// ListMenu 处理菜单列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：菜单列表响应，错误信息
func (s *MenuServiceService) ListMenus(ctx context.Context, req *pbCore.ListMenusRequest) (*pbCore.ListMenusResponse, error) {
	s.log.Infof("查询菜单列表分页，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	params, err := listing.ParseParams(
		req,
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("path", filtering.TypeString),
		filtering.DeclareIdent("status", filtering.TypeInt),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	req.PageSize = int32(params.PageSize)
	count, err := s.muc.CountMenus(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := pbCore.ListMenusResponse{
		Total: count,
	}
	resp.Items, err = s.muc.ListMenus(ctx,
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

// ListMenuTree 处理菜单树形列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：菜单树形列表响应，错误信息
func (s *MenuServiceService) ListMenusTree(ctx context.Context, req *pbCore.ListMenusTreeRequest) (*pbCore.ListMenusTreeResponse, error) {
	s.log.Infof("查询菜单树，parent_id=%d", req.GetParentId())
	return s.muc.ListTree(ctx, req.GetParentId())
}

// GetMenu 处理获取菜单详情请求
// 参数：ctx 上下文，req 获取菜单请求
// 返回值：菜单详情，错误信息
func (s *MenuServiceService) GetMenu(ctx context.Context, req *pbCore.GetMenuRequest) (*pbCore.Menu, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorMenuInvalidId("菜单ID不能为空")
	}
	s.log.Infof("获取菜单详情，菜单ID：%v", req.Id)
	return s.muc.Get(ctx, req.Id)
}

// CreateMenu 处理创建菜单请求
// 参数：ctx 上下文，req 创建菜单请求
// 返回值：创建菜单响应，错误信息
func (s *MenuServiceService) CreateMenu(ctx context.Context, req *pbCore.CreateMenuRequest) (*pbCore.CreateMenuResponse, error) {
	if req.GetMenu() == nil {
		return nil, pb.ErrorMenuInvalidId("菜单信息不能为空")
	}
	s.log.Infof("创建菜单，菜单名称：%s", req.GetMenu().GetName())
	_, err := s.muc.Create(ctx, req.Menu)
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateMenuResponse{}, nil
}

// UpdateMenu 处理更新菜单请求
// 参数：ctx 上下文，req 更新菜单请求
// 返回值：更新菜单响应，错误信息
func (s *MenuServiceService) UpdateMenu(ctx context.Context, req *pbCore.UpdateMenuRequest) (*pbCore.UpdateMenuResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorMenuInvalidId("菜单ID不能为空")
	}
	if req.GetMenu() == nil {
		return nil, pb.ErrorMenuInvalidId("菜单信息不能为空")
	}
	existing, err := s.GetMenu(ctx, &pbCore.GetMenuRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	fieldmask.Update(req.UpdateMask, existing, req.Menu)
	s.log.Infof("更新菜单，菜单ID：%v", req.GetId())
	existing.Id = req.GetId()
	_, err = s.muc.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateMenuResponse{}, nil
}

// DeleteMenu 处理删除菜单请求
// 参数：ctx 上下文，req 删除菜单请求
// 返回值：删除菜单响应，错误信息
func (s *MenuServiceService) DeleteMenu(ctx context.Context, req *pbCore.DeleteMenuRequest) (*pbCore.DeleteMenuResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorMenuInvalidId("菜单ID不能为空")
	}
	s.log.Infof("删除菜单，菜单ID：%v", req.GetId())
	err := s.muc.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.DeleteMenuResponse{}, nil
}

// ExistMenuByPath 处理判断菜单路径是否存在请求
// 参数：ctx 上下文，req 判断菜单路径是否存在请求
// 返回值：判断菜单路径是否存在响应，错误信息
func (s *MenuServiceService) ExistMenuByPath(ctx context.Context, req *pbCore.ExistMenuByPathRequest) (*pbCore.ExistMenuByPathResponse, error) {
	if req.GetPath() == "" {
		return &pbCore.ExistMenuByPathResponse{
			Exist: false,
		}, nil
	}
	s.log.Infof("判断菜单路径是否存在，菜单路径：%v", req.GetPath())
	exist, err := s.muc.ExistByPath(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.ExistMenuByPathResponse{
		Exist: exist,
	}, nil
}

// ExistMenuByName 处理判断菜单名是否存在请求
// 参数：ctx 上下文，req 判断菜单名是否存在请求
// 返回值：判断菜单名是否存在响应，错误信息
func (s *MenuServiceService) ExistMenuByName(ctx context.Context, req *pbCore.ExistMenuByNameRequest) (*pbCore.ExistMenuByNameResponse, error) {
	if req.GetName() == "" {
		return &pbCore.ExistMenuByNameResponse{
			Exist: false,
		}, nil
	}
	s.log.Infof("判断菜单名是否存在，菜单名：%v", req.GetName())
	exist, err := s.muc.ExistByName(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.ExistMenuByNameResponse{
		Exist: exist,
	}, nil
}
