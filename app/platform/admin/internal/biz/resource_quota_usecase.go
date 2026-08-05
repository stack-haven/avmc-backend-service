package biz

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/auth/authn"
)

type ResourceQuotaRepo interface {
	ListUsage(context.Context, uint32) ([]*pbCore.TenantResourceQuotaUsage, error)
	GetUsage(context.Context, uint32, string) (*pbCore.TenantResourceQuotaUsage, error)
	Consume(context.Context, uint32, string, int64, int64, bool, string, uint32) (*pbCore.TenantResourceQuotaUsage, bool, error)
	Release(context.Context, uint32, string, int64, string, uint32) (*pbCore.TenantResourceQuotaUsage, error)
}

type ResourceQuotaUsecase struct {
	repo     ResourceQuotaRepo
	packages TenantMenuPermissionGroupRepo
	log      *log.Helper
}

type ResourceQuotaReservation struct {
	uc                    *ResourceQuotaUsecase
	resourceKey           string
	amount                int64
	releaseIdempotencyKey string
	replay                bool
}

func NewResourceQuotaUsecase(repo ResourceQuotaRepo, packages TenantMenuPermissionGroupRepo, logger log.Logger) *ResourceQuotaUsecase {
	return &ResourceQuotaUsecase{repo: repo, packages: packages, log: log.NewHelper(logger)}
}

func (uc *ResourceQuotaUsecase) ListCurrent(ctx context.Context) ([]*pbCore.TenantResourceQuotaUsage, error) {
	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, err
	}
	limits, err := uc.tenantResourceLimits(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := uc.repo.ListUsage(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]*pbCore.TenantResourceQuotaUsage, len(rows))
	for _, row := range rows {
		if row != nil && row.GetResourceKey() != "" {
			byKey[row.GetResourceKey()] = row
		}
	}
	result := make([]*pbCore.TenantResourceQuotaUsage, 0, len(limits)+len(rows))
	for key, limit := range limits {
		usage := byKey[key]
		if usage == nil {
			usage = &pbCore.TenantResourceQuotaUsage{TenantId: tenantID, ResourceKey: key}
		}
		result = append(result, applyQuotaLimit(usage, limit, false))
		delete(byKey, key)
	}
	for key, usage := range byKey {
		result = append(result, applyQuotaLimit(usage, 0, true))
		delete(byKey, key)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].GetResourceKey() < result[j].GetResourceKey()
	})
	return result, nil
}

func (uc *ResourceQuotaUsecase) CheckCurrent(ctx context.Context, resourceKey string, amount int64) (*pbCore.CheckCurrentTenantResourceQuotaResponse, error) {
	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, err
	}
	resourceKey, err = normalizeResourceKey(resourceKey)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, errors.BadRequest("RESOURCE_QUOTA_AMOUNT_INVALID", "资源额度数量必须大于0")
	}
	limit, unlimited, err := uc.resourceLimit(ctx, tenantID, resourceKey)
	if err != nil {
		return nil, err
	}
	usage, err := uc.repo.GetUsage(ctx, tenantID, resourceKey)
	if err != nil {
		return nil, err
	}
	usage = applyQuotaLimit(usage, limit, unlimited)
	return &pbCore.CheckCurrentTenantResourceQuotaResponse{
		Allowed: unlimited || usage.GetUsed()+amount <= limit,
		Usage:   usage,
	}, nil
}

func (uc *ResourceQuotaUsecase) ConsumeCurrent(ctx context.Context, resourceKey string, amount int64, idempotencyKey string) (*pbCore.TenantResourceQuotaUsage, error) {
	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, err
	}
	resourceKey, err = normalizeResourceKey(resourceKey)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, errors.BadRequest("RESOURCE_QUOTA_AMOUNT_INVALID", "资源额度数量必须大于0")
	}
	idempotencyKey, err = normalizeQuotaIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	limit, unlimited, err := uc.resourceLimit(ctx, tenantID, resourceKey)
	if err != nil {
		return nil, err
	}
	usage, _, err := uc.repo.Consume(ctx, tenantID, resourceKey, amount, limit, unlimited, idempotencyKey, authn.GetAuthUserID(ctx))
	if err != nil {
		return nil, err
	}
	return applyQuotaLimit(usage, limit, unlimited), nil
}

func (uc *ResourceQuotaUsecase) ReleaseCurrent(ctx context.Context, resourceKey string, amount int64, idempotencyKey string) (*pbCore.TenantResourceQuotaUsage, error) {
	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, err
	}
	resourceKey, err = normalizeResourceKey(resourceKey)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, errors.BadRequest("RESOURCE_QUOTA_AMOUNT_INVALID", "资源额度数量必须大于0")
	}
	idempotencyKey, err = normalizeQuotaIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	limit, unlimited, err := uc.resourceLimit(ctx, tenantID, resourceKey)
	if err != nil {
		return nil, err
	}
	usage, err := uc.repo.Release(ctx, tenantID, resourceKey, amount, idempotencyKey, authn.GetAuthUserID(ctx))
	if err != nil {
		return nil, err
	}
	return applyQuotaLimit(usage, limit, unlimited), nil
}

func (uc *ResourceQuotaUsecase) ReserveCurrent(ctx context.Context, resourceKey string, amount int64, idempotencyKey string) (*ResourceQuotaReservation, *pbCore.TenantResourceQuotaUsage, error) {
	idempotencyKey, err := normalizeQuotaIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, nil, err
	}
	if idempotencyKey == "" {
		return nil, nil, errors.BadRequest("RESOURCE_QUOTA_IDEMPOTENCY_KEY_REQUIRED", "资源额度预留必须提供业务幂等键")
	}
	if len(idempotencyKey)+len(":release") > 120 {
		return nil, nil, errors.BadRequest("RESOURCE_QUOTA_IDEMPOTENCY_KEY_INVALID", "资源额度预留幂等键长度不能超过112")
	}
	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, nil, err
	}
	resourceKey, err = normalizeResourceKey(resourceKey)
	if err != nil {
		return nil, nil, err
	}
	if amount <= 0 {
		return nil, nil, errors.BadRequest("RESOURCE_QUOTA_AMOUNT_INVALID", "资源额度数量必须大于0")
	}
	limit, unlimited, err := uc.resourceLimit(ctx, tenantID, resourceKey)
	if err != nil {
		return nil, nil, err
	}
	usage, replay, err := uc.repo.Consume(ctx, tenantID, resourceKey, amount, limit, unlimited, idempotencyKey, authn.GetAuthUserID(ctx))
	if err != nil {
		return nil, nil, err
	}
	usage = applyQuotaLimit(usage, limit, unlimited)
	reservation := &ResourceQuotaReservation{
		uc:                    uc,
		resourceKey:           usage.GetResourceKey(),
		amount:                amount,
		releaseIdempotencyKey: idempotencyKey + ":release",
		replay:                replay,
	}
	return reservation, usage, nil
}

func (r *ResourceQuotaReservation) IsReplay() bool {
	return r != nil && r.replay
}

func (r *ResourceQuotaReservation) Release(ctx context.Context) (*pbCore.TenantResourceQuotaUsage, error) {
	if r == nil || r.uc == nil {
		return nil, errors.BadRequest("RESOURCE_QUOTA_RESERVATION_INVALID", "资源额度预留不存在")
	}
	return r.uc.ReleaseCurrent(ctx, r.resourceKey, r.amount, r.releaseIdempotencyKey)
}

func (uc *ResourceQuotaUsecase) resourceLimit(ctx context.Context, tenantID uint32, resourceKey string) (limit int64, unlimited bool, err error) {
	limits, err := uc.tenantResourceLimits(ctx, tenantID)
	if err != nil {
		return 0, false, err
	}
	limit, ok := limits[resourceKey]
	if !ok {
		return 0, true, nil
	}
	if limit < 0 {
		limit = 0
	}
	return limit, false, nil
}

func (uc *ResourceQuotaUsecase) tenantResourceLimits(ctx context.Context, tenantID uint32) (map[string]int64, error) {
	// TODO: re-implement when tenant-package binding system is rebuilt
	return nil, nil
}

func currentTenantID(ctx context.Context) (uint32, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	if tenantID == 0 {
		return 0, errors.Forbidden("TENANT_CONTEXT_REQUIRED", "缺少有效租户上下文")
	}
	return tenantID, nil
}

var resourceQuotaKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9_-]*)*$`)

func normalizeResourceKey(resourceKey string) (string, error) {
	resourceKey = strings.TrimSpace(resourceKey)
	if !resourceQuotaKeyPattern.MatchString(resourceKey) {
		return "", errors.BadRequest("RESOURCE_QUOTA_KEY_INVALID", "资源额度键必须使用分段小写格式")
	}
	return resourceKey, nil
}

func normalizeQuotaIdempotencyKey(idempotencyKey string) (string, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > 120 {
		return "", errors.BadRequest("RESOURCE_QUOTA_IDEMPOTENCY_KEY_INVALID", "资源额度幂等键长度不能超过120")
	}
	return idempotencyKey, nil
}

func applyQuotaLimit(usage *pbCore.TenantResourceQuotaUsage, limit int64, unlimited bool) *pbCore.TenantResourceQuotaUsage {
	if usage == nil {
		usage = &pbCore.TenantResourceQuotaUsage{}
	}
	result := proto.Clone(usage).(*pbCore.TenantResourceQuotaUsage) //nolint:errcheck // proto.Clone does not return error
	if result.Used < 0 {
		result.Used = 0
	}
	result.Unlimited = unlimited
	if unlimited {
		result.Limit = 0
		result.Remaining = 0
		return result
	}
	if limit < 0 {
		limit = 0
	}
	result.Limit = limit
	result.Remaining = limit - result.Used
	if result.Remaining < 0 {
		result.Remaining = 0
	}
	return result
}
