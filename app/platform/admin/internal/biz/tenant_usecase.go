package biz

import (
	"context"
	"net/mail"
	"regexp"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/crypto"
)

// TenantRepo manages platform tenant records.
type TenantRepo interface {
	Provision(context.Context, *TenantProvisioning) (*TenantProvisioningResult, error)
	RollbackProvisioning(context.Context, *TenantProvisioningResult) error
	Update(context.Context, *pbCore.Tenant, uint32) (*pbCore.Tenant, error)
	FindByID(context.Context, uint32) (*pbCore.Tenant, error)
	CountTenants(context.Context, ...listing.Option) (int32, error)
	ListTenants(context.Context, ...listing.Option) ([]*pbCore.Tenant, error)
	Delete(context.Context, uint32) error
	UpdateLifecycle(context.Context, uint32, pbCore.TenantLifecycleStatus) (*pbCore.Tenant, error)
	ListAdmins(context.Context, uint32) ([]*pbCore.User, error)
	UpdateAdmin(context.Context, uint32, uint32, *string, *string, *string) (*pbCore.User, error)
	ResetAdminPassword(context.Context, uint32, uint32, string) error
}

func (uc *TenantUsecase) ListAdmins(ctx context.Context, tenantID uint32) ([]*pbCore.User, error) {
	if _, err := uc.repo.FindByID(ctx, tenantID); err != nil {
		return nil, err
	}
	return uc.repo.ListAdmins(ctx, tenantID)
}

func (uc *TenantUsecase) UpdateAdmin(ctx context.Context, req *pbCore.UpdateTenantAdminRequest) (*pbCore.User, error) {
	if req.GetEmail() != "" {
		address, err := mail.ParseAddress(req.GetEmail())
		if err != nil || address.Address != req.GetEmail() {
			return nil, pb.ErrorBadRequest("管理员邮箱格式不正确")
		}
	}
	return uc.repo.UpdateAdmin(ctx, req.GetTenantId(), req.GetAdminUserId(), req.Realname, req.Email, req.Phone)
}

func (uc *TenantUsecase) ResetAdminPassword(ctx context.Context, req *pbCore.ResetTenantAdminPasswordRequest) error {
	if err := ValidatePassword(req.GetNewPassword()); err != nil {
		return err
	}
	hash, err := crypto.HashPassword(req.GetNewPassword())
	if err != nil {
		return ErrPasswordHashFailed
	}
	if err := uc.repo.ResetAdminPassword(ctx, req.GetTenantId(), req.GetAdminUserId(), hash); err != nil {
		return err
	}
	if uc.sessions != nil {
		return uc.sessions.RevokeUser(ctx, req.GetAdminUserId())
	}
	return nil
}

// TenantProvisioning holds the input for atomic tenant provisioning.
type TenantProvisioning struct {
	Tenant            *pbCore.Tenant
	OperatorID        uint32
	AdminUsername     string
	AdminPasswordHash string
	AdminRealName     string
	AdminEmail        string
}

// TenantProvisioningResult holds the output of atomic tenant provisioning.
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
	log      *log.Helper
}

func NewTenantUsecase(repo TenantRepo, sessions SessionRepo, logger log.Logger) *TenantUsecase {
	return &TenantUsecase{repo: repo, sessions: sessions, log: log.NewHelper(logger)}
}

// Create provisions a new tenant with initial admin, root dept, and package bindings.
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
		AdminRealName:     admin.GetRealname(),
		AdminEmail:        admin.GetEmail(),
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Update updates tenant basic info.
func (uc *TenantUsecase) Update(ctx context.Context, tenant *pbCore.Tenant, operatorID uint32) (*pbCore.Tenant, error) {
	uc.log.WithContext(ctx).Infof("UpdateTenant: %d", tenant.GetId())
	if tenant.GetIsPlatform() {
		return nil, pb.ErrorBadRequest("平台租户标记不可通过 API 修改")
	}
	if _, err := uc.repo.FindByID(ctx, tenant.GetId()); err != nil {
		return nil, err
	}
	updated, err := uc.repo.Update(ctx, tenant, operatorID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Get returns a tenant by ID.
func (uc *TenantUsecase) Get(ctx context.Context, id uint32) (*pbCore.Tenant, error) {
	return uc.repo.FindByID(ctx, id)
}

// Count returns the tenant count matching the given options.
func (uc *TenantUsecase) Count(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.CountTenants(ctx, opts...)
}

// List returns tenants matching the given options.
func (uc *TenantUsecase) List(ctx context.Context, opts ...listing.Option) ([]*pbCore.Tenant, error) {
	return uc.repo.ListTenants(ctx, opts...)
}

// Delete soft-deletes a tenant and revokes all its sessions.
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

// UpdateLifecycle transitions a tenant through the lifecycle state machine.
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
	// Revoke sessions when leaving ACTIVE state
	if current.GetLifecycleStatus() == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE &&
		target != pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE &&
		uc.sessions != nil {
		if err := uc.sessions.RevokeTenant(ctx, id); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

// tenantLifecycleTransitionAllowed validates lifecycle state transitions.
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
