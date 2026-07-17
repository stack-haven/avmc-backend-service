package data

import (
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantresourcequotausage"
	"backend-service/pkg/utils/convert"
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.ResourceQuotaRepo = (*resourceQuotaRepo)(nil)

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

func (r *resourceQuotaRepo) Consume(ctx context.Context, tenantID uint32, resourceKey string, amount int64, limit int64, unlimited bool, operatorID uint32) (*pbCore.TenantResourceQuotaUsage, error) {
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

	row, err := findQuotaUsageForUpdate(ctx, tx.Client(), tenantID, resourceKey)
	if err != nil && !gen.IsNotFound(err) {
		return nil, err
	}
	if err != nil && gen.IsNotFound(err) {
		if !unlimited && amount > limit {
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
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return resourceQuotaUsageToProto(row), nil
}

func (r *resourceQuotaRepo) Release(ctx context.Context, tenantID uint32, resourceKey string, amount int64, operatorID uint32) (*pbCore.TenantResourceQuotaUsage, error) {
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

	row, err := findQuotaUsageForUpdate(ctx, tx.Client(), tenantID, resourceKey)
	if err != nil {
		if gen.IsNotFound(err) {
			return &pbCore.TenantResourceQuotaUsage{TenantId: tenantID, ResourceKey: resourceKey}, nil
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
