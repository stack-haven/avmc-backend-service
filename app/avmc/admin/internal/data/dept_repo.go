package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent"
	"backend-service/app/avmc/admin/internal/data/ent/dept"
	"backend-service/pkg/utils/convert"
)

var _ biz.DeptRepo = (*deptRepo)(nil)

type deptRepo struct {
	data *Data
	log  *log.Helper
}

// NewDeptRepo 创建新的部门仓库实例
// 参数：data 数据访问层实例，logger 日志记录器
// 返回值：部门仓库实例指针
func NewDeptRepo(data *Data, logger log.Logger) biz.DeptRepo {
	return &deptRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// toProto 转换ent.Dept为pbCore.Dept
func (r *deptRepo) toProto(res *ent.Dept) *pbCore.Dept {
	return &pbCore.Dept{
		Id:          res.ID,
		Name:        res.Name,
		ParentId:    res.ParentID,
		CreatedAt:   convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt:   convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

// toEnt 转换pbCore.Dept为ent.Dept
func (r *deptRepo) toEnt(g *pbCore.Dept) *ent.Dept {
	return &ent.Dept{
		ID:       g.GetId(),
		Name:     g.Name,
		ParentID: g.ParentId,
	}
}

// Save 保存部门信息
// 参数：ctx 上下文，g 部门信息
// 返回值：部门信息，错误信息
func (r *deptRepo) Save(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	r.log.Infof("保存部门，部门信息：%v", g)
	entDept := r.toEnt(g)
	builder := r.data.DB(ctx).Dept.Create()

	id, _ := r.GetDeptExistByName(ctx, *entDept.Name)
	if id > 0 {
		r.log.Errorf("部门名称已存在，部门信息：%v", g)
		return nil, fmt.Errorf("dept name already exists")
	}

	res, err := builder.SetName(*entDept.Name).
		SetNillableParentID(entDept.ParentID).
		Save(ctx)
	if err != nil {
		r.log.Errorf("保存部门失败，部门信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// GetDeptExistByName 获取部门名称是否存在
// 参数：ctx 上下文，name 部门名称
// 返回值：部门ID，错误信息
func (r *deptRepo) GetDeptExistByName(ctx context.Context, name string) (uint32, error) {
	r.log.Infof("获取部门名称是否存在，部门名称：%v", name)
	entDept, err := r.data.DB(ctx).Dept.Query().Where(dept.Name(name), dept.DeletedAtIsNil()).Select(dept.FieldID).First(ctx)
	if err != nil {
		r.log.Errorf("获取部门名称是否存在失败，部门名称：%v，错误：%v", name, err)
		return 0, err
	}
	return entDept.ID, nil
}

// Update 更新部门信息
// 参数：ctx 上下文，g 部门信息
// 返回值：部门信息，错误信息
func (r *deptRepo) Update(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	r.log.Infof("更新部门，部门信息：%v", g)
	entDept := r.toEnt(g)
	builder := r.data.DB(ctx).Dept.UpdateOneID(g.GetId())
	id, _ := r.GetDeptExistByName(ctx, *entDept.Name)
	if id > 0 && id != g.GetId() {
		r.log.Errorf("部门名称已存在，部门信息：%v", g)
		return nil, fmt.Errorf("dept name already exists")
	}

	res, err := builder.
		SetName(*entDept.Name).
		SetNillableParentID(entDept.ParentID).
		Save(ctx)
	if err != nil {
		r.log.Errorf("更新部门失败，部门信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// FindByID 通过ID查询部门信息
// 参数：ctx 上下文，id 部门ID
// 返回值：部门信息，错误信息
func (r *deptRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Dept, error) {
	r.log.Infof("通过ID查询部门，ID：%d", id)
	res, err := r.data.DB(ctx).Dept.Query().
		Where(dept.IDEQ(id), dept.DeletedAtIsNil()).Only(ctx)
	if err != nil {
		r.log.Errorf("通过ID查询部门失败，ID：%d，错误：%v", id, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// Delete 删除部门
// 参数：ctx 上下文，id 部门ID
// 返回值：错误信息
func (r *deptRepo) Delete(ctx context.Context, id uint32) error {
	r.log.Infof("删除部门，部门ID：%d", id)
	err := r.data.DB(ctx).Dept.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if err != nil {
		r.log.Errorf("删除部门失败，部门ID：%d，错误：%v", id, err)
		return err
	}
	return nil
}

// ListByName 通过部门名称查询部门列表
// 参数：ctx 上下文，name 部门名称
// 返回值：部门列表，错误信息
func (r *deptRepo) ListByName(ctx context.Context, name string) ([]*pbCore.Dept, error) {
	r.log.Infof("通过部门名称查询部门，部门名称：%s", name)
	res, err := r.data.DB(ctx).Dept.Query().Where(dept.NameContains(name), dept.DeletedAtIsNil()).All(ctx)
	if err != nil {
		r.log.Errorf("通过部门名称查询部门失败，部门名称：%s，错误：%v", name, err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListAll 查询所有部门列表
// 参数：ctx 上下文
// 返回值：部门列表，错误信息
func (r *deptRepo) ListAll(ctx context.Context) ([]*pbCore.Dept, error) {
	r.log.Infof("查询所有部门列表")
	res, err := r.data.DB(ctx).Dept.Query().Select(dept.FieldID, dept.FieldName).Where(dept.DeletedAtIsNil()).Order(ent.Desc(dept.FieldID)).All(ctx)
	if err != nil {
		r.log.Errorf("查询所有部门列表失败，错误：%v", err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListPage 查询部门列表分页
// 参数：ctx 上下文，pagination 分页请求
// 返回值：部门列表响应，错误信息
func (r *deptRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListDeptResponse, error) {
	r.log.Infof("查询部门列表分页，分页请求：%v", pagination)
	count, err := r.data.DB(ctx).Dept.Query().Select(dept.FieldID).Where(dept.DeletedAtIsNil()).Count(ctx)
	if err != nil {
		r.log.Errorf("查询所有部门列表失败，错误：%v", err)
		return nil, err
	}
	res, err := r.data.DB(ctx).Dept.Query().
		Select(
			dept.FieldID,
			dept.FieldName,
			dept.FieldParentID,
			dept.FieldCreatedAt,
			dept.FieldUpdatedAt,
		).
		Where(dept.DeletedAtIsNil()).
		Offset(int((pagination.GetPage() - 1) * pagination.GetPageSize())).
		Limit(int(pagination.GetPageSize())).
		Order(ent.Desc(dept.FieldID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询部门列表分页失败，分页请求：%v，错误：%v", pagination, err)
		return nil, err
	}
	return &pbCore.ListDeptResponse{
		Items: convert.SliceToAny(res, r.toProto),
		Total: int32(count),
	}, nil
}
