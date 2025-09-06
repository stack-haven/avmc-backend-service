package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/role"
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

// toProto 转换gen.Role为pbCore.Role
func (r *roleRepo) toProto(res *gen.Role) *pbCore.Role {
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

// toEnt 转换pbCore.Role为gen.Role
func (r *roleRepo) toEnt(g *pbCore.Role) *gen.Role {
	return &gen.Role{
		ID:                g.GetId(),
		Name:              g.Name,
		DefaultRouter:     g.DefaultRouter,
		DataScope:         g.DataScope,
		Status:            (*int32)(g.Status),
		MenuCheckStrictly: g.MenuCheckStrictly,
		DeptCheckStrictly: g.DeptCheckStrictly,
	}
}

// Save 保存角色信息
// 参数：ctx 上下文，g 角色信息
// 返回值：角色信息，错误信息
func (r *roleRepo) Save(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	r.log.Infof("保存角色，角色信息：%v", g)
	entRole := r.toEnt(g)
	builder := r.data.DB(ctx).Role.Create()

	exist, _ := r.ExistByName(ctx, &pbCore.ExistRoleByNameRequest{
		Name: g.GetName(),
	})
	if exist {
		r.log.Errorf("角色名称已存在，角色信息：%v", g)
		return nil, fmt.Errorf("role name already exists")
	}

	res, err := builder.SetName(*entRole.Name).
		SetNillableDefaultRouter(entRole.DefaultRouter).
		SetNillableDataScope(entRole.DataScope).
		SetNillableMenuCheckStrictly(entRole.MenuCheckStrictly).
		SetNillableDeptCheckStrictly(entRole.DeptCheckStrictly).
		SetNillableStatus(entRole.Status).
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
	exist, _ := r.ExistByName(ctx, &pbCore.ExistRoleByNameRequest{
		Id:   &g.Id,
		Name: g.GetName(),
	})
	if exist {
		r.log.Errorf("角色名称已存在，角色信息：%v", g)
		return nil, fmt.Errorf("role name already exists")
	}

	entRole := r.toEnt(g)
	res, err := r.data.DB(ctx).Role.UpdateOneID(g.GetId()).
		SetName(*entRole.Name).
		SetNillableDefaultRouter(entRole.DefaultRouter).
		SetNillableDataScope(entRole.DataScope).
		SetNillableMenuCheckStrictly(entRole.MenuCheckStrictly).
		SetNillableDeptCheckStrictly(entRole.DeptCheckStrictly).
		SetNillableStatus(entRole.Status).
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
	res, err := r.data.DB(ctx).Role.Query().Where(role.ID(id)).First(ctx)
	if err != nil {
		r.log.Errorf("根据ID查询角色失败，角色ID：%v，错误：%v", id, err)
		if gen.IsNotFound(err) {
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
	res, err := r.data.DB(ctx).Role.Query().Where(role.NameContains(name)).All(ctx)
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
	res, err := r.data.DB(ctx).Role.Query().All(ctx)
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
func (r *roleRepo) ListPage(ctx context.Context, req *pbCore.ListRoleRequest) (*pbCore.ListRoleResponse, error) {
	r.log.Infof("分页查询角色，分页请求：%v", req)
	query := r.data.DB(ctx).Role.Query()
	if req.GetName() != "" {
		query = query.Where(role.NameContains(req.GetName()))
	}
	if req.GetStatus() != 0 {
		query = query.Where(role.Status(int32(req.GetStatus())))
	}
	if startTime := convert.StringValueToTime(req.StartCreatedAt, time.DateTime); startTime != nil {
		query = query.Where(role.CreatedAtGTE(*startTime))
	}
	if endTime := convert.StringValueToTime(req.EndCreatedAt, time.DateTime); endTime != nil {
		query = query.Where(role.CreatedAtLTE(*endTime))
	}

	// if req.GetStartCreatedAt() != "" && req.GetEndCreatedAt() != "" {
	// 	startTime := convert.StringValueToTime(req.StartCreatedAt, time.DateTime)
	// 	endTime := convert.StringValueToTime(req.EndCreatedAt, time.DateTime)
	// 	// 查询创建时间在指定范围内的角色
	// 	query = query.Where(role.CreatedAtGTE(*startTime), role.CreatedAtLTE(*endTime))
	// }

	count, err := query.Count(ctx)
	if err != nil {
		r.log.Errorf("查询角色总数失败，错误：%v", err)
		return nil, err
	}
	if count == 0 {
		return &pbCore.ListRoleResponse{
			Items: nil,
			Total: 0,
		}, nil
	}
	query = query.Select(
		role.FieldID,
		role.FieldName,
		role.FieldDefaultRouter,
		role.FieldDataScope,
		role.FieldMenuCheckStrictly,
		role.FieldDeptCheckStrictly,
		role.FieldStatus,
		role.FieldCreatedAt,
		role.FieldUpdatedAt,
	).
		Offset(int((req.GetPage() - 1) * req.GetPageSize())).
		Limit(int(req.GetPageSize())).
		Order(gen.Desc(role.FieldID))
		// query = paging.WithPagination(query, int(req.GetPage()), int(req.GetPageSize())).All(ctx)
	res, err := query.All(ctx)
	// 使用类型断言转换为具体的查询类型
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

// ExistByName 判断角色名称是否存在
// 参数：ctx 上下文，req 角色名称请求
// 返回值：是否存在，错误信息
func (r *roleRepo) ExistByName(ctx context.Context, req *pbCore.ExistRoleByNameRequest) (bool, error) {
	r.log.Infof("判断角色名称是否存在，角色名称：%s", req.GetName())
	if req.GetName() == "" {
		return false, nil
	}
	builder := r.data.DB(ctx).Role.Query()
	if req.GetId() != 0 {
		builder = builder.Where(role.IDNotIn(req.GetId()))
	}
	_, err := builder.Select(role.FieldID).Where(role.Name(req.GetName())).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return false, nil
		}
		r.log.Errorf("判断角色名称是否存在失败，角色名称：%s，错误：%v", req.GetName(), err)
		return false, err
	}
	return true, nil
}
