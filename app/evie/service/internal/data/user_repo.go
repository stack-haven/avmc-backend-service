package data

import (
	"context"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/user"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

var _ biz.UserRepo = (*userRepo)(nil)

type userRepo struct {
	Data *Data
	log  *log.Helper
}

func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{Data: data, log: log.NewHelper(log.With(logger, "module", "data/user"))}
}

func (r *userRepo) Save(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	// TODO: implement
	return nil, kerrors.InternalServer("NOT_IMPLEMENTED", "user.Save not implemented")
}

func (r *userRepo) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	// TODO: implement
	return nil, kerrors.InternalServer("NOT_IMPLEMENTED", "user.Update not implemented")
}

func (r *userRepo) FindByID(ctx context.Context, id uint32) (*pbCore.User, error) {
	row, err := r.Data.DB(ctx).User.Query().Where(user.IDEQ(id)).Only(ctx)
	if gen.IsNotFound(err) {
		return nil, kerrors.NotFound("USER_NOT_FOUND", "用户不存在")
	}
	if err != nil {
		return nil, err
	}
	return entToProtoUser(row), nil
}

func (r *userRepo) ListUsers(ctx context.Context, opts ...listing.Option) ([]*pbCore.User, error) {
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	rows, err := r.Data.DB(ctx).User.Query().Offset(o.Offset).Limit(o.Limit).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(rows, entToProtoUser), nil
}

func (r *userRepo) CountUsers(ctx context.Context, opts ...listing.Option) (int32, error) {
	count, err := r.Data.DB(ctx).User.Query().Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *userRepo) ListByName(ctx context.Context, name string) ([]*pbCore.User, error) {
	return nil, kerrors.InternalServer("NOT_IMPLEMENTED", "user.ListByName not implemented")
}

func (r *userRepo) ListByPhone(ctx context.Context, phone string) ([]*pbCore.User, error) {
	return nil, kerrors.InternalServer("NOT_IMPLEMENTED", "user.ListByPhone not implemented")
}

func (r *userRepo) ListUsersByDept(ctx context.Context, deptID uint32, includeChildren bool, opts ...listing.Option) ([]*pbCore.User, error) {
	return nil, kerrors.InternalServer("NOT_IMPLEMENTED", "user.ListUsersByDept not implemented")
}

func (r *userRepo) CountUsersByDept(ctx context.Context, deptID uint32, includeChildren bool, opts ...listing.Option) (int32, error) {
	return 0, kerrors.InternalServer("NOT_IMPLEMENTED", "user.CountUsersByDept not implemented")
}

func (r *userRepo) ListAll(ctx context.Context) ([]*pbCore.User, error) {
	return nil, kerrors.InternalServer("NOT_IMPLEMENTED", "user.ListAll not implemented")
}

func (r *userRepo) ListPageSimple(ctx context.Context, opts ...listing.Option) ([]*pbCore.User, error) {
	return r.ListUsers(ctx, opts...)
}

func (r *userRepo) Delete(ctx context.Context, id uint32) error {
	return kerrors.InternalServer("NOT_IMPLEMENTED", "user.Delete not implemented")
}

func (r *userRepo) ExistByName(ctx context.Context, name string) (uint32, error) {
	return 0, nil
}

func (r *userRepo) ExistByPhone(ctx context.Context, phone string) (uint32, error) {
	return 0, nil
}

func (r *userRepo) ExistByEmail(ctx context.Context, email string) (uint32, error) {
	return 0, nil
}

func entToProtoUser(row *gen.User) *pbCore.User {
	if row == nil {
		return nil
	}
	return &pbCore.User{
		Id:       row.ID,
		Name:     row.Name,
		Realname: row.Realname,
		Nickname: row.Nickname,
		Email:    row.Email,
		Phone:    row.Phone,
		Avatar:   row.Avatar,
		Status:   nil,
	}
}
