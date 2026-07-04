package biz

import (
	pbEnum "backend-service/api/common/enum"
	pb "backend-service/api/platform/admin/v1"
	"context"

	pbCore "backend-service/api/core/service/v1"

	"backend-service/pkg/aip/listing"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"
	"backend-service/pkg/utils/crypto"

	"github.com/go-kratos/kratos/v2/log"
)

// UserRepo is a User repo.
type UserRepo interface {
	Save(context.Context, *pbCore.User) (*pbCore.User, error)
	Update(context.Context, *pbCore.User) (*pbCore.User, error)
	FindByID(context.Context, uint32) (*pbCore.User, error)
	ListByName(context.Context, string) ([]*pbCore.User, error)
	ListByPhone(context.Context, string) ([]*pbCore.User, error)
	ListUsers(context.Context, ...listing.Option) ([]*pbCore.User, error)
	CountUsers(context.Context, ...listing.Option) (int32, error)
	ListAll(context.Context) ([]*pbCore.User, error)
	ListPageSimple(context.Context, ...listing.Option) ([]*pbCore.User, error)
	Delete(context.Context, uint32) error
	ExistByName(context.Context, string) (uint32, error)
	ExistByPhone(context.Context, string) (uint32, error)
	ExistByEmail(context.Context, string) (uint32, error)
}

type TenantAdminPolicy interface {
	SetMembership(context.Context, uint32, uint32, bool) error
}

// UserUsecase 用户业务逻辑
type UserUsecase struct {
	repo     UserRepo
	sessions SessionRepo
	policy   TenantAdminPolicy
	log      *log.Helper
}

// NewUserUsecase new a User usecase.
func NewUserUsecase(repo UserRepo, sessions SessionRepo, policy TenantAdminPolicy, logger log.Logger) *UserUsecase {
	return &UserUsecase{repo: repo, sessions: sessions, policy: policy, log: log.NewHelper(logger)}
}

// Create 创建用户 — 业务逻辑：密码哈希、默认值
func (uc *UserUsecase) Create(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	if g == nil || g.Name == nil || *g.Name == "" {
		return nil, pb.ErrorUserInvalidId("用户名不能为空")
	}
	if g.Password == nil || *g.Password == "" {
		return nil, pb.ErrorUserIncorrectPassword("密码不能为空")
	}
	uc.log.WithContext(ctx).Infof("CreateUser")

	if err := ValidatePassword(*g.Password); err != nil {
		return nil, err
	}
	hash, err := crypto.HashPassword(*g.Password)
	if err != nil {
		uc.log.Errorf("密码哈希失败: %v", err)
		return nil, ErrPasswordHashFailed
	}
	g.Password = &hash

	created, err := uc.repo.Save(ctx, g)
	if err != nil {
		return nil, err
	}
	if isEnabledTenantAdmin(created) {
		if err = uc.setAdminMembership(ctx, created.GetId(), true); err != nil {
			return nil, err
		}
	}
	return created, nil
}

// Get 获取用户
func (uc *UserUsecase) Get(ctx context.Context, id uint32) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("GetUser: %v", id)
	return uc.repo.FindByID(ctx, id)
}

// Update 更新用户 — 业务逻辑：密码变更时重哈希
func (uc *UserUsecase) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("UpdateUser: %v", g.GetId())

	passwordChanged := g.Password != nil
	if passwordChanged {
		if err := ValidatePassword(*g.Password); err != nil {
			return nil, err
		}
		hash, err := crypto.HashPassword(*g.Password)
		if err != nil {
			uc.log.Errorf("密码哈希失败: %v", err)
			return nil, ErrPasswordHashFailed
		}
		g.Password = &hash
	}

	current, err := uc.repo.FindByID(ctx, g.GetId())
	if err != nil {
		return nil, err
	}
	updated, err := uc.repo.Update(ctx, g)
	if err != nil {
		return nil, err
	}
	rolesChanged := !sameUint32Set(current.GetRoleIds(), updated.GetRoleIds())
	if (passwordChanged || rolesChanged) && uc.sessions != nil {
		if err := uc.sessions.RevokeUser(ctx, g.GetId()); err != nil {
			return nil, err
		}
	}
	if isEnabledTenantAdmin(current) != isEnabledTenantAdmin(updated) {
		if err = uc.setAdminMembership(ctx, updated.GetId(), isEnabledTenantAdmin(updated)); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func isEnabledTenantAdmin(user *pbCore.User) bool {
	return user != nil &&
		user.GetIsTenantAdmin() &&
		user.GetStatus() == pbEnum.Status_STATUS_ENABLED
}

// ListPageSimple 用户简单列表分页
func (uc *UserUsecase) ListPageSimple(ctx context.Context, opts ...listing.Option) ([]*pbCore.User, error) {
	return uc.repo.ListPageSimple(ctx, opts...)
}

// Delete 删除用户
func (uc *UserUsecase) Delete(ctx context.Context, id uint32) error {
	current, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	if uc.sessions != nil {
		if err := uc.sessions.RevokeUser(ctx, id); err != nil {
			return err
		}
	}
	if current.GetIsTenantAdmin() {
		if err := uc.setAdminMembership(ctx, id, false); err != nil {
			return err
		}
	}
	return nil
}

func (uc *UserUsecase) setAdminMembership(ctx context.Context, userID uint32, enabled bool) error {
	if uc.policy == nil {
		return nil
	}
	claims, ok := authn.AuthClaimsFromContext(ctx)
	if !ok {
		return pb.ErrorBadRequest("缺少租户认证上下文")
	}
	tenantID := convert.StringToUnit32(claims.GetTenant())
	if tenantID == 0 {
		return pb.ErrorBadRequest("租户认证上下文无效")
	}
	return uc.policy.SetMembership(ctx, tenantID, userID, enabled)
}

func sameUint32Set(left, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	values := make(map[uint32]struct{}, len(left))
	for _, value := range left {
		values[value] = struct{}{}
	}
	for _, value := range right {
		if _, ok := values[value]; !ok {
			return false
		}
	}
	return true
}

// UpdateStatus 更新用户状态
func (uc *UserUsecase) UpdateStatus(ctx context.Context, id uint32, status int32) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("UpdateStatus: %d %d", id, status)
	g, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	wasEnabledAdmin := isEnabledTenantAdmin(g)
	s := pbEnum.Status(status)
	g.Status = &s
	updated, err := uc.repo.Update(ctx, g)
	if err != nil {
		return nil, err
	}
	if status != int32(pbEnum.Status_STATUS_ENABLED) && uc.sessions != nil {
		if err := uc.sessions.RevokeUser(ctx, id); err != nil {
			return nil, err
		}
	}
	if wasEnabledAdmin != isEnabledTenantAdmin(updated) {
		if err := uc.setAdminMembership(ctx, updated.GetId(), isEnabledTenantAdmin(updated)); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

// ListUsers 用户列表
func (uc *UserUsecase) ListUsers(ctx context.Context, opts ...listing.Option) ([]*pbCore.User, error) {
	return uc.repo.ListUsers(ctx, opts...)
}

// CountUsers 用户计数
func (uc *UserUsecase) CountUsers(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.CountUsers(ctx, opts...)
}
