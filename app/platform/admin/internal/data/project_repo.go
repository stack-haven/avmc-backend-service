package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/project"
	"backend-service/app/platform/admin/internal/data/ent/gen/user"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

var _ biz.ProjectRepo = (*projectRepo)(nil)

type projectRepo struct {
	BaseRepo
}

func NewProjectRepo(data *Data, logger log.Logger) biz.ProjectRepo {
	return &projectRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func (r *projectRepo) convertProto(res *gen.Project, ownerNames map[uint32]*string) *pbCore.Project {
	status := enum.Status(0)
	if res.Status != nil {
		status = enum.Status(*res.Status)
	}
	var ownerName *string
	if res.OwnerID != nil {
		ownerName = ownerNames[*res.OwnerID]
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
		OwnerName:   ownerName,
		Status:      convert.EmptyToNil(status),
		Description: res.Description,
		MemberIds:   memberIds,
		CreatedAt:   convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt:   convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

func (r *projectRepo) getOwnerNames(ctx context.Context, projects []*gen.Project) (map[uint32]*string, error) {
	ownerIDs := make([]uint32, 0, len(projects))
	seen := make(map[uint32]struct{}, len(projects))
	for _, item := range projects {
		if item.OwnerID == nil || *item.OwnerID == 0 {
			continue
		}
		if _, ok := seen[*item.OwnerID]; ok {
			continue
		}
		seen[*item.OwnerID] = struct{}{}
		ownerIDs = append(ownerIDs, *item.OwnerID)
	}
	if len(ownerIDs) == 0 {
		return map[uint32]*string{}, nil
	}

	owners, err := r.Data.DB(ctx).User.Query().
		Where(user.IDIn(ownerIDs...)).
		Select(user.FieldID, user.FieldName).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ownerNames := make(map[uint32]*string, len(owners))
	for _, owner := range owners {
		ownerNames[owner.ID] = owner.Name
	}
	return ownerNames, nil
}

func (r *projectRepo) validateUsersInTenant(ctx context.Context, ownerID uint32, memberIDs []uint32) error {
	ids := make([]uint32, 0, len(memberIDs)+1)
	seen := make(map[uint32]struct{}, len(memberIDs)+1)
	for _, id := range append(memberIDs, ownerID) {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	count, err := r.Data.DB(ctx).User.Query().
		Where(user.IDIn(ids...)).
		Count(ctx)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return pb.ErrorBadRequest("项目负责人或成员不存在或不属于当前域")
	}
	return nil
}

func (r *projectRepo) scopedProjectQuery(ctx context.Context) (*gen.ProjectQuery, error) {
	query := r.Data.DB(ctx).Project.Query()
	scope, err := r.resolveDataScopeUsers(ctx)
	if err != nil {
		return nil, err
	}
	if scope.all {
		return query, nil
	}
	if len(scope.userIDs) == 0 {
		return query.Where(project.IDEQ(0)), nil
	}
	return query.Where(project.Or(
		project.OwnerIDIn(scope.userIDs...),
		project.HasMembersWith(user.IDIn(scope.userIDs...)),
	)), nil
}

func (r *projectRepo) Save(ctx context.Context, g *pbCore.Project) (*pbCore.Project, error) {
	if g == nil || g.GetName() == "" {
		return nil, pb.ErrorProjectNameCannotBeEmpty("项目名称不能为空")
	}
	r.Log.Infof("保存项目: %s", g.GetName())
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	if err := r.validateUsersInTenant(ctx, g.GetOwnerId(), g.GetMemberIds()); err != nil {
		return nil, err
	}
	id, err := r.GetProjectExistByName(ctx, g.GetName())
	if err != nil {
		return nil, fmt.Errorf("checking project name uniqueness: %w", err)
	}
	if id > 0 {
		return nil, pb.ErrorProjectAlreadyExists("项目名称已存在")
	}
	if g.GetCode() != "" {
		id, err = r.GetProjectExistByCode(ctx, g.GetCode())
		if err != nil {
			return nil, fmt.Errorf("checking project code uniqueness: %w", err)
		}
		if id > 0 {
			return nil, pb.ErrorProjectCodeAlreadyExists("项目编码已存在")
		}
	}

	builder := r.Data.DB(ctx).Project.Create().
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
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorProjectAlreadyExists("项目名称或编码已存在")
		}
		return nil, err
	}
	return r.FindByID(ctx, res.ID)
}

func (r *projectRepo) GetProjectExistByName(ctx context.Context, name string) (uint32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	entProject, err := r.Data.DB(ctx).Project.Query().Where(project.Name(name)).Select(project.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entProject.ID, nil
}

func (r *projectRepo) GetProjectExistByCode(ctx context.Context, code string) (uint32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	entProject, err := r.Data.DB(ctx).Project.Query().Where(project.Code(code)).Select(project.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entProject.ID, nil
}

func (r *projectRepo) Update(ctx context.Context, g *pbCore.Project) (*pbCore.Project, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" {
		return nil, pb.ErrorProjectInvalidId("项目ID和名称不能为空")
	}
	r.Log.Infof("更新项目: %d", g.GetId())
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	if err := r.validateUsersInTenant(ctx, g.GetOwnerId(), g.GetMemberIds()); err != nil {
		return nil, err
	}
	id, err := r.GetProjectExistByName(ctx, g.GetName())
	if err != nil {
		return nil, fmt.Errorf("checking project name uniqueness: %w", err)
	}
	if id > 0 && id != g.GetId() {
		return nil, pb.ErrorProjectAlreadyExists("项目名称已存在")
	}
	if g.GetCode() != "" {
		id, err = r.GetProjectExistByCode(ctx, g.GetCode())
		if err != nil {
			return nil, fmt.Errorf("checking project code uniqueness: %w", err)
		}
		if id > 0 && id != g.GetId() {
			return nil, pb.ErrorProjectCodeAlreadyExists("项目编码已存在")
		}
	}

	builder := r.Data.DB(ctx).Project.UpdateOneID(g.GetId()).
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
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorProjectAlreadyExists("项目名称或编码已存在")
		}
		if gen.IsNotFound(err) {
			return nil, pb.ErrorProjectNotFound("项目不存在")
		}
		return nil, err
	}
	return r.FindByID(ctx, res.ID)
}

func (r *projectRepo) UpdateStatus(ctx context.Context, id uint32, status int32) error {
	r.Log.Infof("更新项目状态 ID: %d, status: %d", id, status)
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	err := r.Data.DB(ctx).Project.UpdateOneID(id).SetStatus(status).Exec(ctx)
	if gen.IsNotFound(err) {
		return pb.ErrorProjectNotFound("项目不存在")
	}
	return err
}

func (r *projectRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Project, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	query, err := r.scopedProjectQuery(ctx)
	if err != nil {
		return nil, err
	}
	res, err := query.
		Where(project.IDEQ(id)).
		WithMembers().
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorProjectNotFound("项目不存在")
		}
		return nil, err
	}
	ownerNames, err := r.getOwnerNames(ctx, []*gen.Project{res})
	if err != nil {
		return nil, err
	}
	return r.convertProto(res, ownerNames), nil
}

func (r *projectRepo) Delete(ctx context.Context, id uint32) error {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	err := r.Data.DB(ctx).Project.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if gen.IsNotFound(err) {
		return pb.ErrorProjectNotFound("项目不存在")
	}
	return err
}

func (r *projectRepo) CountProjects(ctx context.Context, opts ...listing.Option) (int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	query, err := r.scopedProjectQuery(ctx)
	if err != nil {
		return 0, err
	}
	count, err := query.
		Select(project.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *projectRepo) ListProjects(ctx context.Context, opts ...listing.Option) ([]*pbCore.Project, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	query, err := r.scopedProjectQuery(ctx)
	if err != nil {
		return nil, err
	}
	res, err := query.
		Select(project.FieldID, project.FieldName, project.FieldCode, project.FieldOwnerID, project.FieldStatus, project.FieldDescription, project.FieldCreatedAt, project.FieldUpdatedAt).
		Where(ents.ApplyFilter(o.Filter)).
		WithMembers().
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ownerNames, err := r.getOwnerNames(ctx, res)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, func(item *gen.Project) *pbCore.Project {
		return r.convertProto(item, ownerNames)
	}), nil
}
