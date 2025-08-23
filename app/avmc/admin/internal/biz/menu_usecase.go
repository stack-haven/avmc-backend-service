package biz

import (
	"context"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

var (
// ErrMenuNotFound is user not found.
// ErrMenuNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// MenuRepo is a Greater repo.
type MenuRepo interface {
	Save(context.Context, *pbCore.Menu) (*pbCore.Menu, error)
	Update(context.Context, *pbCore.Menu) (*pbCore.Menu, error)
	FindByID(context.Context, uint32) (*pbCore.Menu, error)
	ListAll(context.Context) ([]*pbCore.Menu, error)
	ListPage(context.Context, *pbPagination.PagingRequest) (*pbCore.ListMenuResponse, error) // 新增的方法用于分页查询
	Delete(context.Context, uint32) error
	ExistByName(context.Context, *pbCore.ExistMenuByNameRequest) (bool, error)
	ExistByPath(context.Context, *pbCore.ExistMenuByPathRequest) (bool, error)
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
func (uc *MenuUsecase) Create(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	uc.log.WithContext(ctx).Infof("CreateMenu: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

// Get 处理获取菜单详情请求
// 参数：ctx 上下文，id 菜单ID
// 返回值：菜单详情，错误信息
func (uc *MenuUsecase) Get(ctx context.Context, id uint32) (*pbCore.Menu, error) {
	return uc.repo.FindByID(ctx, id)
}

// Update 处理更新菜单请求
// 参数：ctx 上下文，g 菜单信息
// 返回值：更新后的菜单信息，错误信息
func (uc *MenuUsecase) Update(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	uc.log.WithContext(ctx).Infof("UpdateMenu: %v", g.Name)
	_, err := uc.repo.FindByID(ctx, g.GetId())
	if err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, g)
}

// ListSimple 处理获取菜单简单列表请求
// 参数：ctx 上下文，pageNum 页码，pageSize 每页数量
// 返回值：菜单列表，错误信息
func (uc *MenuUsecase) ListSimple(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.Menu, error) {
	return uc.repo.ListAll(ctx)
}

// ListPage 处理获取菜单分页列表请求
// 参数：ctx 上下文，pagination 分页请求
// 返回值：菜单列表响应，错误信息
func (uc *MenuUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListMenuResponse, error) {
	return uc.repo.ListPage(ctx, pagination)
}

// ListTree 处理获取菜单树形列表请求
// 参数：ctx 上下文，pagination 分页请求
// 返回值：菜单树形列表响应，错误信息
func (uc *MenuUsecase) ListTree(ctx context.Context, pid uint32) (*pbCore.ListMenuTreeResponse, error) {
	menus, err := uc.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	tree := make([]*pbCore.Menu, 0)
	for _, menu := range menus {
		if menu.GetPid() == pid {
			tree = append(tree, menu)
		}
	}
	return &pbCore.ListMenuTreeResponse{Items: tree}, nil
}

// Delete 处理删除菜单请求
// 参数：ctx 上下文，id 菜单ID
// 返回值：错误信息
func (uc *MenuUsecase) Delete(ctx context.Context, id uint32) error {
	uc.log.WithContext(ctx).Infof("DeleteMenu: %v", id)
	_, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}

// ExistByPath 处理判断菜单路径是否存在请求
// 参数：ctx 上下文，req 判断菜单路径是否存在请求
// 返回值：是否存在，错误信息
func (uc *MenuUsecase) ExistByPath(ctx context.Context, req *pbCore.ExistMenuByPathRequest) (bool, error) {
	uc.log.WithContext(ctx).Infof("ExistByPath：%v", req.GetPath())
	return uc.repo.ExistByPath(ctx, req)
}

// ExistByName 处理判断菜单名是否存在请求
// 参数：ctx 上下文，req 判断菜单名是否存在请求
// 返回值：是否存在，错误信息
func (uc *MenuUsecase) ExistByName(ctx context.Context, req *pbCore.ExistMenuByNameRequest) (bool, error) {
	uc.log.WithContext(ctx).Infof("ExistByName：%v", req.GetName())
	return uc.repo.ExistByName(ctx, req)
}
