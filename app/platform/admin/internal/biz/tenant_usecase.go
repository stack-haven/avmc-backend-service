package biz

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/crypto"
	"context"
	"net/mail"
	"regexp"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// TenantRepo manages platform tenant records.
type TenantRepo interface {
	Save(context.Context, *pbCore.Tenant, uint32) (*pbCore.Tenant, error)
	Provision(context.Context, *TenantProvisioning) (*TenantProvisioningResult, error)
	Update(context.Context, *pbCore.Tenant, uint32) (*pbCore.Tenant, error)
	FindByID(context.Context, uint32) (*pbCore.Tenant, error)
	CountTenants(context.Context, ...listing.Option) (int32, error)
	ListTenants(context.Context, ...listing.Option) ([]*pbCore.Tenant, error)
	Delete(context.Context, uint32) error
	UpdateStatus(context.Context, uint32, pbEnum.Status) (*pbCore.Tenant, error)
	UpdateLifecycle(context.Context, uint32, pbCore.TenantLifecycleStatus) (*pbCore.Tenant, error)
}

type TenantProvisioning struct {
	Tenant            *pbCore.Tenant
	OperatorID        uint32
	AdminUsername     string
	AdminPasswordHash string
	AdminRealname     string
	AdminEmail        string
}

type TenantProvisioningResult struct {
	Tenant      *pbCore.Tenant
	AdminUserID uint32
	AdminRoleID uint32
	RootDeptID  uint32
}

var tenantAdminUsernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

// TenantUsecase contains platform tenant business rules.
type TenantUsecase struct {
	repo     TenantRepo
	sessions SessionRepo
	policy   TenantAdminPolicy
	log      *log.Helper
}

func NewTenantUsecase(repo TenantRepo, sessions SessionRepo, policy TenantAdminPolicy, logger log.Logger) *TenantUsecase {
	return &TenantUsecase{repo: repo, sessions: sessions, policy: policy, log: log.NewHelper(logger)}
}

func (uc *TenantUsecase) Create(ctx context.Context, tenant *pbCore.Tenant, admin *pbCore.TenantInitialAdmin, operatorID uint32) (*TenantProvisioningResult, error) {
	if tenant == nil || admin == nil {
		return nil, pb.ErrorBadRequest("租户信息和初始管理员不能为空")
	}
	if len(tenant.GetGroupIds()) == 0 {
		return nil, pb.ErrorBadRequest("至少需要绑定一个业务套餐")
	}
	if !tenantAdminUsernamePattern.MatchString(admin.GetUsername()) {
		return nil, pb.ErrorBadRequest("管理员用户名格式不正确")
	}
	if admin.GetEmail() != "" {
		address, err := mail.ParseAddress(admin.GetEmail())
		if err != nil || address.Address != admin.GetEmail() {
			return nil, pb.ErrorBadRequest("管理员邮箱格式不正确")
		}
	}
	if err := ValidatePassword(admin.GetPassword()); err != nil {
		return nil, err
	}
	if expiresAt := tenant.GetExpiresAt(); expiresAt != "" {
		expiry, err := time.Parse(time.DateTime, expiresAt)
		if err != nil {
			return nil, pb.ErrorBadRequest("到期时间格式必须为 YYYY-MM-DD HH:mm:ss")
		}
		if !expiry.After(time.Now()) {
			return nil, pb.ErrorBadRequest("到期时间必须晚于当前时间")
		}
	}
	passwordHash, err := crypto.HashPassword(admin.GetPassword())
	if err != nil {
		return nil, ErrPasswordHashFailed
	}
	uc.log.WithContext(ctx).Infof("CreateTenant: %s", tenant.GetCode())
	result, err := uc.repo.Provision(ctx, &TenantProvisioning{
		Tenant:            tenant,
		OperatorID:        operatorID,
		AdminUsername:     admin.GetUsername(),
		AdminPasswordHash: passwordHash,
		AdminRealname:     admin.GetRealname(),
		AdminEmail:        admin.GetEmail(),
	})
	if err != nil {
		return nil, err
	}
	if uc.policy != nil {
		if err = uc.policy.SetMembership(ctx, result.Tenant.GetId(), result.AdminUserID, true); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (uc *TenantUsecase) Update(ctx context.Context, tenant *pbCore.Tenant, operatorID uint32) (*pbCore.Tenant, error) {
	uc.log.WithContext(ctx).Infof("UpdateTenant: %d", tenant.GetId())
	current, err := uc.repo.FindByID(ctx, tenant.GetId())
	if err != nil {
		return nil, err
	}
	updated, err := uc.repo.Update(ctx, tenant, operatorID)
	if err != nil {
		return nil, err
	}
	if tenant.ExpiresAt != nil && current.GetExpiresAt() != updated.GetExpiresAt() && uc.sessions != nil {
		if err = uc.sessions.RevokeTenant(ctx, tenant.GetId()); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func (uc *TenantUsecase) Get(ctx context.Context, id uint32) (*pbCore.Tenant, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *TenantUsecase) Count(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.CountTenants(ctx, opts...)
}

func (uc *TenantUsecase) List(ctx context.Context, opts ...listing.Option) ([]*pbCore.Tenant, error) {
	return uc.repo.ListTenants(ctx, opts...)
}

func (uc *TenantUsecase) Delete(ctx context.Context, id uint32) error {
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return err
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	if uc.sessions != nil {
		return uc.sessions.RevokeTenant(ctx, id)
	}
	return nil
}

func (uc *TenantUsecase) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pbCore.Tenant, error) {
	switch status {
	case pbEnum.Status_STATUS_ENABLED:
		return uc.UpdateLifecycle(ctx, id, pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE)
	case pbEnum.Status_STATUS_DISABLED:
		return uc.UpdateLifecycle(ctx, id, pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED)
	default:
		return nil, pb.ErrorBadRequest("不支持的租户状态")
	}
}

func (uc *TenantUsecase) UpdateLifecycle(ctx context.Context, id uint32, target pbCore.TenantLifecycleStatus) (*pbCore.Tenant, error) {
	current, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !tenantLifecycleTransitionAllowed(current.GetLifecycleStatus(), target) {
		return nil, pb.ErrorBadRequest("不允许的租户生命周期状态迁移")
	}
	updated, err := uc.repo.UpdateLifecycle(ctx, id, target)
	if err != nil {
		return nil, err
	}
	if target != pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE && uc.sessions != nil {
		if err := uc.sessions.RevokeTenant(ctx, id); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func tenantLifecycleTransitionAllowed(current, target pbCore.TenantLifecycleStatus) bool {
	if current == target {
		return true
	}
	switch current {
	case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING:
		return target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE ||
			target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED
	case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE:
		return target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED ||
			target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED ||
			target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED
	case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED:
		return target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE ||
			target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED ||
			target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED
	case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED:
		return target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE ||
			target == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED
	default:
		return false
	}
}
