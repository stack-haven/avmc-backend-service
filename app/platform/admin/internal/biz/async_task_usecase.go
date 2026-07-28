package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	pb "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	AsyncTaskTypeRetentionCleanup          = "system.task_retention_cleanup"
	AsyncTaskTypePermissionCacheInvalidate = "system.permission_cache_invalidate"
)

type AsyncTaskSpec struct {
	TenantID       *uint32
	TaskType       string
	Queue          string
	Priority       int32
	Payload        json.RawMessage
	PayloadSummary string
	IdempotencyKey string
	MaxAttempts    int32
	ScheduledAt    time.Time
	CreatedBy      *uint32
}

type AsyncTaskExecution struct {
	Task    *pb.AsyncTask
	Payload json.RawMessage
}

type AsyncTaskRepo interface {
	Enqueue(context.Context, *AsyncTaskSpec) (*pb.AsyncTask, error)
	List(context.Context, *pb.ListAsyncTasksRequest) ([]*pb.AsyncTask, int32, error)
	Stats(context.Context, *pb.GetAsyncTaskStatsRequest) (*pb.AsyncTaskStats, error)
	Get(context.Context, uint32) (*pb.AsyncTask, error)
	Cancel(context.Context, uint32) error
	Retry(context.Context, uint32) (*pb.AsyncTask, error)
	Claim(context.Context, string, string, time.Duration) (*AsyncTaskExecution, error)
	Complete(context.Context, uint32, string, string) error
	Fail(context.Context, uint32, string, string, time.Time) error
	PurgeTerminal(context.Context, time.Time, int) (int, error)
}

type AsyncTaskHandler interface {
	Type() string
	Handle(context.Context, json.RawMessage) (string, error)
}

type PermissionCacheInvalidator interface {
	InvalidateMenuPermissionCache(context.Context) error
	InvalidateTenantPackagePermissionCache(context.Context, uint32) error
	InvalidateTenantAuthorizationCache(context.Context, uint32) error
}

type AsyncTaskUsecase struct {
	repo     AsyncTaskRepo
	handlers map[string]AsyncTaskHandler
	log      *log.Helper
}

func NewAsyncTaskUsecase(repo AsyncTaskRepo, handlers []AsyncTaskHandler, logger log.Logger) *AsyncTaskUsecase {
	registry := make(map[string]AsyncTaskHandler, len(handlers))
	for _, handler := range handlers {
		if handler != nil && strings.TrimSpace(handler.Type()) != "" {
			registry[handler.Type()] = handler
		}
	}
	return &AsyncTaskUsecase{
		repo:     repo,
		handlers: registry,
		log:      log.NewHelper(log.With(logger, "module", "biz/async-task")),
	}
}

func (uc *AsyncTaskUsecase) Enqueue(ctx context.Context, spec *AsyncTaskSpec) (*pb.AsyncTask, error) {
	if spec == nil {
		return nil, errors.BadRequest("ASYNC_TASK_INVALID", "任务不能为空")
	}
	spec.TaskType = strings.TrimSpace(spec.TaskType)
	if _, ok := uc.handlers[spec.TaskType]; !ok {
		return nil, errors.BadRequest("ASYNC_TASK_TYPE_UNREGISTERED", "任务类型未注册")
	}
	if spec.Queue == "" {
		spec.Queue = "default"
	}
	if spec.MaxAttempts == 0 {
		spec.MaxAttempts = 3
	}
	if spec.MaxAttempts < 1 || spec.MaxAttempts > 10 {
		return nil, errors.BadRequest("ASYNC_TASK_MAX_ATTEMPTS_INVALID", "最大重试次数必须在 1 到 10 之间")
	}
	if len(spec.IdempotencyKey) > 100 {
		return nil, errors.BadRequest("ASYNC_TASK_IDEMPOTENCY_KEY_INVALID", "任务幂等键长度不能超过 100")
	}
	if spec.ScheduledAt.IsZero() {
		spec.ScheduledAt = time.Now()
	}
	if len(spec.Payload) == 0 {
		spec.Payload = json.RawMessage(`{}`)
	}
	if !json.Valid(spec.Payload) {
		return nil, errors.BadRequest("ASYNC_TASK_PAYLOAD_INVALID", "任务载荷必须是有效 JSON")
	}
	return uc.repo.Enqueue(ctx, spec)
}

func (uc *AsyncTaskUsecase) List(ctx context.Context, req *pb.ListAsyncTasksRequest) ([]*pb.AsyncTask, int32, error) {
	return uc.repo.List(ctx, req)
}

func (uc *AsyncTaskUsecase) Stats(ctx context.Context, req *pb.GetAsyncTaskStatsRequest) (*pb.AsyncTaskStats, error) {
	return uc.repo.Stats(ctx, req)
}

func (uc *AsyncTaskUsecase) Get(ctx context.Context, id uint32) (*pb.AsyncTask, error) {
	return uc.repo.Get(ctx, id)
}

func (uc *AsyncTaskUsecase) Cancel(ctx context.Context, id uint32) error {
	return uc.repo.Cancel(ctx, id)
}

func (uc *AsyncTaskUsecase) Retry(ctx context.Context, id uint32) (*pb.AsyncTask, error) {
	return uc.repo.Retry(ctx, id)
}

func (uc *AsyncTaskUsecase) RunOne(ctx context.Context, workerID, queue string, lease time.Duration) (bool, error) {
	execution, err := uc.repo.Claim(ctx, workerID, queue, lease)
	if err != nil || execution == nil {
		return false, err
	}
	task := execution.Task
	handler, ok := uc.handlers[task.GetTaskType()]
	if !ok {
		err = fmt.Errorf("task handler %q is not registered", task.GetTaskType())
	} else {
		result, runErr := uc.callHandler(ctx, handler, execution.Payload)
		if runErr == nil {
			if err = uc.repo.Complete(ctx, task.GetId(), workerID, truncateTaskText(result, 1000)); err != nil {
				return true, err
			}
			return true, nil
		}
		err = runErr
	}

	nextRun := time.Now().Add(taskRetryBackoff(task.GetAttempts()))
	if failErr := uc.repo.Fail(ctx, task.GetId(), workerID, truncateTaskText(err.Error(), 4000), nextRun); failErr != nil {
		return true, failErr
	}
	uc.log.WithContext(ctx).Warnf("async task failed: id=%d type=%s attempt=%d err=%v",
		task.GetId(), task.GetTaskType(), task.GetAttempts(), err)
	return true, nil
}

func (uc *AsyncTaskUsecase) callHandler(ctx context.Context, handler AsyncTaskHandler, payload json.RawMessage) (result string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			uc.log.WithContext(ctx).Errorf("async task panic: type=%s panic=%v stack=%s", handler.Type(), recovered, debug.Stack())
			err = fmt.Errorf("task handler panic: %v", recovered)
		}
	}()
	return handler.Handle(ctx, payload)
}

func (uc *AsyncTaskUsecase) EnsureMaintenanceTasks(ctx context.Context, now time.Time) error {
	payload, _ := json.Marshal(retentionCleanupPayload{RetentionDays: 30, BatchSize: 500})
	_, err := uc.Enqueue(ctx, &AsyncTaskSpec{
		TaskType:       AsyncTaskTypeRetentionCleanup,
		Queue:          "maintenance",
		Priority:       -10,
		Payload:        payload,
		PayloadSummary: "清理 30 天前的终态任务，单批最多 500 条",
		IdempotencyKey: fmt.Sprintf("%s:%s", AsyncTaskTypeRetentionCleanup, now.Format("2006-01-02")),
		MaxAttempts:    3,
		ScheduledAt:    now,
	})
	return err
}

func taskRetryBackoff(attempt int32) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	backoff := 5 * time.Second * time.Duration(1<<min(attempt-1, 6))
	if backoff > 5*time.Minute {
		return 5 * time.Minute
	}
	return backoff
}

func truncateTaskText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

type retentionCleanupPayload struct {
	RetentionDays int `json:"retentionDays"`
	BatchSize     int `json:"batchSize"`
}

type retentionCleanupHandler struct{ repo AsyncTaskRepo }

type permissionCacheInvalidationPayload struct {
	Scope    string `json:"scope"`
	TenantID uint32 `json:"tenantId,omitempty"`
}

type permissionCacheInvalidationHandler struct {
	invalidator PermissionCacheInvalidator
}

func NewAsyncTaskHandlers(repo AsyncTaskRepo, invalidator PermissionCacheInvalidator, notification AsyncTaskHandler, webhookRepo WebhookRepo, logger log.Logger) []AsyncTaskHandler {
	handlers := []AsyncTaskHandler{
		&retentionCleanupHandler{repo: repo},
		&permissionCacheInvalidationHandler{invalidator: invalidator},
	}
	if notification != nil {
		handlers = append(handlers, notification)
	}
	handlers = append(handlers, NewWebhookAsyncTaskHandler(webhookRepo, logger))
	return handlers
}

func (h *retentionCleanupHandler) Type() string { return AsyncTaskTypeRetentionCleanup }

func (h *retentionCleanupHandler) Handle(ctx context.Context, payload json.RawMessage) (string, error) {
	var input retentionCleanupPayload
	if err := json.Unmarshal(payload, &input); err != nil {
		return "", fmt.Errorf("decode retention payload: %w", err)
	}
	if input.RetentionDays < 7 || input.RetentionDays > 3650 {
		return "", fmt.Errorf("retention days must be between 7 and 3650")
	}
	if input.BatchSize <= 0 || input.BatchSize > 5000 {
		return "", fmt.Errorf("batch size must be between 1 and 5000")
	}
	count, err := h.repo.PurgeTerminal(ctx, time.Now().AddDate(0, 0, -input.RetentionDays), input.BatchSize)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已清理 %d 条过期终态任务", count), nil
}

func (h *permissionCacheInvalidationHandler) Type() string {
	return AsyncTaskTypePermissionCacheInvalidate
}

func (h *permissionCacheInvalidationHandler) Handle(ctx context.Context, payload json.RawMessage) (string, error) {
	var input permissionCacheInvalidationPayload
	if err := json.Unmarshal(payload, &input); err != nil {
		return "", fmt.Errorf("decode permission cache invalidation payload: %w", err)
	}
	switch input.Scope {
	case "menu":
		if err := h.invalidator.InvalidateMenuPermissionCache(ctx); err != nil {
			return "", err
		}
		return "已刷新全局菜单权限缓存版本", nil
	case "tenant_package":
		if input.TenantID == 0 {
			return "", fmt.Errorf("tenant id is required for tenant package invalidation")
		}
		if err := h.invalidator.InvalidateTenantPackagePermissionCache(ctx, input.TenantID); err != nil {
			return "", err
		}
		return fmt.Sprintf("已刷新租户 %d 套餐权限缓存版本", input.TenantID), nil
	case "tenant_authorization":
		if input.TenantID == 0 {
			return "", fmt.Errorf("tenant id is required for tenant authorization invalidation")
		}
		if err := h.invalidator.InvalidateTenantAuthorizationCache(ctx, input.TenantID); err != nil {
			return "", err
		}
		return fmt.Sprintf("已刷新租户 %d 授权快照缓存版本", input.TenantID), nil
	default:
		return "", fmt.Errorf("unsupported permission cache invalidation scope %q", input.Scope)
	}
}
