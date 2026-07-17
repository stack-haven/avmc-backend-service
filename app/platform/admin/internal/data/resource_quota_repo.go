package data

import (
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantresourcequotaoperation"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantresourcequotausage"
	"backend-service/pkg/utils/convert"
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.ResourceQuotaRepo = (*resourceQuotaRepo)(nil)

const (
	quotaOperationConsume = "consume"
	quotaOperationRelease = "release"
)

type resourceQuotaRepo struct {
	BaseRepo
}

func NewResourceQuotaRepo(data *Data, logger log.Logger) biz.ResourceQuotaRepo {
	return &resourceQuotaRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func resourceQuotaUsageToProto(row *gen.TenantResourceQuotaUsage) *pbCore.TenantResourceQuotaUsage {
	if row == nil {
		return nil
	}
	return &pbCore.TenantResourceQuotaUsage{
		TenantId:    row.TenantID,
		ResourceKey: row.ResourceKey,
		Used:        row.Used,
		UpdatedAt:   convert.TimeValueToString(&row.UpdatedAt, time.DateTime),
	}
}

func (r *resourceQuotaRepo) ListUsage(ctx context.Context, tenantID uint32) ([]*pbCore.TenantResourceQuotaUsage, error) {
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	rows, err := r.Data.DB(ctx).TenantResourceQuotaUsage.Query().
		Where(tenantresourcequotausage.TenantIDEQ(tenantID)).
		Order(gen.Asc(tenantresourcequotausage.FieldResourceKey)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(rows, resourceQuotaUsageToProto), nil
}

func (r *resourceQuotaRepo) GetUsage(ctx context.Context, tenantID uint32, resourceKey string) (*pbCore.TenantResourceQuotaUsage, error) {
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	row, err := r.Data.DB(ctx).TenantResourceQuotaUsage.Query().
		Where(
			tenantresourcequotausage.TenantIDEQ(tenantID),
			tenantresourcequotausage.ResourceKeyEQ(resourceKey),
		).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return &pbCore.TenantResourceQuotaUsage{TenantId: tenantID, ResourceKey: resourceKey}, nil
		}
		return nil, err
	}
	return resourceQuotaUsageToProto(row), nil
}

func (r *resourceQuotaRepo) Consume(ctx context.Context, tenantID uint32, resourceKey string, amount int64, limit int64, unlimited bool, idempotencyKey string, operatorID uint32) (*pbCore.TenantResourceQuotaUsage, error) {
	r.recordConsume()
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	if amount <= 0 {
		return nil, pb.ErrorBadRequest("资源额度数量必须大于0")
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)

	if idempotencyKey != "" {
		op, err := findQuotaOperationForUpdate(ctx, tx.Client(), tenantID, quotaOperationConsume, idempotencyKey)
		if err != nil && !gen.IsNotFound(err) {
			return nil, err
		}
		if op != nil {
			if err = ensureQuotaOperationReplay(op, resourceKey, amount); err != nil {
				r.recordIdempotencyConflict()
				return nil, err
			}
			return quotaUsageFromOperation(op), nil
		}
	}

	row, err := findQuotaUsageForUpdate(ctx, tx.Client(), tenantID, resourceKey)
	if err != nil && !gen.IsNotFound(err) {
		return nil, err
	}
	if err != nil && gen.IsNotFound(err) {
		if !unlimited && amount > limit {
			r.recordQuotaExceeded()
			return nil, errors.Forbidden("RESOURCE_QUOTA_EXCEEDED", "资源额度不足")
		}
		builder := tx.TenantResourceQuotaUsage.Create().
			SetTenantID(tenantID).
			SetResourceKey(resourceKey).
			SetUsed(amount)
		if operatorID > 0 {
			builder.SetUpdatedBy(operatorID)
		}
		row, err = builder.Save(ctx)
		if err != nil {
			if gen.IsConstraintError(err) {
				return nil, pb.ErrorBadRequest("资源额度使用量正在被并发更新，请重试")
			}
			return nil, err
		}
	} else {
		nextUsed := row.Used + amount
		if !unlimited && nextUsed > limit {
			r.recordQuotaExceeded()
			return nil, errors.Forbidden("RESOURCE_QUOTA_EXCEEDED", "资源额度不足")
		}
		builder := tx.TenantResourceQuotaUsage.UpdateOneID(row.ID).SetUsed(nextUsed)
		if operatorID > 0 {
			builder.SetUpdatedBy(operatorID)
		}
		row, err = builder.Save(ctx)
		if err != nil {
			return nil, err
		}
	}
	if idempotencyKey != "" {
		if err = createQuotaOperation(ctx, tx.Client(), tenantID, resourceKey, quotaOperationConsume, idempotencyKey, amount, row.Used, operatorID); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return resourceQuotaUsageToProto(row), nil
}

func (r *resourceQuotaRepo) Release(ctx context.Context, tenantID uint32, resourceKey string, amount int64, idempotencyKey string, operatorID uint32) (*pbCore.TenantResourceQuotaUsage, error) {
	r.recordRelease()
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	if amount <= 0 {
		return nil, pb.ErrorBadRequest("资源额度数量必须大于0")
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)

	if idempotencyKey != "" {
		op, err := findQuotaOperationForUpdate(ctx, tx.Client(), tenantID, quotaOperationRelease, idempotencyKey)
		if err != nil && !gen.IsNotFound(err) {
			return nil, err
		}
		if op != nil {
			if err = ensureQuotaOperationReplay(op, resourceKey, amount); err != nil {
				r.recordIdempotencyConflict()
				return nil, err
			}
			return quotaUsageFromOperation(op), nil
		}
	}

	row, err := findQuotaUsageForUpdate(ctx, tx.Client(), tenantID, resourceKey)
	if err != nil {
		if gen.IsNotFound(err) {
			usage := &pbCore.TenantResourceQuotaUsage{TenantId: tenantID, ResourceKey: resourceKey}
			if idempotencyKey != "" {
				if err = createQuotaOperation(ctx, tx.Client(), tenantID, resourceKey, quotaOperationRelease, idempotencyKey, amount, 0, operatorID); err != nil {
					return nil, err
				}
				if err = tx.Commit(); err != nil {
					return nil, err
				}
			}
			return usage, nil
		}
		return nil, err
	}
	nextUsed := row.Used - amount
	if nextUsed < 0 {
		nextUsed = 0
	}
	builder := tx.TenantResourceQuotaUsage.UpdateOneID(row.ID).SetUsed(nextUsed)
	if operatorID > 0 {
		builder.SetUpdatedBy(operatorID)
	}
	row, err = builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	if idempotencyKey != "" {
		if err = createQuotaOperation(ctx, tx.Client(), tenantID, resourceKey, quotaOperationRelease, idempotencyKey, amount, row.Used, operatorID); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return resourceQuotaUsageToProto(row), nil
}

func findQuotaUsageForUpdate(ctx context.Context, client *gen.Client, tenantID uint32, resourceKey string) (*gen.TenantResourceQuotaUsage, error) {
	query := client.TenantResourceQuotaUsage.Query().Where(
		tenantresourcequotausage.TenantIDEQ(tenantID),
		tenantresourcequotausage.ResourceKeyEQ(resourceKey),
	)
	row, err := query.Clone().ForUpdate().Only(ctx)
	if isSelectForUpdateUnsupported(err) {
		return query.Only(ctx)
	}
	return row, err
}

func findQuotaOperationForUpdate(ctx context.Context, client *gen.Client, tenantID uint32, operationType string, idempotencyKey string) (*gen.TenantResourceQuotaOperation, error) {
	query := client.TenantResourceQuotaOperation.Query().Where(
		tenantresourcequotaoperation.TenantIDEQ(tenantID),
		tenantresourcequotaoperation.OperationTypeEQ(operationType),
		tenantresourcequotaoperation.IdempotencyKeyEQ(idempotencyKey),
	)
	row, err := query.Clone().ForUpdate().Only(ctx)
	if isSelectForUpdateUnsupported(err) {
		return query.Only(ctx)
	}
	return row, err
}

func ensureQuotaOperationReplay(row *gen.TenantResourceQuotaOperation, resourceKey string, amount int64) error {
	if row.ResourceKey != resourceKey || row.Amount != amount {
		return errors.Conflict("RESOURCE_QUOTA_IDEMPOTENCY_CONFLICT", "资源额度幂等键已被不同请求使用")
	}
	return nil
}

func quotaUsageFromOperation(row *gen.TenantResourceQuotaOperation) *pbCore.TenantResourceQuotaUsage {
	if row == nil {
		return nil
	}
	return &pbCore.TenantResourceQuotaUsage{
		TenantId:    row.TenantID,
		ResourceKey: row.ResourceKey,
		Used:        row.UsedAfter,
		UpdatedAt:   convert.TimeValueToString(&row.UpdatedAt, time.DateTime),
	}
}

func createQuotaOperation(ctx context.Context, client *gen.Client, tenantID uint32, resourceKey string, operationType string, idempotencyKey string, amount int64, usedAfter int64, operatorID uint32) error {
	builder := client.TenantResourceQuotaOperation.Create().
		SetTenantID(tenantID).
		SetResourceKey(resourceKey).
		SetOperationType(operationType).
		SetIdempotencyKey(idempotencyKey).
		SetAmount(amount).
		SetUsedAfter(usedAfter)
	if operatorID > 0 {
		builder.SetUpdatedBy(operatorID)
	}
	_, err := builder.Save(ctx)
	if err != nil && gen.IsConstraintError(err) {
		return errors.Conflict("RESOURCE_QUOTA_IDEMPOTENCY_CONFLICT", "资源额度幂等键正在被并发使用，请重试")
	}
	return err
}

func (r *resourceQuotaRepo) recordConsume() {
	if r != nil && r.Data != nil {
		r.Data.resourceQuotaStats.consumes.Add(1)
	}
}

func (r *resourceQuotaRepo) recordRelease() {
	if r != nil && r.Data != nil {
		r.Data.resourceQuotaStats.releases.Add(1)
	}
}

func (r *resourceQuotaRepo) recordQuotaExceeded() {
	if r != nil && r.Data != nil {
		r.Data.resourceQuotaStats.quotaExceeded.Add(1)
	}
}

func (r *resourceQuotaRepo) recordIdempotencyConflict() {
	if r != nil && r.Data != nil {
		r.Data.resourceQuotaStats.idempotencyConflicts.Add(1)
	}
}
