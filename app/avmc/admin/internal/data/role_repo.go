package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent"
	"backend-service/app/avmc/admin/internal/data/ent/role"
	"backend-service/pkg/utils/convert"
)

var _ biz.RoleRepo = (*roleRepo)(nil)

type roleRepo struct {
	data *Data
	log  *log.Helper
}

// NewRoleRepo 创建新的角色仓库实例
// 参数：data 数据访问层实例，logger 日志记录器
// 返回值：角色仓库实例指针
func NewRoleRepo(data *Data, logger log.Logger) biz.RoleRepo {
	return &roleRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// toProto 转换ent.Role为pbCore.Role
func (r *roleRepo) toProto(res *ent.Role) *pbCore.Role {
	return &pbCore.Role{
		Id:                res.ID,
		Name:              res.Name,
		DefaultRouter:     res.DefaultRouter,
		DataScope:         res.DataScope,
		Status:            (*enum.Status)(res.Status),
		MenuCheckStrictly: res.MenuCheckStrictly,
		DeptCheckStrictly: res.DeptCheckStrictly,
		CreatedAt:         convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt:         convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

// toEnt 转换pbCore.Role为ent.Role
func (r *roleRepo) toEnt(g *pbCore.Role) *ent.Role {
	return &ent.Role{
		ID:                g.GetId(),
		Name:              g.Name,
		DefaultRouter:     g.DefaultRouter,
		DataScope:         g.DataScope,
		Status:            (*int32)(g.Status),
		MenuCheckStrictly: g.MenuCheckStrictly,
		DeptCheckStrictly: g.DeptCheckStrictly,
	}
}

// GetRoleExistByName 获取角色名称是否存在
// 参数：ctx 上下文，name 角色名称
// 返回值：角色ID，错误信息
func (r *roleRepo) GetRoleExistByName(ctx context.Context, name string) (uint32, error) {
	r.log.Infof("获取角色名称是否存在，角色名称：%v", name)
	entRole, err := r.data.DB(ctx).Role.Query().Where(role.Name(name), role.DeletedAtIsNil()).Select(role.FieldID).First(ctx)
	if err != nil {
		r.log.Errorf("获取角色名称是否存在失败，角色名称：%v，错误：%v", name, err)
		return 0, err
	}
	return entRole.ID, nil
}

// Save 保存角色信息
// 参数：ctx 上下文，g 角色信息
// 返回值：角色信息，错误信息
func (r *roleRepo) Save(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	r.log.Infof("保存角色，角色信息：%v", g)
	entRole := r.toEnt(g)
	builder := r.data.DB(ctx).Role.Create()

	id, _ := r.GetRoleExistByName(ctx, *entRole.Name)
	if id > 0 {
		r.log.Errorf("角色名称已存在，角色信息：%v", g)
		return nil, fmt.Errorf("role name already exists")
	}

	res, err := builder.SetName(*entRole.Name).
		SetNillableDefaultRouter(entRole.DefaultRouter).
		SetNillableDataScope(entRole.DataScope).
		SetNillableMenuCheckStrictly(entRole.MenuCheckStrictly).
		SetNillableDeptCheckStrictly(entRole.DeptCheckStrictly).
		Save(ctx)
	if err != nil {
		r.log.Errorf("保存角色失败，角色信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// Update 更新角色信息
// 参数：ctx 上下文，g 角色信息
// 返回值：角色信息，错误信息
func (r *roleRepo) Update(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	r.log.Infof("更新角色，角色信息：%v", g)
	entRole := r.toEnt(g)
	id, _ := r.GetRoleExistByName(ctx, *entRole.Name)
	if id > 0 && id != g.GetId() {
		r.log.Errorf("角色名称已存在，角色信息：%v", g)
		return nil, fmt.Errorf("role name already exists")
	}

	res, err := r.data.DB(ctx).Role.UpdateOneID(g.GetId()).
		SetName(*entRole.Name).
		SetNillableDefaultRouter(entRole.DefaultRouter).
		SetNillableDataScope(entRole.DataScope).
		SetNillableMenuCheckStrictly(entRole.MenuCheckStrictly).
		SetNillableDeptCheckStrictly(entRole.DeptCheckStrictly).
		Save(ctx)
	if err != nil {
		r.log.Errorf("更新角色失败，角色信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// FindByID 根据ID查询角色信息
// 参数：ctx 上下文，id 角色ID
// 返回值：角色信息，错误信息
func (r *roleRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Role, error) {
	r.log.Infof("根据ID查询角色，角色ID：%v", id)
	res, err := r.data.DB(ctx).Role.Query().Where(role.ID(id), role.DeletedAtIsNil()).First(ctx)
	if err != nil {
		r.log.Errorf("根据ID查询角色失败，角色ID：%v，错误：%v", id, err)
		if ent.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.toProto(res), nil
}

// Delete 软删除角色
// 参数：ctx 上下文，id 角色ID
// 返回值：错误信息
func (r *roleRepo) Delete(ctx context.Context, id uint32) error {
	r.log.Infof("删除角色，角色ID：%v", id)
	err := r.data.DB(ctx).Role.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if err != nil {
		r.log.Errorf("删除角色失败，角色ID：%v，错误：%v", id, err)
		return err
	}
	return nil
}

// ListByName 根据名称模糊查询角色列表
// 参数：ctx 上下文，name 角色名称
// 返回值：角色列表，错误信息
func (r *roleRepo) ListByName(ctx context.Context, name string) ([]*pbCore.Role, error) {
	r.log.Infof("根据名称模糊查询角色，角色名称：%v", name)
	res, err := r.data.DB(ctx).Role.Query().Where(role.NameContains(name), role.DeletedAtIsNil()).All(ctx)
	if err != nil {
		r.log.Errorf("根据名称模糊查询角色失败，角色名称：%v，错误：%v", name, err)
		return nil, err
	}

	var roles []*pbCore.Role
	for _, role := range res {
		roles = append(roles, r.toProto(role))
	}
	return roles, nil
}

// ListAll 查询所有角色
// 参数：ctx 上下文
// 返回值：角色列表，错误信息
func (r *roleRepo) ListAll(ctx context.Context) ([]*pbCore.Role, error) {
	r.log.Infof("查询所有角色")
	res, err := r.data.DB(ctx).Role.Query().Where(role.DeletedAtIsNil()).All(ctx)
	if err != nil {
		r.log.Errorf("查询所有角色失败，错误：%v", err)
		return nil, err
	}

	var roles []*pbCore.Role
	for _, role := range res {
		roles = append(roles, r.toProto(role))
	}
	return roles, nil
}

// ListPage 分页查询角色
// 参数：ctx 上下文，pagination 分页请求
// 返回值：角色列表响应，错误信息
func (r *roleRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListRoleResponse, error) {
	r.log.Infof("分页查询角色，分页请求：%v", pagination)

	// 查询总数
	count, err := r.data.DB(ctx).Role.Query().Where(role.DeletedAtIsNil()).Count(ctx)
	if err != nil {
		r.log.Errorf("查询角色总数失败，错误：%v", err)
		return nil, err
	}

	// 计算偏移量
	offset := (pagination.GetPage() - 1) * pagination.GetPageSize()

	// 查询分页数据
	res, err := r.data.DB(ctx).Role.Query().Where(role.DeletedAtIsNil()).Offset(int(offset)).Limit(int(pagination.GetPageSize())).All(ctx)
	if err != nil {
		r.log.Errorf("分页查询角色失败，错误：%v", err)
		return nil, err
	}

	// 转换数据
	return &pbCore.ListRoleResponse{
		Items: convert.SliceToAny(res, r.toProto),
		Total: int32(count),
	}, nil
}

// 重复方法定义已删除
