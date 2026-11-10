package data

import (
	"context"
	"time"

	pbenum "backend-service/api/common/enum"
	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/data/ent/gen"
	"backend-service/app/edai/service/internal/data/ent/gen/user"

	"backend-service/pkg/utils/convert"
)

type userRepo struct {
	data *Data
}

var _ interface {
	GetByUsername(context.Context, string) (*pb.User, string, error)
	GetByID(context.Context, uint32) (*pb.User, error)
	UpdateLastLogin(context.Context, uint32, time.Time) error
	Create(context.Context, *pb.User, string) (*pb.User, error)
	Count(context.Context) (int, error)
} = (*userRepo)(nil)

// NewUserRepo 创建账号仓库。
func NewUserRepo(data *Data) *userRepo { return &userRepo{data: data} }

func (r *userRepo) GetByUsername(ctx context.Context, username string) (*pb.User, string, error) {
	ctx = r.data.TenantContext(ctx)
	row, err := r.data.Ent().User.Query().
		Where(user.UsernameEQ(username)).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			// 返回 nil 而非错误，由 biz 层统一处理为「账号或密码错误」
			return nil, "", nil
		}
		return nil, "", err
	}
	return entUserToProto(row), row.PasswordHash, nil
}

func (r *userRepo) GetByID(ctx context.Context, id uint32) (*pb.User, error) {
	ctx = r.data.TenantContext(ctx)
	row, err := r.data.Ent().User.Get(ctx, id)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return entUserToProto(row), nil
}

func (r *userRepo) UpdateLastLogin(ctx context.Context, id uint32, at time.Time) error {
	ctx = r.data.TenantContext(ctx)
	return r.data.Ent().User.UpdateOneID(id).SetLastLoginAt(at).Exec(ctx)
}

func (r *userRepo) Create(ctx context.Context, u *pb.User, passwordHash string) (*pb.User, error) {
	ctx = r.data.TenantContext(ctx)
	b := r.data.Ent().User.Create().
		SetUsername(u.GetUsername()).
		SetPasswordHash(passwordHash).
		SetRole(nonEmpty(u.GetRole(), "viewer")).
		SetStatus(int32(pbenum.Status_STATUS_ENABLED))
	if dn := u.GetDisplayName(); dn != "" {
		b.SetDisplayName(dn)
	}
	if mail := u.GetEmail(); mail != "" {
		b.SetEmail(mail)
	}
	row, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entUserToProto(row), nil
}

func (r *userRepo) Count(ctx context.Context) (int, error) {
	ctx = r.data.TenantContext(ctx)
	return r.data.Ent().User.Query().Count(ctx)
}

func entUserToProto(row *gen.User) *pb.User {
	if row == nil {
		return nil
	}
	out := &pb.User{
		Id:          row.ID,
		Username:    row.Username,
		DisplayName: ptrIfNotEmpty(row.DisplayName),
		Email:       ptrIfNotEmpty(row.Email),
		Role:        row.Role,
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
	}
	if row.Status != nil {
		st := pbenum.Status(*row.Status)
		out.Status = &st
	}
	if row.LastLoginAt != nil {
		out.LastLoginAt = convert.ToPointer(row.LastLoginAt.Format(time.RFC3339))
	}
	return out
}

func nonEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
