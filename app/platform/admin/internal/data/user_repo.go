package data

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"context"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/user"
	"backend-service/pkg/aip/listing"
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

// entToProto 将 ent.User 转换为 pbCore.User（读出层）
func (r *userRepo) entToProto(e *gen.User) *pbCore.User {
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
	return &pbCore.User{
		Id:          e.ID,
		Name:        e.Name,
		Nickname:    e.Nickname,
		Realname:    e.Realname,
		Birthday:    convert.TimeValueToString(e.Birthday, time.DateOnly),
		Gender:      &gender,
		Phone:       e.Phone,
		Email:       e.Email,
		Avatar:      e.Avatar,
		Description: e.Description,
		Status:      &status,
		CreatedAt:   convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:   convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
	}
}

// protoToEnt 将 pbCore.User 转换为 ent.User（写入层）
func (r *userRepo) protoToEnt(g *pbCore.User) *gen.User {
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
		Status:      (*int32)(g.Status),
	}
}

// Save 保存用户
func (r *userRepo) Save(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	if g == nil || g.Name == nil || g.Password == nil {
		return nil, pb.ErrorBadRequest("用户名和密码不能为空")
	}
	r.Log.Infof("保存用户")
	ent := r.protoToEnt(g)

	res, err := r.Data.DB(ctx).User.Create().
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
		SetNillableStatus(ent.Status).
		Save(ctx)
	if err != nil {
		r.Log.Errorf("保存用户失败: %v", err)
		// 唯一约束冲突友好提示
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorUserAlreadyExists("用户名、邮箱或手机号已存在")
		}
		return nil, err
	}
	return r.entToProto(res), nil
}

// Update 更新用户
func (r *userRepo) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	if g == nil || g.GetId() == 0 {
		return nil, pb.ErrorUserInvalidId("用户ID不能为空")
	}
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	r.Log.Infof("更新用户 ID: %d", g.GetId())
	ent := r.protoToEnt(g)
	builder := r.Data.DB(ctx).User.UpdateOneID(g.GetId())

	if g.Password != nil {
		builder = builder.SetPassword(*g.Password)
	}

	res, err := builder.
		SetNillableName(ent.Name).
		SetNillableEmail(ent.Email).
		SetNillableNickname(ent.Nickname).
		SetNillableRealname(ent.Realname).
		SetNillableBirthday(ent.Birthday).
		SetNillableGender(ent.Gender).
		SetNillablePhone(ent.Phone).
		SetNillableAvatar(ent.Avatar).
		SetNillableDescription(ent.Description).
		SetNillableStatus(ent.Status).
		Save(ctx)
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
	return r.entToProto(res), nil
}

// FindByID 通过 ID 查询 — 显式 Select 排除 password、deleted_at 等敏感/非必要字段
func (r *userRepo) FindByID(ctx context.Context, id uint32) (*pbCore.User, error) {
	r.Log.Infof("查询用户 ID: %d", id)
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).User.Query().
		Select(
			user.FieldID, user.FieldName,
			user.FieldNickname, user.FieldRealname,
			user.FieldBirthday, user.FieldGender,
			user.FieldPhone, user.FieldEmail,
			user.FieldAvatar, user.FieldDescription,
			user.FieldStatus, user.FieldTenantID,
			user.FieldCreatedAt, user.FieldUpdatedAt,
		).
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
func (r *userRepo) ListByName(ctx context.Context, name string) ([]*pbCore.User, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).User.Query().
		Select(user.FieldID, user.FieldName).
		Where(user.NameContains(name)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(res, r.entToProto), nil
}

// ListByPhone 按手机号查询
func (r *userRepo) ListByPhone(ctx context.Context, phone string) ([]*pbCore.User, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).User.Query().
		Where(user.PhoneEQ(phone)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(res, r.entToProto), nil
}

// ListAll 查询所有用户
func (r *userRepo) ListAll(ctx context.Context) ([]*pbCore.User, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).User.Query().
		Select(user.FieldID, user.FieldName).
		Order(gen.Desc(user.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(res, r.entToProto), nil
}

// ListPageSimple 用户简单列表分页
func (r *userRepo) ListPageSimple(ctx context.Context, opts ...listing.Option) ([]*pbCore.User, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	res, err := r.Data.DB(ctx).User.Query().
		Select(user.FieldID, user.FieldName).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		Order(gen.Desc(user.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(res, r.entToProto), nil
}

// ListUsers 用户完整列表分页
func (r *userRepo) ListUsers(ctx context.Context, opts ...listing.Option) ([]*pbCore.User, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	res, err := r.Data.DB(ctx).User.Query().
		Select(
			user.FieldID, user.FieldName, user.FieldEmail,
			user.FieldNickname, user.FieldRealname, user.FieldBirthday,
			user.FieldGender, user.FieldPhone, user.FieldAvatar,
			user.FieldStatus, user.FieldTenantID,
			user.FieldCreatedAt, user.FieldUpdatedAt,
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

// CountUsers 用户计数
func (r *userRepo) CountUsers(ctx context.Context, opts ...listing.Option) (int32, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return 0, err
	}
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).User.Query().
		Select(user.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

// Delete 软删除
func (r *userRepo) Delete(ctx context.Context, id uint32) error {
	if _, err := requireTenantID(ctx); err != nil {
		return err
	}
	err := r.Data.DB(ctx).User.UpdateOneID(id).
		SetDeletedAt(time.Now()).
		Exec(ctx)
	if gen.IsNotFound(err) {
		return pb.ErrorUserNotFound("用户不存在")
	}
	return err
}

// ExistByName 检查用户名是否存在
func (r *userRepo) ExistByName(ctx context.Context, name string) (uint32, error) {
	if _, err := requireTenantID(ctx); err != nil {
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
	if _, err := requireTenantID(ctx); err != nil {
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
	if _, err := requireTenantID(ctx); err != nil {
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
