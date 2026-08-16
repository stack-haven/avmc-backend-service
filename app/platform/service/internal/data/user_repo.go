package data

import (
	pbEnum "backend-service/api/common/enum"
	pb "backend-service/api/platform/service/v1"
	"context"
	"time"

	"github.com/go-kratos/aip-go/ents"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/platform/service/internal/biz"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/dept"
	"backend-service/app/platform/service/internal/data/ent/gen/role"
	"backend-service/app/platform/service/internal/data/ent/gen/user"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"
)

var _ biz.UserRepo = (*userRepo)(nil)

// userRepo 用户仓库
type userRepo struct {
	BaseRepo // 注入 Data + Log
}

// NewUserRepo 创建用户仓库
func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// entToProto 将 ent.User 转换为 pb.User（读出层）
func (r *userRepo) entToProto(e *gen.User) *pb.User {
	if e == nil {
		return nil
	}
	status := pbEnum.Status(0)
	if e.Status != nil {
		status = pbEnum.Status(*e.Status)
	}
	gender := pbEnum.Gender(0)
	if e.Gender != nil {
		gender = pbEnum.Gender(*e.Gender)
	}
	return &pb.User{
		Id:            e.ID,
		Name:          e.Name,
		Nickname:      e.Nickname,
		Realname:      e.Realname,
		Birthday:      convert.TimeValueToString(e.Birthday, time.DateOnly),
		Gender:        &gender,
		Phone:         e.Phone,
		Email:         e.Email,
		Avatar:        e.Avatar,
		Description:   e.Description,
		Status:        &status,
		CreatedAt:     convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:     convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
		RoleIds:       userRoleIDs(e),
		IsTenantAdmin: userHasTenantAdminRole(e),
		DeptId:        e.DeptID,
	}
}

// protoToEnt 将 pb.User 转换为 ent.User（写入层）
func (r *userRepo) protoToEnt(g *pb.User) *gen.User {
	return &gen.User{
		ID:          g.GetId(),
		Name:        g.Name,
		Password:    g.Password,
		Nickname:    g.Nickname,
		Realname:    g.Realname,
		Birthday:    convert.StringValueToTime(g.Birthday, time.DateOnly),
		Gender:      (*int32)(g.Gender),
		Phone:       g.Phone,
		Email:       g.Email,
		Avatar:      g.Avatar,
		Description: g.Description,
		DeptID:      g.DeptId,
		Status:      (*int32)(g.Status),
		Edges: gen.UserEdges{
			Roles: roleEntities(g.GetRoleIds()),
		},
	}
}

func roleEntities(ids []uint32) []*gen.Role {
	items := make([]*gen.Role, 0, len(ids))
	for _, id := range ids {
		items = append(items, &gen.Role{ID: id})
	}
	return items
}

func userRoleIDs(item *gen.User) []uint32 {
	if item == nil || len(item.Edges.Roles) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(item.Edges.Roles))
	for _, roleItem := range item.Edges.Roles {
		if roleItem != nil {
			ids = append(ids, roleItem.ID)
		}
	}
	return ids
}

func userHasTenantAdminRole(item *gen.User) bool {
	if item == nil {
		return false
	}
	for _, roleItem := range item.Edges.Roles {
		if roleItem != nil && roleItem.IsTenantAdmin && roleItem.Status != nil &&
			*roleItem.Status == int32(pbEnum.Status_STATUS_ENABLED) {
			return true
		}
	}
	return false
}

func (r *userRepo) validateRoleIDs(ctx context.Context, client *gen.Client, roleIDs []uint32) error {
	ids := uniqueUint32(roleIDs)
	if len(ids) == 0 {
		return nil
	}
	count, err := client.Role.Query().
		Where(
			role.IDIn(ids...),
			role.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
		).
		Count(ctx)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return pb.ErrorBadRequest("存在无效、已禁用或不属于当前租户的角色")
	}
	return nil
}

func (r *userRepo) validateDeptID(ctx context.Context, client *gen.Client, deptID uint32) error {
	if deptID == 0 {
		return nil
	}
	exists, err := client.Dept.Query().Where(dept.IDEQ(deptID)).Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return pb.ErrorBadRequest("所属部门无效或不属于当前租户")
	}
	return nil
}

func uniqueUint32(values []uint32) []uint32 {
	result := make([]uint32, 0, len(values))
	seen := make(map[uint32]struct{}, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// Save 保存用户
func (r *userRepo) Save(ctx context.Context, g *pb.User) (*pb.User, error) {
	if g == nil || g.Name == nil || g.Password == nil {
		return nil, pb.ErrorBadRequest("用户名和密码不能为空")
	}
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	r.Log.Infof("保存用户")
	ent := r.protoToEnt(g)
	if err := r.validateRoleIDs(ctx, r.Data.DB(ctx), g.GetRoleIds()); err != nil {
		return nil, err
	}
	if err := r.validateDeptID(ctx, r.Data.DB(ctx), g.GetDeptId()); err != nil {
		return nil, err
	}

	builder := r.Data.DB(ctx).User.Create().
		SetName(*ent.Name).
		SetPassword(*ent.Password).
		SetNillableEmail(ent.Email).
		SetNillableNickname(ent.Nickname).
		SetNillableRealname(ent.Realname).
		SetNillableBirthday(ent.Birthday).
		SetNillableGender(ent.Gender).
		SetNillablePhone(ent.Phone).
		SetNillableAvatar(ent.Avatar).
		SetNillableDescription(ent.Description).
		SetNillableDeptID(ent.DeptID).
		SetNillableStatus(ent.Status)
	if len(g.GetRoleIds()) > 0 {
		builder.AddRoleIDs(uniqueUint32(g.GetRoleIds())...)
	}
	res, err := builder.Save(ctx)
	if err != nil {
		r.Log.Errorf("保存用户失败: %v", err)
		// 唯一约束冲突友好提示
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorUserAlreadyExists("用户名、邮箱或手机号已存在")
		}
		return nil, err
	}
	res, err = r.loadUserWithRoles(ctx, r.Data.DB(ctx), res.ID)
	if err != nil {
		return nil, err
	}
	if len(g.GetRoleIds()) > 0 {
		r.bumpTenantAuthorizationVersion(ctx, tenantID)
		SyncUserRoles(ctx, r.Data.DB(ctx), r.Data.authorizer, tenantID, res.ID, g.GetRoleIds())
	}
	return r.entToProto(res), nil
}

// Update 更新用户
func (r *userRepo) Update(ctx context.Context, g *pb.User) (*pb.User, error) {
	if g == nil || g.GetId() == 0 {
		return nil, pb.ErrorUserInvalidId("用户ID不能为空")
	}
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	r.Log.Infof("更新用户 ID: %d", g.GetId())
	ent := r.protoToEnt(g)
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)
	current, err := findUserForUpdate(ctx, tx.Client(), g.GetId())
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorUserNotFound("用户不存在")
		}
		return nil, err
	}
	if err = r.validateRoleIDs(ctx, tx.Client(), g.GetRoleIds()); err != nil {
		return nil, err
	}
	if err = r.validateDeptID(ctx, tx.Client(), g.GetDeptId()); err != nil {
		return nil, err
	}
	if err = r.protectLastTenantAdmin(ctx, tx.Client(), current, g.GetRoleIds(), g.Status, false); err != nil {
		return nil, err
	}
	builder := tx.User.UpdateOneID(g.GetId())

	if g.Password != nil {
		builder = builder.SetPassword(*g.Password)
	}

	builder = builder.
		SetNillableName(ent.Name).
		SetNillableEmail(ent.Email).
		SetNillableNickname(ent.Nickname).
		SetNillableRealname(ent.Realname).
		SetNillableBirthday(ent.Birthday).
		SetNillableGender(ent.Gender).
		SetNillablePhone(ent.Phone).
		SetNillableAvatar(ent.Avatar).
		SetNillableDescription(ent.Description).
		SetNillableDeptID(ent.DeptID).
		SetNillableStatus(ent.Status).
		ClearRoles()
	if len(g.GetRoleIds()) > 0 {
		builder.AddRoleIDs(uniqueUint32(g.GetRoleIds())...)
	}
	res, err := builder.Save(ctx)
	if err != nil {
		r.Log.Errorf("更新用户失败: %v", err)
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorUserAlreadyExists("用户名、邮箱或手机号已被使用")
		}
		if gen.IsNotFound(err) {
			return nil, pb.ErrorUserNotFound("用户不存在")
		}
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	res, err = r.loadUserWithRoles(ctx, r.Data.DB(ctx), res.ID)
	if err != nil {
		return nil, err
	}
	r.bumpTenantAuthorizationVersion(ctx, tenantID)
	SyncUserRoles(ctx, r.Data.DB(ctx), r.Data.authorizer, tenantID, g.GetId(), g.GetRoleIds())
	return r.entToProto(res), nil
}

func (r *userRepo) loadUserWithRoles(ctx context.Context, client *gen.Client, id uint32) (*gen.User, error) {
	return client.User.Query().
		Where(user.IDEQ(id)).
		WithRoles().
		Only(ctx)
}

// FindByID 通过 ID 查询 — 显式 Select 排除 password、deleted_at 等敏感/非必要字段
func (r *userRepo) FindByID(ctx context.Context, id uint32) (*pb.User, error) {
	r.Log.Infof("查询用户 ID: %d", id)
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	query, err := r.scopedUserQuery(ctx)
	if err != nil {
		return nil, err
	}
	res, err := query.
		Select(
			user.FieldID, user.FieldName,
			user.FieldNickname, user.FieldRealname,
			user.FieldBirthday, user.FieldGender,
			user.FieldPhone, user.FieldEmail,
			user.FieldAvatar, user.FieldDescription,
			user.FieldStatus, user.FieldTenantID, user.FieldDeptID,
			user.FieldCreatedAt, user.FieldUpdatedAt,
		).
		WithRoles().
		Where(user.IDEQ(id)).
		Only(ctx)
	if err != nil {
		r.Log.Errorf("查询用户失败 ID: %d, err: %v", id, err)
		if gen.IsNotFound(err) {
			return nil, pb.ErrorUserNotFound("用户不存在")
		}
		return nil, err
	}
	return r.entToProto(res), nil
}

// ListByName 按用户名模糊查询
func (r *userRepo) ListByName(ctx context.Context, name string) ([]*pb.User, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	query, err := r.scopedUserQuery(ctx)
	if err != nil {
		return nil, err
	}
	res, err := query.
		Select(user.FieldID, user.FieldName).
		WithRoles().
		Where(user.NameContains(name)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.entToProto), nil
}

// ListByPhone 按手机号查询
func (r *userRepo) ListByPhone(ctx context.Context, phone string) ([]*pb.User, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	query, err := r.scopedUserQuery(ctx)
	if err != nil {
		return nil, err
	}
	res, err := query.
		Where(user.PhoneEQ(phone)).
		WithRoles().
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.entToProto), nil
}

// ListAll 查询所有用户
func (r *userRepo) ListAll(ctx context.Context) ([]*pb.User, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	query, err := r.scopedUserQuery(ctx)
	if err != nil {
		return nil, err
	}
	res, err := query.
		Select(user.FieldID, user.FieldName).
		WithRoles().
		Order(gen.Desc(user.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.entToProto), nil
}

// ListPageSimple 用户简单列表分页
func (r *userRepo) ListPageSimple(ctx context.Context, opts ...listing.Option) ([]*pb.User, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	query, err := r.scopedUserQuery(ctx)
	if err != nil {
		return nil, err
	}
	res, err := query.
		Select(user.FieldID, user.FieldName).
		WithRoles().
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		Order(gen.Desc(user.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.entToProto), nil
}

// ListUsers 用户完整列表分页
func (r *userRepo) ListUsers(ctx context.Context, opts ...listing.Option) ([]*pb.User, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	query, err := r.scopedUserQuery(ctx)
	if err != nil {
		return nil, err
	}
	res, err := query.
		Select(
			user.FieldID, user.FieldName, user.FieldEmail,
			user.FieldNickname, user.FieldRealname, user.FieldBirthday,
			user.FieldGender, user.FieldPhone, user.FieldAvatar,
			user.FieldStatus, user.FieldTenantID, user.FieldDeptID,
			user.FieldCreatedAt, user.FieldUpdatedAt,
		).
		WithRoles().
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.entToProto), nil
}

// CountUsers 用户计数
func (r *userRepo) CountUsers(ctx context.Context, opts ...listing.Option) (int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	query, err := r.scopedUserQuery(ctx)
	if err != nil {
		return 0, err
	}
	count, err := query.
		Select(user.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *userRepo) deptScopeIDs(ctx context.Context, deptID uint32, includeChildren bool) ([]uint32, error) {
	if deptID == 0 {
		return nil, nil
	}
	items, err := r.Data.DB(ctx).Dept.Query().
		Select(dept.FieldID, dept.FieldParentID, dept.FieldAncestors).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := []uint32{deptID}
	found := false
	parents := make(map[uint32]uint32, len(items))
	for _, item := range items {
		if item.ParentID != nil {
			parents[item.ID] = *item.ParentID
		}
		if item.ID == deptID {
			found = true
		}
	}
	if !found {
		return nil, pb.ErrorBadRequest("筛选部门不存在或不属于当前租户")
	}
	if includeChildren {
		for _, item := range items {
			current := parents[item.ID]
			visited := map[uint32]struct{}{item.ID: {}}
			for current != 0 {
				if current == deptID {
					ids = append(ids, item.ID)
					break
				}
				if _, exists := visited[current]; exists {
					break
				}
				visited[current] = struct{}{}
				current = parents[current]
			}
		}
	}
	return uniqueUint32(ids), nil
}

func (r *userRepo) ListUsersByDept(ctx context.Context, deptID uint32, includeChildren bool, opts ...listing.Option) ([]*pb.User, error) {
	if deptID == 0 {
		return r.ListUsers(ctx, opts...)
	}
	ids, err := r.deptScopeIDs(ctx, deptID, includeChildren)
	if err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	query, err := r.scopedUserQuery(ctx)
	if err != nil {
		return nil, err
	}
	res, err := query.
		Select(user.FieldID, user.FieldName, user.FieldEmail, user.FieldNickname, user.FieldRealname, user.FieldBirthday, user.FieldGender, user.FieldPhone, user.FieldAvatar, user.FieldStatus, user.FieldTenantID, user.FieldDeptID, user.FieldCreatedAt, user.FieldUpdatedAt).
		WithRoles().
		Where(ents.ApplyFilter(o.Filter), user.DeptIDIn(ids...)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.entToProto), nil
}

func (r *userRepo) CountUsersByDept(ctx context.Context, deptID uint32, includeChildren bool, opts ...listing.Option) (int32, error) {
	if deptID == 0 {
		return r.CountUsers(ctx, opts...)
	}
	ids, err := r.deptScopeIDs(ctx, deptID, includeChildren)
	if err != nil {
		return 0, err
	}
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	query, err := r.scopedUserQuery(ctx)
	if err != nil {
		return 0, err
	}
	count, err := query.Where(ents.ApplyFilter(o.Filter), user.DeptIDIn(ids...)).Count(ctx)
	return int32(count), err
}

func (r *userRepo) scopedUserQuery(ctx context.Context) (*gen.UserQuery, error) {
	query := r.Data.DB(ctx).User.Query()
	scope, err := r.resolveDataScopeUsers(ctx)
	if err != nil {
		return nil, err
	}
	if scope.all {
		return query, nil
	}
	if len(scope.userIDs) == 0 {
		return query.Where(user.IDEQ(authn.GetAuthUserID(ctx))), nil
	}
	return query.Where(user.IDIn(scope.userIDs...)), nil
}

// Delete 软删除
func (r *userRepo) Delete(ctx context.Context, id uint32) error {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx, r.Log)
	current, err := findUserForUpdate(ctx, tx.Client(), id)
	if err != nil {
		if gen.IsNotFound(err) {
			return pb.ErrorUserNotFound("用户不存在")
		}
		return err
	}
	if err = r.protectLastTenantAdmin(ctx, tx.Client(), current, nil, nil, true); err != nil {
		return err
	}
	err = tx.User.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if gen.IsNotFound(err) {
		return pb.ErrorUserNotFound("用户不存在")
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *userRepo) protectLastTenantAdmin(
	ctx context.Context,
	client *gen.Client,
	current *gen.User,
	nextRoleIDs []uint32,
	nextStatus *pbEnum.Status,
	deleting bool,
) error {
	if current == nil || !userHasTenantAdminRole(current) ||
		current.Status == nil || *current.Status != int32(pbEnum.Status_STATUS_ENABLED) {
		return nil
	}
	nextIsAdmin := false
	if !deleting && len(nextRoleIDs) > 0 {
		exists, err := client.Role.Query().
			Where(
				role.IDIn(uniqueUint32(nextRoleIDs)...),
				role.TenantIDEQ(current.TenantID),
				role.IsTenantAdminEQ(true),
			).
			Exist(ctx)
		if err != nil {
			return err
		}
		nextIsAdmin = exists
	}
	nextEnabled := !deleting && nextStatus != nil &&
		int32(*nextStatus) == int32(pbEnum.Status_STATUS_ENABLED)
	if nextIsAdmin && nextEnabled {
		return nil
	}
	adminQuery := client.User.Query().
		Where(
			user.TenantIDEQ(current.TenantID),
			user.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
			user.HasRolesWith(
				role.IsTenantAdminEQ(true),
				role.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
			),
		)
	admins, err := adminQuery.Clone().ForUpdate().All(ctx)
	if isSelectForUpdateUnsupported(err) {
		admins, err = adminQuery.All(ctx)
	}
	if err != nil {
		return err
	}
	if len(admins) <= 1 {
		return kerrors.Conflict("LAST_TENANT_ADMIN_REQUIRED", "租户必须保留至少一个已启用的管理员")
	}
	return nil
}

func findUserForUpdate(ctx context.Context, client *gen.Client, id uint32) (*gen.User, error) {
	query := client.User.Query().Where(user.IDEQ(id)).WithRoles()
	item, err := query.Clone().ForUpdate().Only(ctx)
	if isSelectForUpdateUnsupported(err) {
		return query.Only(ctx)
	}
	return item, err
}

// ExistByName 检查用户名是否存在
func (r *userRepo) ExistByName(ctx context.Context, name string) (uint32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	entUser, err := r.Data.DB(ctx).User.Query().
		Where(user.Name(name)).
		Select(user.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entUser.ID, nil
}

// ExistByPhone 检查手机号是否存在
func (r *userRepo) ExistByPhone(ctx context.Context, phone string) (uint32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	entUser, err := r.Data.DB(ctx).User.Query().
		Where(user.Phone(phone)).
		Select(user.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entUser.ID, nil
}

// ExistByEmail 检查邮箱是否存在
func (r *userRepo) ExistByEmail(ctx context.Context, email string) (uint32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	entUser, err := r.Data.DB(ctx).User.Query().
		Where(user.Email(email)).
		Select(user.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entUser.ID, nil
}
