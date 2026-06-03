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
	"backend-service/app/avmc/admin/internal/data/ent/gen/project"
	"backend-service/pkg/utils/convert"
)

var _ biz.ProjectRepo = (*projectRepo)(nil)

type projectRepo struct {
	data *Data
	log  *log.Helper
}

// NewProjectRepo creates a project repo.
func NewProjectRepo(data *Data, logger log.Logger) biz.ProjectRepo {
	return &projectRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *projectRepo) convertProto(ctx context.Context, res *gen.Project) *pbCore.Project {
	status := enum.Status(0)
	if res.Status != nil {
		status = enum.Status(*res.Status)
	}
	memberIds := make([]uint32, 0)
	if members, err := res.Edges.MembersOrErr(); err == nil {
		for _, member := range members {
			memberIds = append(memberIds, member.ID)
		}
	}
	return &pbCore.Project{
		Id:          res.ID,
		Name:        convert.EmptyToNil(res.Name),
		Code:        res.Code,
		OwnerId:     res.OwnerID,
		OwnerName:   r.getOwnerName(ctx, res.OwnerID),
		Status:      convert.EmptyToNil(status),
		Description: res.Description,
		MemberIds:   memberIds,
		CreatedAt:   convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt:   convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

func (r *projectRepo) getOwnerName(ctx context.Context, ownerID *uint32) *string {
	if ownerID == nil || *ownerID == 0 {
		return nil
	}
	res, err := r.data.DB(ctx).User.Get(ctx, *ownerID)
	if err != nil {
		return nil
	}
	return res.Name
}

// Save saves project data.
func (r *projectRepo) Save(ctx context.Context, g *pbCore.Project) (*pbCore.Project, error) {
	r.log.Infof("保存项目，项目信息：%v", g)
	if id, _ := r.GetProjectExistByName(ctx, g.GetName()); id > 0 {
		return nil, fmt.Errorf("project name already exists")
	}
	if g.GetCode() != "" {
		if id, _ := r.GetProjectExistByCode(ctx, g.GetCode()); id > 0 {
			return nil, fmt.Errorf("project code already exists")
		}
	}

	builder := r.data.DB(ctx).Project.Create().
		SetName(g.GetName()).
		SetNillableCode(convert.EmptyToNil(g.GetCode())).
		SetNillableOwnerID(convert.EmptyToNil(g.GetOwnerId())).
		SetNillableStatus(convert.EmptyToNil(int32(g.GetStatus()))).
		SetNillableDescription(convert.EmptyToNil(g.GetDescription()))

	if len(g.GetMemberIds()) > 0 {
		builder.AddMemberIDs(g.GetMemberIds()...)
	}

	res, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("保存项目失败，项目信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.FindByID(ctx, res.ID)
}

// GetProjectExistByName checks project name existence.
func (r *projectRepo) GetProjectExistByName(ctx context.Context, name string) (uint32, error) {
	entProject, err := r.data.DB(ctx).Project.Query().Where(project.Name(name)).Select(project.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entProject.ID, nil
}

// GetProjectExistByCode checks project code existence.
func (r *projectRepo) GetProjectExistByCode(ctx context.Context, code string) (uint32, error) {
	entProject, err := r.data.DB(ctx).Project.Query().Where(project.Code(code)).Select(project.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entProject.ID, nil
}

// Update updates project data.
func (r *projectRepo) Update(ctx context.Context, g *pbCore.Project) (*pbCore.Project, error) {
	r.log.Infof("更新项目，项目信息：%v", g)
	if id, _ := r.GetProjectExistByName(ctx, g.GetName()); id > 0 && id != g.GetId() {
		return nil, fmt.Errorf("project name already exists")
	}
	if g.GetCode() != "" {
		if id, _ := r.GetProjectExistByCode(ctx, g.GetCode()); id > 0 && id != g.GetId() {
			return nil, fmt.Errorf("project code already exists")
		}
	}

	builder := r.data.DB(ctx).Project.UpdateOneID(g.GetId()).
		SetName(g.GetName()).
		SetNillableStatus(convert.EmptyToNil(int32(g.GetStatus()))).
		SetDescription(g.GetDescription()).
		ClearMembers()

	if g.GetCode() == "" {
		builder.ClearCode()
	} else {
		builder.SetCode(g.GetCode())
	}
	if g.GetOwnerId() == 0 {
		builder.ClearOwnerID()
	} else {
		builder.SetOwnerID(g.GetOwnerId())
	}
	if len(g.GetMemberIds()) > 0 {
		builder.AddMemberIDs(g.GetMemberIds()...)
	}

	res, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("更新项目失败，项目信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.FindByID(ctx, res.ID)
}

// UpdateStatus updates project status.
func (r *projectRepo) UpdateStatus(ctx context.Context, id uint32, status int32) error {
	r.log.Infof("更新项目状态，项目ID：%d，状态：%d", id, status)
	return r.data.DB(ctx).Project.UpdateOneID(id).SetStatus(status).Exec(ctx)
}

// FindByID finds project by id.
func (r *projectRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Project, error) {
	r.log.Infof("通过ID查询项目，ID：%d", id)
	res, err := r.data.DB(ctx).Project.Query().
		Where(project.IDEQ(id)).
		WithMembers().
		Only(ctx)
	if err != nil {
		r.log.Errorf("通过ID查询项目失败，ID：%d，错误：%v", id, err)
		if gen.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.convertProto(ctx, res), nil
}

// Delete soft-deletes project.
func (r *projectRepo) Delete(ctx context.Context, id uint32) error {
	r.log.Infof("删除项目，项目ID：%d", id)
	err := r.data.DB(ctx).Project.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if err != nil {
		r.log.Errorf("删除项目失败，项目ID：%d，错误：%v", id, err)
		return err
	}
	return nil
}

// CountProjects counts projects.
func (r *projectRepo) CountProjects(ctx context.Context, opts ...biz.ListOption) (int32, error) {
	r.log.Infof("查询项目数量，过滤条件：%v", opts)
	o := biz.ListOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.data.DB(ctx).Project.Query().
		Select(project.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		r.log.Errorf("查询项目数量失败，错误：%v", err)
		return 0, err
	}
	return int32(count), nil
}

// ListProjects lists projects.
func (r *projectRepo) ListProjects(ctx context.Context, opts ...biz.ListOption) ([]*pbCore.Project, error) {
	r.log.Infof("查询项目列表，分页选项：%v", opts)
	o := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	res, err := r.data.DB(ctx).Project.Query().
		Select(
			project.FieldID,
			project.FieldName,
			project.FieldCode,
			project.FieldOwnerID,
			project.FieldStatus,
			project.FieldDescription,
			project.FieldCreatedAt,
			project.FieldUpdatedAt,
		).
		Where(ents.ApplyFilter(o.Filter)).
		WithMembers().
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).
		Limit(o.Limit).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询项目列表失败，错误：%v", err)
		return nil, err
	}
	return convert.SliceToAny(res, func(item *gen.Project) *pbCore.Project {
		return r.convertProto(ctx, item)
	}), nil
}
