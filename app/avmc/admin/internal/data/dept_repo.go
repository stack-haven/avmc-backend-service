package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/dept"
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

// convertProto 转换gen.Dept为pbCore.Dept
func (r *deptRepo) convertProto(res *gen.Dept) *pbCore.Dept {
	return &pbCore.Dept{
		Id:       res.ID,
		Name:     res.Name,
		ParentId: res.ParentID,
		Ancestors: convert.ToAny(res.Ancestors, func(i int) uint32 {
			return uint32(i)
		}),
		LeaderId:  res.LeaderID,
		Sort:      res.Sort,
		Status:    convert.EmptyToNil(enum.Status(*res.Status)),
		Remark:    res.Remark,
		CreatedAt: convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt: convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

// convertEnt 转换pbCore.Dept为gen.Dept
func (r *deptRepo) convertEnt(g *pbCore.Dept) *gen.Dept {
	return &gen.Dept{
		ID:       g.GetId(),
		Name:     g.Name,
		ParentID: g.ParentId,
		Ancestors: convert.ToAny(g.Ancestors, func(i uint32) int {
			return int(i)
		}),
		LeaderID: g.LeaderId,
		Sort:     g.Sort,
		Status:   convert.ToPointer(int32(g.GetStatus())),
		Remark:   g.Remark,
	}
}

// Save 保存部门信息
// 参数：ctx 上下文，g 部门信息
// 返回值：部门信息，错误信息
func (r *deptRepo) Save(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	r.log.Infof("保存部门，部门信息：%v", g)
	entDept := r.convertEnt(g)
	builder := r.data.DB(ctx).Dept.Create()

	id, _ := r.GetDeptExistByName(ctx, *entDept.Name)
	if id > 0 {
		r.log.Errorf("部门名称已存在，部门信息：%v", g)
		return nil, fmt.Errorf("dept name already exists")
	}

	res, err := builder.SetName(*entDept.Name).
		SetNillableParentID(entDept.ParentID).
		SetNillableLeaderID(entDept.LeaderID).
		SetNillableSort(entDept.Sort).
		SetNillableRemark(entDept.Remark).
		SetNillableStatus(entDept.Status).
		SetAncestors(entDept.Ancestors).
		Save(ctx)
	if err != nil {
		r.log.Errorf("保存部门失败，部门信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.convertProto(res), nil
}

// GetDeptExistByName 获取部门名称是否存在
// 参数：ctx 上下文，name 部门名称
// 返回值：部门ID，错误信息
func (r *deptRepo) GetDeptExistByName(ctx context.Context, name string) (uint32, error) {
	r.log.Infof("获取部门名称是否存在，部门名称：%v", name)
	entDept, err := r.data.DB(ctx).Dept.Query().Where(dept.Name(name)).Select(dept.FieldID).First(ctx)
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
	entDept := r.convertEnt(g)
	builder := r.data.DB(ctx).Dept.UpdateOneID(g.GetId())
	id, _ := r.GetDeptExistByName(ctx, *entDept.Name)
	if id > 0 && id != g.GetId() {
		r.log.Errorf("部门名称已存在，部门信息：%v", g)
		return nil, fmt.Errorf("dept name already exists")
	}

	res, err := builder.
		SetName(*entDept.Name).
		SetNillableParentID(entDept.ParentID).
		SetNillableLeaderID(entDept.LeaderID).
		SetNillableSort(entDept.Sort).
		SetNillableRemark(entDept.Remark).
		SetNillableStatus(entDept.Status).
		SetAncestors(entDept.Ancestors).
		Save(ctx)
	if err != nil {
		r.log.Errorf("更新部门失败，部门信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.convertProto(res), nil
}

// FindByID 通过ID查询部门信息
// 参数：ctx 上下文，id 部门ID
// 返回值：部门信息，错误信息
func (r *deptRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Dept, error) {
	r.log.Infof("通过ID查询部门，ID：%d", id)
	res, err := r.data.DB(ctx).Dept.Query().
		Where(dept.IDEQ(id)).Only(ctx)
	if err != nil {
		r.log.Errorf("通过ID查询部门失败，ID：%d，错误：%v", id, err)
		if gen.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
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
	res, err := r.data.DB(ctx).Dept.Query().Where(dept.NameContains(name)).All(ctx)
	if err != nil {
		r.log.Errorf("通过部门名称查询部门失败，部门名称：%s，错误：%v", name, err)
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

// CountDepts 查询部门数量
// 参数：ctx 上下文，filter 过滤条件
// 返回值：部门数量，错误信息
func (r *deptRepo) CountDepts(ctx context.Context, opts ...biz.ListOption) (int32, error) {
	r.log.Infof("查询部门数量，过滤条件：%v", opts)
	o := biz.ListOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.data.db.Dept.Query().
		Select(dept.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		r.log.Errorf("查询所有部门列表失败，错误：%v", err)
		return 0, err
	}
	return int32(count), nil
}

// ListAll 查询所有部门列表
// 参数：ctx 上下文
// 返回值：部门列表，错误信息
func (r *deptRepo) ListAll(ctx context.Context) ([]*pbCore.Dept, error) {
	r.log.Infof("查询所有部门列表")
	res, err := r.data.DB(ctx).Dept.Query().Select(dept.FieldID, dept.FieldName, dept.FieldParentID, dept.FieldStatus, dept.FieldRemark, dept.FieldLeaderID, dept.FieldSort).Where().Order(gen.Desc(dept.FieldID)).All(ctx)
	if err != nil {
		r.log.Errorf("查询所有部门列表失败，错误：%v", err)
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

// ListDepts 查询部门列表
// 参数：ctx 上下文，opts 分页选项
// 返回值：部门列表，错误信息
func (r *deptRepo) ListDepts(ctx context.Context, opts ...biz.ListOption) ([]*pbCore.Dept, error) {
	r.log.Infof("查询部门列表，分页选项：%v", opts)
	o := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	pos, err := r.data.db.Dept.Query().
		Select(
			dept.FieldID,
			dept.FieldName,
			dept.FieldParentID,
			dept.FieldStatus,
			dept.FieldRemark,
			dept.FieldLeaderID,
			dept.FieldSort,
			dept.FieldCreatedAt,
			dept.FieldUpdatedAt,
		).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).
		Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(pos, r.convertProto), nil
}
