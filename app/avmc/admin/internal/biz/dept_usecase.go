package biz

import (
	"context"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

var (
// ErrDeptNotFound is user not found.
// ErrDeptNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// DeptRepo is a Greater repo.
type DeptRepo interface {
	Save(context.Context, *pbCore.Dept) (*pbCore.Dept, error)
	Update(context.Context, *pbCore.Dept) (*pbCore.Dept, error)
	FindByID(context.Context, uint32) (*pbCore.Dept, error)
	ListAll(context.Context) ([]*pbCore.Dept, error)
	ListPage(context.Context, *pbPagination.PagingRequest) (*pbCore.ListDeptResponse, error) // 新增的方法用于分页查询
	Delete(context.Context, uint32) error
}

// DeptUsecase is a Dept usecase.
// 包含部门仓库和日志记录器
type DeptUsecase struct {
	repo DeptRepo
	log  *log.Helper
}

// NewDeptUsecase new a Dept usecase.
// 参数：repo 部门仓库，logger 日志记录器
// 返回值：部门用例实例指针
func NewDeptUsecase(repo DeptRepo, logger log.Logger) *DeptUsecase {
	return &DeptUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create 处理创建部门请求
// 参数：ctx 上下文，g 部门信息
// 返回值：创建后的部门信息，错误信息
func (uc *DeptUsecase) Create(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	uc.log.WithContext(ctx).Infof("CreateDept: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

// Get 处理获取部门详情请求
// 参数：ctx 上下文，id 部门ID
// 返回值：部门详情，错误信息
func (uc *DeptUsecase) Get(ctx context.Context, id uint32) (*pbCore.Dept, error) {
	return uc.repo.FindByID(ctx, id)
}

// Update 处理更新部门请求
// 参数：ctx 上下文，g 部门信息
// 返回值：更新后的部门信息，错误信息
func (uc *DeptUsecase) Update(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	uc.log.WithContext(ctx).Infof("UpdateDept: %v", g.Name)
	_, err := uc.repo.FindByID(ctx, g.GetId())
	if err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, g)
}

// ListSimple 处理部门简单列表请求
// 参数：ctx 上下文，pageNum 页码，pageSize 每页数量
// 返回值：部门列表，错误信息
func (uc *DeptUsecase) ListSimple(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.Dept, error) {
	return uc.repo.ListAll(ctx)
}

// ListPage 处理部门分页列表请求
// 参数：ctx 上下文，pagination 分页请求
// 返回值：部门列表响应，错误信息
func (uc *DeptUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListDeptResponse, error) {
	return uc.repo.ListPage(ctx, pagination)
}

// Delete 处理删除部门请求
// 参数：ctx 上下文，id 部门ID
// 返回值：错误信息
func (uc *DeptUsecase) Delete(ctx context.Context, id uint32) error {
	uc.log.WithContext(ctx).Infof("DeleteDept: %v", id)
	_, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}
