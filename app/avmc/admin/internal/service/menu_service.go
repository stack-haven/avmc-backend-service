package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
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
func (s *MenuServiceService) ListMenu(ctx context.Context, req *pbPagination.PagingRequest) (*pbCore.ListMenuResponse, error) {
	s.log.Infof("查询菜单列表分页，分页请求：%v", req)
	return s.muc.ListPage(ctx, req)
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
	s.log.Infof("创建菜单，菜单信息：%v", req.Menu)
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
	s.log.Infof("更新菜单，菜单ID：%v，菜单信息：%v", req.GetId(), req.GetMenu())
	req.Menu.Id = req.GetId()
	_, err := s.muc.Update(ctx, req.GetMenu())
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
