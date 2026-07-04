package data

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	pb "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

func newAsyncTaskRepoForTest(t *testing.T) (*asyncTaskRepo, func()) {
	t.Helper()
	client := newTestClient(t)
	repo := NewAsyncTaskRepo(&Data{db: client}, log.NewStdLogger(io.Discard)).(*asyncTaskRepo)
	return repo, func() { client.Close() }
}

func taskSpec(key string) *biz.AsyncTaskSpec {
	return &biz.AsyncTaskSpec{
		TaskType:       biz.AsyncTaskTypeRetentionCleanup,
		Queue:          "maintenance",
		Payload:        json.RawMessage(`{"retentionDays":30,"batchSize":100}`),
		PayloadSummary: "cleanup",
		IdempotencyKey: key,
		MaxAttempts:    3,
		ScheduledAt:    time.Now().Add(-time.Second),
	}
}

func TestAsyncTaskRepoIdempotentEnqueueAndExclusiveClaim(t *testing.T) {
	repo, closeRepo := newAsyncTaskRepoForTest(t)
	defer closeRepo()
	ctx := systemContext()

	first, err := repo.Enqueue(ctx, taskSpec("daily:2026-06-30"))
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	second, err := repo.Enqueue(ctx, taskSpec("daily:2026-06-30"))
	if err != nil {
		t.Fatalf("idempotent enqueue: %v", err)
	}
	if first.GetId() != second.GetId() {
		t.Fatalf("idempotent task ids differ: %d != %d", first.GetId(), second.GetId())
	}

	claimed, err := repo.Claim(ctx, "worker-a", "maintenance", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim task = %+v, %v", claimed, err)
	}
	if claimed.Task.GetAttempts() != 1 || claimed.Task.GetStatus() != pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING {
		t.Fatalf("claimed task = %+v", claimed.Task)
	}
	other, err := repo.Claim(ctx, "worker-b", "maintenance", time.Minute)
	if err != nil || other != nil {
		t.Fatalf("second claim = %+v, %v", other, err)
	}
	if err = repo.Complete(ctx, first.GetId(), "worker-b", "wrong owner"); !errors.IsConflict(err) {
		t.Fatalf("stale worker complete error = %v, want conflict", err)
	}
	if err = repo.Complete(ctx, first.GetId(), "worker-a", "done"); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	completed, err := repo.Get(ctx, first.GetId())
	if err != nil || completed.GetStatus() != pb.AsyncTaskStatus_ASYNC_TASK_STATUS_SUCCEEDED {
		t.Fatalf("completed task = %+v, %v", completed, err)
	}
}

func TestAsyncTaskRepoScopesIdempotencyByTenant(t *testing.T) {
	repo, closeRepo := newAsyncTaskRepoForTest(t)
	defer closeRepo()
	ctx := systemContext()
	tenantOne := uint32(1)
	tenantTwo := uint32(2)
	firstSpec := taskSpec("same-business-key")
	firstSpec.TenantID = &tenantOne
	secondSpec := taskSpec("same-business-key")
	secondSpec.TenantID = &tenantTwo

	first, err := repo.Enqueue(ctx, firstSpec)
	if err != nil {
		t.Fatalf("enqueue tenant one task: %v", err)
	}
	second, err := repo.Enqueue(ctx, secondSpec)
	if err != nil {
		t.Fatalf("enqueue tenant two task: %v", err)
	}
	if first.GetId() == second.GetId() {
		t.Fatalf("different tenants shared idempotent task id %d", first.GetId())
	}
	duplicate, err := repo.Enqueue(ctx, firstSpec)
	if err != nil || duplicate.GetId() != first.GetId() {
		t.Fatalf("same tenant duplicate = %+v, %v", duplicate, err)
	}
}

func TestAsyncTaskRepoRetriesAndLeaseRecovery(t *testing.T) {
	repo, closeRepo := newAsyncTaskRepoForTest(t)
	defer closeRepo()
	ctx := systemContext()
	task, err := repo.Enqueue(ctx, taskSpec("retry-task"))
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}

	claimed, err := repo.Claim(ctx, "worker-a", "maintenance", time.Millisecond)
	if err != nil || claimed == nil {
		t.Fatalf("first claim = %+v, %v", claimed, err)
	}
	time.Sleep(2 * time.Millisecond)
	reclaimed, err := repo.Claim(ctx, "worker-b", "maintenance", time.Minute)
	if err != nil || reclaimed == nil || reclaimed.Task.GetAttempts() != 2 {
		t.Fatalf("reclaimed task = %+v, %v", reclaimed, err)
	}
	if err = repo.Fail(ctx, task.GetId(), "worker-b", "temporary", time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("fail task for retry: %v", err)
	}
	pending, err := repo.Get(ctx, task.GetId())
	if err != nil || pending.GetStatus() != pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING {
		t.Fatalf("pending retry = %+v, %v", pending, err)
	}
	third, err := repo.Claim(ctx, "worker-c", "maintenance", time.Minute)
	if err != nil || third == nil || third.Task.GetAttempts() != 3 {
		t.Fatalf("third claim = %+v, %v", third, err)
	}
	if err = repo.Fail(ctx, task.GetId(), "worker-c", "terminal", time.Now()); err != nil {
		t.Fatalf("terminal fail: %v", err)
	}
	failed, err := repo.Get(ctx, task.GetId())
	if err != nil || failed.GetStatus() != pb.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED {
		t.Fatalf("failed task = %+v, %v", failed, err)
	}
	retried, err := repo.Retry(ctx, task.GetId())
	if err != nil || retried.GetStatus() != pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING || retried.GetAttempts() != 0 {
		t.Fatalf("manual retry = %+v, %v", retried, err)
	}
}

func TestAsyncTaskRepoCancelAndRetention(t *testing.T) {
	repo, closeRepo := newAsyncTaskRepoForTest(t)
	defer closeRepo()
	ctx := systemContext()
	task, err := repo.Enqueue(ctx, taskSpec("cancel-task"))
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	if err = repo.Cancel(ctx, task.GetId()); err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	if err = repo.Cancel(ctx, task.GetId()); !errors.IsConflict(err) {
		t.Fatalf("cancel terminal task error = %v, want conflict", err)
	}

	old := time.Now().AddDate(0, 0, -40)
	repo.Data.DB(ctx).AsyncTask.UpdateOneID(task.GetId()).
		SetCompletedAt(old).
		SaveX(ctx)
	count, err := repo.PurgeTerminal(ctx, time.Now().AddDate(0, 0, -30), 10)
	if err != nil || count != 1 {
		t.Fatalf("purge terminal = %d, %v", count, err)
	}
	if _, err = repo.Get(ctx, task.GetId()); !errors.IsNotFound(err) {
		t.Fatalf("get purged task error = %v, want not found", err)
	}
}

func TestAsyncTaskRepoDoesNotExceedMaxAttemptsAfterLeaseExpiry(t *testing.T) {
	repo, closeRepo := newAsyncTaskRepoForTest(t)
	defer closeRepo()
	ctx := systemContext()
	expired := time.Now().Add(-time.Minute)
	task := repo.Data.DB(ctx).AsyncTask.Create().
		SetTaskType(biz.AsyncTaskTypeRetentionCleanup).
		SetQueue("maintenance").
		SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_RUNNING)).
		SetPayload(`{"retentionDays":30,"batchSize":100}`).
		SetScheduledAt(time.Now().Add(-time.Hour)).
		SetAttempts(3).
		SetMaxAttempts(3).
		SetLeaseOwner("dead-worker").
		SetLeaseExpiresAt(expired).
		SaveX(ctx)

	claimed, err := repo.Claim(ctx, "new-worker", "maintenance", time.Minute)
	if err != nil || claimed != nil {
		t.Fatalf("claim exhausted task = %+v, %v", claimed, err)
	}
	failed, err := repo.Get(ctx, task.ID)
	if err != nil || failed.GetStatus() != pb.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED {
		t.Fatalf("exhausted task = %+v, %v", failed, err)
	}
	if failed.GetAttempts() != 3 {
		t.Fatalf("attempts = %d, want 3", failed.GetAttempts())
	}
}
