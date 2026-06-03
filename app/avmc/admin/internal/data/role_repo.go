package data

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/menu"
	"backend-service/app/avmc/admin/internal/data/ent/gen/role"
	"backend-service/pkg/utils/convert"
)

var _ biz.RoleRepo = (*roleRepo)(nil)

type roleRepo struct {
	BaseRepo
}

// NewRoleRepo 创建角色仓库
func NewRoleRepo(data *Data, logger log.Logger) biz.RoleRepo {
	return &roleRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// entToProto 将 ent.Role 转换为 pbCore.Role
func (r *roleRepo) entToProto(e *gen.Role) *pbCore.Role {
	if e == nil {
		return nil
	}
	status := pbEnum.Status(0)
	if e.Status != nil {
		status = pbEnum.Status(*e.Status)
	}
	return &pbCore.Role{
		Id:                e.ID,
		Name:              e.Name,
		DefaultRouter:     e.DefaultRouter,
		DataScope:         e.DataScope,
		Status:            &status,
		MenuCheckStrictly: e.MenuCheckStrictly,
		DeptCheckStrictly: e.DeptCheckStrictly,
		MenuIds:           roleMenuIDs(e),
		CreatedAt:         convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:         convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
	}
}

// protoToEnt 将 pbCore.Role 转换为 ent.Role
func (r *roleRepo) protoToEnt(g *pbCore.Role) *gen.Role {
	return &gen.Role{
		ID:                g.GetId(),
		Name:              g.Name,
		DefaultRouter:     g.DefaultRouter,
		DataScope:         g.DataScope,
		Status:            (*int32)(g.Status),
		MenuCheckStrictly: g.MenuCheckStrictly,
		DeptCheckStrictly: g.DeptCheckStrictly,
		Edges: gen.RoleEdges{
			Menus: roleMenus(g.GetMenuIds()),
		},
	}
}

// Save 保存角色
func (r *roleRepo) Save(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	r.Log.Infof("保存角色: %s", g.GetName())
	ent := r.protoToEnt(g)

	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)

	builder := tx.Role.Create().
		SetName(g.GetName()).
		SetNillableDefaultRouter(ent.DefaultRouter).
		SetNillableDataScope(ent.DataScope).
		SetNillableMenuCheckStrictly(ent.MenuCheckStrictly).
		SetNillableDeptCheckStrictly(ent.DeptCheckStrictly).
		SetNillableStatus(ent.Status)
	if len(g.GetMenuIds()) > 0 {
		builder.AddMenuIDs(g.GetMenuIds()...)
	}
	res, err := builder.Save(ctx)
	if err != nil {
		r.Log.Errorf("保存角色失败: %v", err)
		if gen.IsConstraintError(err) {
			return nil, fmt.Errorf("角色名称已存在")
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.entToProto(res), nil
}

// Update 更新角色
func (r *roleRepo) Update(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	r.Log.Infof("更新角色 ID: %d", g.GetId())
	ent := r.protoToEnt(g)

	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)

	builder := tx.Role.UpdateOneID(g.GetId()).
		SetName(g.GetName()).
		SetNillableDefaultRouter(ent.DefaultRouter).
		SetNillableDataScope(ent.DataScope).
		SetNillableMenuCheckStrictly(ent.MenuCheckStrictly).
		SetNillableDeptCheckStrictly(ent.DeptCheckStrictly).
		SetNillableStatus(ent.Status)
	if g.MenuIds != nil {
		builder.ClearMenus()
		if len(g.MenuIds) > 0 {
			builder.AddMenuIDs(g.MenuIds...)
		}
	}
	res, err := builder.Save(ctx)
	if err != nil {
		r.Log.Errorf("更新角色失败: %v", err)
		if gen.IsConstraintError(err) {
			return nil, fmt.Errorf("角色名称已存在")
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.entToProto(res), nil
}

// FindByID 根据 ID 查询
func (r *roleRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Role, error) {
	r.Log.Infof("查询角色 ID: %d", id)
	res, err := r.Data.DB(ctx).Role.Query().
		Select(
			role.FieldID, role.FieldName,
			role.FieldDefaultRouter, role.FieldDataScope,
			role.FieldMenuCheckStrictly, role.FieldDeptCheckStrictly,
			role.FieldStatus,
			role.FieldCreatedAt, role.FieldUpdatedAt,
		).
		WithMenus(func(q *gen.MenuQuery) {
			q.Select(menu.FieldID)
		}).
		Where(role.ID(id)).
		First(ctx)
	if err != nil {
		r.Log.Errorf("查询角色失败 ID: %d, err: %v", id, err)
		if gen.IsNotFound(err) {
			return nil, fmt.Errorf("角色不存在")
		}
		return nil, err
	}
	return r.entToProto(res), nil
}

// Delete 软删除
func (r *roleRepo) Delete(ctx context.Context, id uint32) error {
	return r.Data.DB(ctx).Role.UpdateOneID(id).
		SetDeletedAt(time.Now()).
		Exec(ctx)
}

// ListAll 查询所有角色
func (r *roleRepo) ListAll(ctx context.Context) ([]*pbCore.Role, error) {
	res, err := r.Data.DB(ctx).Role.Query().
		Select(role.FieldID, role.FieldName, role.FieldStatus).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(res, r.entToProto), nil
}

// CountRoles 角色计数
func (r *roleRepo) CountRoles(ctx context.Context, opts ...biz.ListOption) (int32, error) {
	o := biz.ListOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).Role.Query().
		Select(role.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

// ListRoles 分页查询角色
func (r *roleRepo) ListRoles(ctx context.Context, opts ...biz.ListOption) ([]*pbCore.Role, error) {
	o := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	res, err := r.Data.DB(ctx).Role.Query().
		Select(
			role.FieldID, role.FieldName,
			role.FieldDefaultRouter, role.FieldDataScope,
			role.FieldMenuCheckStrictly, role.FieldDeptCheckStrictly,
			role.FieldStatus,
			role.FieldCreatedAt, role.FieldUpdatedAt,
		).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(res, r.entToProto), nil
}

// ExistByName 判断角色名是否存在
func (r *roleRepo) ExistByName(ctx context.Context, name string, excludeID uint32) (bool, error) {
	if name == "" {
		return false, nil
	}
	builder := r.Data.DB(ctx).Role.Query()
	if excludeID != 0 {
		builder = builder.Where(role.IDNotIn(excludeID))
	}
	_, err := builder.Select(role.FieldID).Where(role.Name(name)).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func roleMenus(ids []uint32) []*gen.Menu {
	menus := make([]*gen.Menu, 0, len(ids))
	for _, id := range ids {
		menus = append(menus, &gen.Menu{ID: id})
	}
	return menus
}

func roleMenuIDs(e *gen.Role) []uint32 {
	if e == nil || len(e.Edges.Menus) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(e.Edges.Menus))
	for _, m := range e.Edges.Menus {
		if m != nil {
			ids = append(ids, m.ID)
		}
	}
	return ids
}
