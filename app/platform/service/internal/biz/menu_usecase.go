package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/platform/service/v1"

	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

// MenuRepo is a Greater repo.
type MenuRepo interface {
	Save(context.Context, *pb.Menu) (*pb.Menu, error)
	Update(context.Context, *pb.Menu) (*pb.Menu, error)
	FindByID(context.Context, uint32) (*pb.Menu, error)
	CountMenus(context.Context, ...listing.Option) (int32, error)
	ListAll(context.Context) ([]*pb.Menu, error)
	ListMenus(context.Context, ...listing.Option) ([]*pb.Menu, error) // 新增的方法用于分页查询
	Delete(context.Context, uint32) error
	ExistByName(context.Context, *pb.ExistMenuByNameRequest) (bool, error)
	ExistByPath(context.Context, *pb.ExistMenuByPathRequest) (bool, error)
}

// MenuUsecase is a Menu usecase.
// 包含菜单仓库和日志记录器
type MenuUsecase struct {
	repo MenuRepo
	log  *log.Helper
}

// NewMenuUsecase new a Menu usecase.
// 参数：repo 菜单仓库，logger 日志记录器
// 返回值：菜单用例实例指针
func NewMenuUsecase(repo MenuRepo, logger log.Logger) *MenuUsecase {
	return &MenuUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create 处理创建菜单请求
// 参数：ctx 上下文，g 菜单信息
// 返回值：创建后的菜单信息，错误信息
func (uc *MenuUsecase) Create(ctx context.Context, g *pb.Menu) (*pb.Menu, error) {
	uc.log.WithContext(ctx).Infof("CreateMenu: %v", g.GetName())
	return uc.repo.Save(ctx, g)
}

// Get 处理获取菜单详情请求
// 参数：ctx 上下文，id 菜单ID
// 返回值：菜单详情，错误信息
func (uc *MenuUsecase) Get(ctx context.Context, id uint32) (*pb.Menu, error) {
	return uc.repo.FindByID(ctx, id)
}

// Update 处理更新菜单请求
// 参数：ctx 上下文，g 菜单信息
// 返回值：更新后的菜单信息，错误信息
func (uc *MenuUsecase) Update(ctx context.Context, g *pb.Menu) (*pb.Menu, error) {
	uc.log.WithContext(ctx).Infof("UpdateMenu: %v", g.GetName())
	_, err := uc.repo.FindByID(ctx, g.GetId())
	if err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, g)
}

// CountMenus 处理菜单条件查询聚合请求
// 参数：ctx 上下文，opts 分页选项
// 返回值：菜单数量，错误信息
func (uc *MenuUsecase) CountMenus(ctx context.Context, opts ...listing.Option) (int32, error) {
	resp, err := uc.repo.CountMenus(ctx, opts...)
	if err != nil {
		return 0, err
	}
	return resp, nil
}

// ListSimple 处理获取菜单简单列表请求
// 参数：ctx 上下文，pageNum 页码，pageSize 每页数量
// 返回值：菜单列表，错误信息
func (uc *MenuUsecase) ListSimple(ctx context.Context, pageNum, pageSize int64) ([]*pb.Menu, error) {
	return uc.repo.ListAll(ctx)
}

// ListMenu 处理获取菜单分页列表请求
// 参数：ctx 上下文，pagination 分页请求
// 返回值：菜单列表响应，错误信息
func (uc *MenuUsecase) ListMenus(ctx context.Context, opts ...listing.Option) ([]*pb.Menu, error) {
	return uc.repo.ListMenus(ctx, opts...)
}

// ListTree 处理获取菜单树形列表请求
// 参数：ctx 上下文，pagination 分页请求
// 返回值：菜单树形列表响应，错误信息
func (uc *MenuUsecase) ListTree(ctx context.Context, pid uint32) (*pb.ListMenusTreeResponse, error) {
	menus, err := uc.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	tree, err := convert.ToTree(menus, pid, func(parent *pb.Menu, childrens ...*pb.Menu) error {
		parent.Children = append(parent.Children, childrens...)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &pb.ListMenusTreeResponse{Items: tree}, nil
}

// Delete 处理删除菜单请求
// 参数：ctx 上下文，id 菜单ID
// 返回值：错误信息
func (uc *MenuUsecase) Delete(ctx context.Context, id uint32) error {
	uc.log.WithContext(ctx).Infof("DeleteMenu: %v", id)

	return uc.repo.Delete(ctx, id)
}

// ExistByPath 处理判断菜单路径是否存在请求
// 参数：ctx 上下文，req 判断菜单路径是否存在请求
// 返回值：是否存在，错误信息
func (uc *MenuUsecase) ExistByPath(ctx context.Context, req *pb.ExistMenuByPathRequest) (bool, error) {
	uc.log.WithContext(ctx).Infof("ExistByPath：%v", req.GetPath())
	return uc.repo.ExistByPath(ctx, req)
}

// ExistByName 处理判断菜单名是否存在请求
// 参数：ctx 上下文，req 判断菜单名是否存在请求
// 返回值：是否存在，错误信息
func (uc *MenuUsecase) ExistByName(ctx context.Context, req *pb.ExistMenuByNameRequest) (bool, error) {
	uc.log.WithContext(ctx).Infof("ExistByName：%v", req.GetName())
	return uc.repo.ExistByName(ctx, req)
}
