package data

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	pb "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/asynctask"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type asyncTaskRepo struct{ BaseRepo }

type asyncTaskAlertThresholds struct {
	pendingOverdueWarning                int32
	failedWarning                        int32
	runningLeaseExpiredCritical          int32
	retryPressureCritical                int32
	permissionCacheFailedCritical        int32
	permissionCacheRetryPressureCritical int32
}

func NewAsyncTaskRepo(data *Data, logger log.Logger) biz.AsyncTaskRepo {
	return &asyncTaskRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func asyncTaskAlertThresholdsFromEnv() asyncTaskAlertThresholds {
	return asyncTaskAlertThresholds{
		pendingOverdueWarning:                int32Env("platform_ADMIN_ASYNC_TASK_PENDING_OVERDUE_WARNING_THRESHOLD", 1),
		failedWarning:                        int32Env("platform_ADMIN_ASYNC_TASK_FAILED_WARNING_THRESHOLD", 1),
		runningLeaseExpiredCritical:          int32Env("platform_ADMIN_ASYNC_TASK_RUNNING_LEASE_EXPIRED_CRITICAL_THRESHOLD", 1),
		retryPressureCritical:                int32Env("platform_ADMIN_ASYNC_TASK_RETRY_PRESSURE_CRITICAL_THRESHOLD", 1),
		permissionCacheFailedCritical:        int32Env("platform_ADMIN_PERMISSION_CACHE_FAILED_CRITICAL_THRESHOLD", 1),
		permissionCacheRetryPressureCritical: int32Env("platform_ADMIN_PERMISSION_CACHE_RETRY_PRESSURE_CRITICAL_THRESHOLD", 1),
	}
}

func int32Env(key string, fallback int32) int32 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return int32(value)
}

func asyncTaskProto(row *gen.AsyncTask) *pb.AsyncTask {
	if row == nil {
		return nil
	}
	result := &pb.AsyncTask{
		Id:             row.ID,
		TenantId:       row.TenantID,
		TaskType:       row.TaskType,
		Queue:          row.Queue,
		Status:         pb.AsyncTaskStatus(row.Status),
		Priority:       row.Priority,
		Attempts:       row.Attempts,
		MaxAttempts:    row.MaxAttempts,
		IdempotencyKey: row.IdempotencyKey,
		CreatedBy:      row.CreatedBy,
	}
	setOptionalString(&result.PayloadSummary, row.PayloadSummary)
	setOptionalString(&result.ResultSummary, row.ResultSummary)
	setOptionalString(&result.LastError, row.LastError)
	setOptionalString(&result.LeaseOwner, row.LeaseOwner)
	setOptionalTime(&result.ScheduledAt, &row.ScheduledAt)
	setOptionalTime(&result.StartedAt, row.StartedAt)
	setOptionalTime(&result.CompletedAt, row.CompletedAt)
	setOptionalTime(&result.LeaseExpiresAt, row.LeaseExpiresAt)
	setOptionalTime(&result.CreatedAt, &row.CreatedAt)
	setOptionalTime(&result.UpdatedAt, &row.UpdatedAt)
	return result
}

func (r *asyncTaskRepo) Enqueue(ctx context.Context, spec *biz.AsyncTaskSpec) (*pb.AsyncTask, error) {
	ctx = entviewer.NewSystemContext(ctx)
	idempotencyKey := scopedTaskIdempotencyKey(spec)
	builder := r.Data.DB(ctx).AsyncTask.Create().
		SetNillableTenantID(spec.TenantID).
		SetTaskType(spec.TaskType).
		SetQueue(spec.Queue).
		SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING)).
		SetPriority(spec.Priority).
		SetPayload(string(spec.Payload)).
		SetPayloadSummary(spec.PayloadSummary).
		SetAttempts(0).
		SetMaxAttempts(spec.MaxAttempts).
		SetScheduledAt(spec.ScheduledAt).
		SetNillableCreatedBy(spec.CreatedBy)
	if idempotencyKey != "" {
		builder.SetIdempotencyKey(idempotencyKey)
	}
	row, err := builder.Save(ctx)
	if gen.IsConstraintError(err) && idempotencyKey != "" {
		row, err = r.Data.DB(ctx).AsyncTask.Query().
			Where(asynctask.IdempotencyKeyEQ(idempotencyKey)).
			Only(ctx)
	}
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("ASYNC_TASK_IDEMPOTENCY_CONFLICT", "任务幂等键已存在")
	}
	if err != nil {
		return nil, err
	}
	return asyncTaskProto(row), nil
}

func scopedTaskIdempotencyKey(spec *biz.AsyncTaskSpec) string {
	if spec == nil || spec.IdempotencyKey == "" {
		return ""
	}
	if spec.TenantID == nil {
		return "platform:" + spec.IdempotencyKey
	}
	return fmt.Sprintf("tenant:%d:%s", *spec.TenantID, spec.IdempotencyKey)
}

func (r *asyncTaskRepo) List(ctx context.Context, req *pb.ListAsyncTasksRequest) ([]*pb.AsyncTask, int32, error) {
	ctx = entviewer.NewSystemContext(ctx)
	query := r.Data.DB(ctx).AsyncTask.Query()
	if req.TenantId != nil {
		query.Where(asynctask.TenantIDEQ(req.GetTenantId()))
	}
	if value := strings.TrimSpace(req.GetTaskType()); value != "" {
		query.Where(asynctask.TaskTypeContains(value))
	}
	if req.Status != nil && req.GetStatus() != pb.AsyncTaskStatus_ASYNC_TASK_STATUS_UNSPECIFIED {
		query.Where(asynctask.StatusEQ(int32(req.GetStatus())))
	}
	if value := strings.TrimSpace(req.GetQueue()); value != "" {
		query.Where(asynctask.QueueEQ(value))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.
		Order(gen.Desc(asynctask.FieldCreatedAt)).
		Offset(offset).
		Limit(size).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return convert.SliceToAny(rows, asyncTaskProto), int32(total), nil
}

func (r *asyncTaskRepo) Stats(ctx context.Context, req *pb.GetAsyncTaskStatsRequest) (*pb.AsyncTaskStats, error) {
	if req == nil {
		req = &pb.GetAsyncTaskStatsRequest{}
	}
	ctx = entviewer.NewSystemContext(ctx)
	now := time.Now()
	overdueSeconds := req.GetPendingOverdueSeconds()
	if overdueSeconds <= 0 {
		overdueSeconds = 300
	}
	overdueBefore := now.Add(-time.Duration(overdueSeconds) * time.Second)
	statuses := []pb.AsyncTaskStatus{
		pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING,
		pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING,
		pb.AsyncTaskStatus_ASYNC_TASK_STATUS_SUCCEEDED,
		pb.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED,
		pb.AsyncTaskStatus_ASYNC_TASK_STATUS_CANCELED,
	}
	base := func() *gen.AsyncTaskQuery {
		query := r.Data.DB(ctx).AsyncTask.Query()
		if value := strings.TrimSpace(req.GetQueue()); value != "" {
			query.Where(asynctask.QueueEQ(value))
		}
		return query
	}

	total, err := base().Count(ctx)
	if err != nil {
		return nil, err
	}
	stats := &pb.AsyncTaskStats{Total: int32(total)}
	for _, status := range statuses {
		count, countErr := base().Where(asynctask.StatusEQ(int32(status))).Count(ctx)
		if countErr != nil {
			return nil, countErr
		}
		stats.StatusCounts = append(stats.StatusCounts, &pb.AsyncTaskStatusCount{
			Status: status,
			Count:  int32(count),
		})
		if status == pb.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED {
			stats.Failed = int32(count)
		}
	}
	pendingOverdue, err := base().Where(
		asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING)),
		asynctask.ScheduledAtLTE(overdueBefore),
	).Count(ctx)
	if err != nil {
		return nil, err
	}
	stats.PendingOverdue = int32(pendingOverdue)

	runningLeaseExpired, err := base().Where(
		asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING)),
		asynctask.LeaseExpiresAtNotNil(),
		asynctask.LeaseExpiresAtLTE(now),
	).Count(ctx)
	if err != nil {
		return nil, err
	}
	stats.RunningLeaseExpired = int32(runningLeaseExpired)

	retryPressureRows, err := base().Where(
		asynctask.StatusIn(
			int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING),
			int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING),
		),
	).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range retryPressureRows {
		if row.Attempts >= row.MaxAttempts-1 {
			stats.RetryPressure++
		}
	}
	permissionCacheFailed, err := base().Where(
		asynctask.TaskTypeEQ(biz.AsyncTaskTypePermissionCacheInvalidate),
		asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED)),
	).Count(ctx)
	if err != nil {
		return nil, err
	}
	permissionCacheRetryPressureRows, err := base().Where(
		asynctask.TaskTypeEQ(biz.AsyncTaskTypePermissionCacheInvalidate),
		asynctask.StatusIn(
			int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING),
			int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING),
		),
	).All(ctx)
	if err != nil {
		return nil, err
	}
	var permissionCacheRetryPressure int32
	for _, row := range permissionCacheRetryPressureRows {
		if row.Attempts >= row.MaxAttempts-1 {
			permissionCacheRetryPressure++
		}
	}
	stats.HealthStatus = pb.AsyncTaskHealthStatus_ASYNC_TASK_HEALTH_STATUS_HEALTHY
	thresholds := asyncTaskAlertThresholdsFromEnv()
	appendAlert := func(code string, status pb.AsyncTaskHealthStatus, count int32, threshold int32) {
		if threshold <= 0 || count < threshold {
			return
		}
		stats.Alerts = append(stats.Alerts, &pb.AsyncTaskHealthAlert{
			Code:   code,
			Status: status,
			Count:  count,
		})
		if status > stats.HealthStatus {
			stats.HealthStatus = status
		}
	}
	appendAlert("pending_overdue", pb.AsyncTaskHealthStatus_ASYNC_TASK_HEALTH_STATUS_WARNING, stats.GetPendingOverdue(), thresholds.pendingOverdueWarning)
	appendAlert("failed", pb.AsyncTaskHealthStatus_ASYNC_TASK_HEALTH_STATUS_WARNING, stats.GetFailed(), thresholds.failedWarning)
	appendAlert("running_lease_expired", pb.AsyncTaskHealthStatus_ASYNC_TASK_HEALTH_STATUS_CRITICAL, stats.GetRunningLeaseExpired(), thresholds.runningLeaseExpiredCritical)
	appendAlert("retry_pressure", pb.AsyncTaskHealthStatus_ASYNC_TASK_HEALTH_STATUS_CRITICAL, stats.GetRetryPressure(), thresholds.retryPressureCritical)
	appendAlert("permission_cache_failed", pb.AsyncTaskHealthStatus_ASYNC_TASK_HEALTH_STATUS_CRITICAL, int32(permissionCacheFailed), thresholds.permissionCacheFailedCritical)
	appendAlert("permission_cache_retry_pressure", pb.AsyncTaskHealthStatus_ASYNC_TASK_HEALTH_STATUS_CRITICAL, permissionCacheRetryPressure, thresholds.permissionCacheRetryPressureCritical)
	setOptionalTime(&stats.CheckedAt, &now)
	return stats, nil
}

func (r *asyncTaskRepo) Get(ctx context.Context, id uint32) (*pb.AsyncTask, error) {
	ctx = entviewer.NewSystemContext(ctx)
	row, err := r.Data.DB(ctx).AsyncTask.Get(ctx, id)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("ASYNC_TASK_NOT_FOUND", "异步任务不存在")
	}
	if err != nil {
		return nil, err
	}
	return asyncTaskProto(row), nil
}

func (r *asyncTaskRepo) Cancel(ctx context.Context, id uint32) error {
	ctx = entviewer.NewSystemContext(ctx)
	now := time.Now()
	affected, err := r.Data.DB(ctx).AsyncTask.Update().
		Where(
			asynctask.IDEQ(id),
			asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING)),
		).
		SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_CANCELED)).
		SetCompletedAt(now).
		ClearLeaseExpiresAt().
		SetLeaseOwner("").
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	if _, err = r.Get(ctx, id); err != nil {
		return err
	}
	return errors.Conflict("ASYNC_TASK_NOT_CANCELABLE", "只有待执行任务可以取消")
}

func (r *asyncTaskRepo) Retry(ctx context.Context, id uint32) (*pb.AsyncTask, error) {
	ctx = entviewer.NewSystemContext(ctx)
	affected, err := r.Data.DB(ctx).AsyncTask.Update().
		Where(
			asynctask.IDEQ(id),
			asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED)),
		).
		SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING)).
		SetAttempts(0).
		SetScheduledAt(time.Now()).
		SetLastError("").
		SetResultSummary("").
		ClearStartedAt().
		ClearCompletedAt().
		ClearLeaseExpiresAt().
		SetLeaseOwner("").
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		if _, err = r.Get(ctx, id); err != nil {
			return nil, err
		}
		return nil, errors.Conflict("ASYNC_TASK_NOT_RETRYABLE", "只有失败任务可以重试")
	}
	return r.Get(ctx, id)
}

func (r *asyncTaskRepo) Claim(ctx context.Context, workerID, queue string, lease time.Duration) (*biz.AsyncTaskExecution, error) {
	ctx = entviewer.NewSystemContext(ctx)
	now := time.Now()
	candidates, err := r.Data.DB(ctx).AsyncTask.Query().
		Where(
			asynctask.QueueEQ(queue),
			asynctask.Or(
				asynctask.And(
					asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING)),
					asynctask.ScheduledAtLTE(now),
				),
				asynctask.And(
					asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING)),
					asynctask.LeaseExpiresAtNotNil(),
					asynctask.LeaseExpiresAtLTE(now),
				),
			),
		).
		Order(gen.Desc(asynctask.FieldPriority), gen.Asc(asynctask.FieldScheduledAt), gen.Asc(asynctask.FieldID)).
		Limit(10).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		if candidate.Attempts >= candidate.MaxAttempts {
			_, updateErr := r.Data.DB(ctx).AsyncTask.Update().
				Where(
					asynctask.IDEQ(candidate.ID),
					asynctask.Or(
						asynctask.And(
							asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING)),
							asynctask.ScheduledAtLTE(now),
						),
						asynctask.And(
							asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING)),
							asynctask.LeaseExpiresAtNotNil(),
							asynctask.LeaseExpiresAtLTE(now),
						),
					),
				).
				SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED)).
				SetLastError("worker lease expired after the final allowed attempt").
				SetCompletedAt(now).
				SetLeaseOwner("").
				ClearLeaseExpiresAt().
				Save(ctx)
			if updateErr != nil {
				return nil, updateErr
			}
			continue
		}
		affected, updateErr := r.Data.DB(ctx).AsyncTask.Update().
			Where(
				asynctask.IDEQ(candidate.ID),
				asynctask.Or(
					asynctask.And(
						asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING)),
						asynctask.ScheduledAtLTE(now),
					),
					asynctask.And(
						asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING)),
						asynctask.LeaseExpiresAtNotNil(),
						asynctask.LeaseExpiresAtLTE(now),
					),
				),
			).
			SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING)).
			SetAttempts(candidate.Attempts + 1).
			SetStartedAt(now).
			SetLeaseOwner(workerID).
			SetLeaseExpiresAt(now.Add(lease)).
			Save(ctx)
		if updateErr != nil {
			return nil, updateErr
		}
		if affected == 0 {
			continue
		}
		row, getErr := r.Data.DB(ctx).AsyncTask.Get(ctx, candidate.ID)
		if getErr != nil {
			return nil, getErr
		}
		return &biz.AsyncTaskExecution{
			Task:    asyncTaskProto(row),
			Payload: []byte(row.Payload),
		}, nil
	}
	return nil, nil
}

func (r *asyncTaskRepo) Complete(ctx context.Context, id uint32, workerID, result string) error {
	ctx = entviewer.NewSystemContext(ctx)
	affected, err := r.Data.DB(ctx).AsyncTask.Update().
		Where(
			asynctask.IDEQ(id),
			asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING)),
			asynctask.LeaseOwnerEQ(workerID),
		).
		SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_SUCCEEDED)).
		SetResultSummary(result).
		SetLastError("").
		SetCompletedAt(time.Now()).
		SetLeaseOwner("").
		ClearLeaseExpiresAt().
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.Conflict("ASYNC_TASK_LEASE_LOST", "任务租约已失效")
	}
	return nil
}

func (r *asyncTaskRepo) Fail(ctx context.Context, id uint32, workerID, lastError string, nextRun time.Time) error {
	ctx = entviewer.NewSystemContext(ctx)
	row, err := r.Data.DB(ctx).AsyncTask.Query().
		Where(
			asynctask.IDEQ(id),
			asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING)),
			asynctask.LeaseOwnerEQ(workerID),
		).
		Only(ctx)
	if gen.IsNotFound(err) {
		return errors.Conflict("ASYNC_TASK_LEASE_LOST", "任务租约已失效")
	}
	if err != nil {
		return err
	}
	update := r.Data.DB(ctx).AsyncTask.Update().
		Where(
			asynctask.IDEQ(id),
			asynctask.StatusEQ(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING)),
			asynctask.LeaseOwnerEQ(workerID),
		).
		SetLastError(lastError).
		SetLeaseOwner("").
		ClearLeaseExpiresAt()
	if row.Attempts >= row.MaxAttempts {
		update.
			SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED)).
			SetCompletedAt(time.Now())
	} else {
		update.
			SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING)).
			SetScheduledAt(nextRun).
			ClearCompletedAt()
	}
	affected, err := update.Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.Conflict("ASYNC_TASK_LEASE_LOST", "任务租约已失效")
	}
	return nil
}

func (r *asyncTaskRepo) PurgeTerminal(ctx context.Context, before time.Time, limit int) (int, error) {
	ctx = entviewer.NewSystemContext(ctx)
	rows, err := r.Data.DB(ctx).AsyncTask.Query().
		Where(
			asynctask.StatusIn(
				int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_SUCCEEDED),
				int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED),
				int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_CANCELED),
			),
			asynctask.CompletedAtNotNil(),
			asynctask.CompletedAtLT(before),
		).
		Order(gen.Asc(asynctask.FieldCompletedAt)).
		Limit(limit).
		Select(asynctask.FieldID).
		All(ctx)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	ids := make([]uint32, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return r.Data.DB(ctx).AsyncTask.Delete().
		Where(asynctask.IDIn(ids...)).
		Exec(ctx)
}
