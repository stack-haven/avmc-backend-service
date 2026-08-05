package biz

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/core/service/v1"
)

type panicTaskHandler struct{}

func (panicTaskHandler) Type() string { return "test.panic" }
func (panicTaskHandler) Handle(context.Context, json.RawMessage) (string, error) {
	panic("boom")
}

type successTaskHandler struct{}

func (successTaskHandler) Type() string { return "test.success" }
func (successTaskHandler) Handle(context.Context, json.RawMessage) (string, error) {
	return "done", nil
}

type permissionCacheInvalidatorStub struct {
	menuCalls                int
	tenantCalls              []uint32
	tenantAuthorizationCalls []uint32
}

func (s *permissionCacheInvalidatorStub) InvalidateMenuPermissionCache(context.Context) error {
	s.menuCalls++
	return nil
}

func (s *permissionCacheInvalidatorStub) InvalidateTenantPackagePermissionCache(_ context.Context, tenantID uint32) error {
	s.tenantCalls = append(s.tenantCalls, tenantID)
	return nil
}

func (s *permissionCacheInvalidatorStub) InvalidateTenantAuthorizationCache(_ context.Context, tenantID uint32) error {
	s.tenantAuthorizationCalls = append(s.tenantAuthorizationCalls, tenantID)
	return nil
}

type taskRepoStub struct {
	claimed     *AsyncTaskExecution
	enqueued    *AsyncTaskSpec
	statsReq    *pb.GetAsyncTaskStatsRequest
	stats       *pb.AsyncTaskStats
	completedID uint32
	failed      bool
	failedID    uint32
	failReason  string
	purgeBefore time.Time
	purgeLimit  int
	purgeCount  int
	purgeErr    error
}

func (r *taskRepoStub) Enqueue(_ context.Context, spec *AsyncTaskSpec) (*pb.AsyncTask, error) {
	clone := *spec
	if spec.Payload != nil {
		clone.Payload = append(json.RawMessage(nil), spec.Payload...)
	}
	r.enqueued = &clone
	return &pb.AsyncTask{Id: 1}, nil
}
func (r *taskRepoStub) List(context.Context, *pb.ListAsyncTasksRequest) ([]*pb.AsyncTask, int32, error) {
	return nil, 0, nil
}
func (r *taskRepoStub) Stats(_ context.Context, req *pb.GetAsyncTaskStatsRequest) (*pb.AsyncTaskStats, error) {
	r.statsReq = req
	if r.stats != nil {
		return r.stats, nil
	}
	return &pb.AsyncTaskStats{}, nil
}
func (r *taskRepoStub) Get(context.Context, uint32) (*pb.AsyncTask, error) { return nil, nil }
func (r *taskRepoStub) Cancel(context.Context, uint32) error               { return nil }
func (r *taskRepoStub) Retry(context.Context, uint32) (*pb.AsyncTask, error) {
	return nil, nil
}
func (r *taskRepoStub) Claim(context.Context, string, string, time.Duration) (*AsyncTaskExecution, error) {
	item := r.claimed
	r.claimed = nil
	return item, nil
}
func (r *taskRepoStub) Complete(_ context.Context, id uint32, _, _ string) error {
	r.completedID = id
	return nil
}
func (r *taskRepoStub) Fail(_ context.Context, id uint32, _, reason string, _ time.Time) error {
	r.failed = true
	r.failedID = id
	r.failReason = reason
	return nil
}
func (r *taskRepoStub) PurgeTerminal(_ context.Context, before time.Time, limit int) (int, error) {
	r.purgeBefore = before
	r.purgeLimit = limit
	return r.purgeCount, r.purgeErr
}

func TestAsyncTaskUsecaseRejectsUnregisteredType(t *testing.T) {
	uc := NewAsyncTaskUsecase(&taskRepoStub{}, nil, log.NewStdLogger(io.Discard))
	_, err := uc.Enqueue(context.Background(), &AsyncTaskSpec{
		TaskType: "unknown",
		Payload:  json.RawMessage(`{}`),
	})
	if !errors.IsBadRequest(err) {
		t.Fatalf("enqueue error = %v, want bad request", err)
	}
}

func TestAsyncTaskUsecaseEnqueueAppliesDefaultsAndValidatesInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    *AsyncTaskSpec
		wantErr bool
	}{
		{name: "nil spec", wantErr: true},
		{name: "invalid max attempts", spec: &AsyncTaskSpec{TaskType: "test.success", MaxAttempts: 11}, wantErr: true},
		{name: "long idempotency key", spec: &AsyncTaskSpec{TaskType: "test.success", IdempotencyKey: strings.Repeat("a", 101)}, wantErr: true},
		{name: "invalid json payload", spec: &AsyncTaskSpec{TaskType: "test.success", Payload: json.RawMessage(`{bad`)}, wantErr: true},
		{name: "default values", spec: &AsyncTaskSpec{TaskType: "test.success"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &taskRepoStub{}
			uc := NewAsyncTaskUsecase(repo, []AsyncTaskHandler{successTaskHandler{}}, log.NewStdLogger(io.Discard))
			_, err := uc.Enqueue(context.Background(), tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Enqueue() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if repo.enqueued != nil {
					t.Fatal("invalid async task was enqueued")
				}
				return
			}
			if repo.enqueued.Queue != "default" || repo.enqueued.MaxAttempts != 3 || string(repo.enqueued.Payload) != "{}" || repo.enqueued.ScheduledAt.IsZero() {
				t.Fatalf("enqueued defaults = queue:%q attempts:%d payload:%s scheduled:%v",
					repo.enqueued.Queue, repo.enqueued.MaxAttempts, repo.enqueued.Payload, repo.enqueued.ScheduledAt)
			}
		})
	}
}

func TestAsyncTaskUsecaseRunOneCompletesSuccessfulTask(t *testing.T) {
	t.Parallel()

	repo := &taskRepoStub{
		claimed: &AsyncTaskExecution{
			Task:    &pb.AsyncTask{Id: 3, TaskType: "test.success", Attempts: 1},
			Payload: json.RawMessage(`{}`),
		},
	}
	uc := NewAsyncTaskUsecase(repo, []AsyncTaskHandler{successTaskHandler{}}, log.NewStdLogger(io.Discard))

	handled, err := uc.RunOne(context.Background(), "worker", "default", time.Minute)
	if err != nil || !handled {
		t.Fatalf("RunOne() = %v, %v", handled, err)
	}
	if repo.completedID != 3 || repo.failed {
		t.Fatalf("completedID=%d failed=%v", repo.completedID, repo.failed)
	}
}

func TestAsyncTaskUsecaseStatsPassThrough(t *testing.T) {
	t.Parallel()

	queue := "maintenance"
	repo := &taskRepoStub{stats: &pb.AsyncTaskStats{Total: 4, RetryPressure: 1}}
	uc := NewAsyncTaskUsecase(repo, []AsyncTaskHandler{successTaskHandler{}}, log.NewStdLogger(io.Discard))

	got, err := uc.Stats(context.Background(), &pb.GetAsyncTaskStatsRequest{Queue: &queue, PendingOverdueSeconds: 120})
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if repo.statsReq.GetQueue() != queue || repo.statsReq.GetPendingOverdueSeconds() != 120 {
		t.Fatalf("stats request = %+v", repo.statsReq)
	}
	if got.GetTotal() != 4 || got.GetRetryPressure() != 1 {
		t.Fatalf("stats = %+v", got)
	}
}

func TestAsyncTaskUsecaseRunOneFailsUnregisteredClaimedTask(t *testing.T) {
	t.Parallel()

	repo := &taskRepoStub{
		claimed: &AsyncTaskExecution{
			Task:    &pb.AsyncTask{Id: 4, TaskType: "test.missing", Attempts: 1},
			Payload: json.RawMessage(`{}`),
		},
	}
	uc := NewAsyncTaskUsecase(repo, nil, log.NewStdLogger(io.Discard))

	handled, err := uc.RunOne(context.Background(), "worker", "default", time.Minute)
	if err != nil || !handled {
		t.Fatalf("RunOne() = %v, %v", handled, err)
	}
	if !repo.failed || repo.failedID != 4 || !strings.Contains(repo.failReason, "not registered") {
		t.Fatalf("failed=%v id=%d reason=%q", repo.failed, repo.failedID, repo.failReason)
	}
}

func TestAsyncTaskUsecaseRecoversHandlerPanic(t *testing.T) {
	t.Parallel()

	repo := &taskRepoStub{
		claimed: &AsyncTaskExecution{
			Task: &pb.AsyncTask{
				Id:          1,
				TaskType:    "test.panic",
				Attempts:    1,
				MaxAttempts: 3,
			},
			Payload: json.RawMessage(`{}`),
		},
	}
	uc := NewAsyncTaskUsecase(repo, []AsyncTaskHandler{panicTaskHandler{}}, log.NewStdLogger(io.Discard))
	handled, err := uc.RunOne(context.Background(), "worker", "default", time.Minute)
	if err != nil || !handled {
		t.Fatalf("run one = %v, %v", handled, err)
	}
	if !repo.failed {
		t.Fatal("panicking task was not marked failed/retryable")
	}
}

func TestAsyncTaskUsecaseEnsureMaintenanceTasks(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 8, 9, 30, 0, 0, time.UTC)
	repo := &taskRepoStub{}
	uc := NewAsyncTaskUsecase(repo, NewAsyncTaskHandlers(repo, &permissionCacheInvalidatorStub{}, nil, nil, log.DefaultLogger), log.NewStdLogger(io.Discard))

	if err := uc.EnsureMaintenanceTasks(context.Background(), now); err != nil {
		t.Fatalf("EnsureMaintenanceTasks() error = %v", err)
	}
	if repo.enqueued.TaskType != AsyncTaskTypeRetentionCleanup ||
		repo.enqueued.Queue != "maintenance" ||
		repo.enqueued.IdempotencyKey != "system.task_retention_cleanup:2026-07-08" ||
		repo.enqueued.ScheduledAt != now {
		t.Fatalf("maintenance task spec = %#v", repo.enqueued)
	}
}

func TestRetentionCleanupHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload json.RawMessage
		wantErr bool
	}{
		{name: "invalid json", payload: json.RawMessage(`{bad`), wantErr: true},
		{name: "retention too short", payload: json.RawMessage(`{"retentionDays":1,"batchSize":100}`), wantErr: true},
		{name: "batch too large", payload: json.RawMessage(`{"retentionDays":30,"batchSize":5001}`), wantErr: true},
		{name: "repo error", payload: json.RawMessage(`{"retentionDays":30,"batchSize":100}`), wantErr: true},
		{name: "success", payload: json.RawMessage(`{"retentionDays":30,"batchSize":100}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &taskRepoStub{purgeCount: 8}
			if tt.name == "repo error" {
				repo.purgeErr = stderrors.New("purge failed")
			}
			handler := &retentionCleanupHandler{repo: repo}
			result, err := handler.Handle(context.Background(), tt.payload)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Handle() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if repo.purgeLimit != 100 || repo.purgeBefore.IsZero() || !strings.Contains(result, "8") {
				t.Fatalf("purge limit=%d before=%v result=%q", repo.purgeLimit, repo.purgeBefore, result)
			}
		})
	}
}

func TestAsyncTaskHelpers(t *testing.T) {
	t.Parallel()

	if got := taskRetryBackoff(0); got != 5*time.Second {
		t.Fatalf("taskRetryBackoff(0) = %v, want 5s", got)
	}
	if got := taskRetryBackoff(8); got != 5*time.Minute {
		t.Fatalf("taskRetryBackoff(8) = %v, want 5m", got)
	}
	if got := truncateTaskText("  abc  ", 10); got != "abc" {
		t.Fatalf("truncateTaskText trim = %q, want abc", got)
	}
	if got := truncateTaskText("abcdef", 3); got != "abc" {
		t.Fatalf("truncateTaskText limit = %q, want abc", got)
	}
}

func TestPermissionCacheInvalidationHandler(t *testing.T) {
	t.Parallel()

	invalidator := &permissionCacheInvalidatorStub{}
	handler := &permissionCacheInvalidationHandler{invalidator: invalidator}

	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"menu"}`)); err != nil {
		t.Fatalf("invalidate menu cache: %v", err)
	}
	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"tenant_package","tenantId":7}`)); err != nil {
		t.Fatalf("invalidate tenant package cache: %v", err)
	}
	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"tenant_authorization","tenantId":8}`)); err != nil {
		t.Fatalf("invalidate tenant authorization cache: %v", err)
	}
	if invalidator.menuCalls != 1 {
		t.Fatalf("menu invalidation calls = %d, want 1", invalidator.menuCalls)
	}
	if len(invalidator.tenantCalls) != 1 || invalidator.tenantCalls[0] != 7 {
		t.Fatalf("tenant invalidation calls = %v, want [7]", invalidator.tenantCalls)
	}
	if len(invalidator.tenantAuthorizationCalls) != 1 || invalidator.tenantAuthorizationCalls[0] != 8 {
		t.Fatalf("tenant authorization invalidation calls = %v, want [8]", invalidator.tenantAuthorizationCalls)
	}
	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"tenant_package"}`)); err == nil {
		t.Fatal("tenant package invalidation without tenant id succeeded")
	}
	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"tenant_authorization"}`)); err == nil {
		t.Fatal("tenant authorization invalidation without tenant id succeeded")
	}
	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"arbitrary"}`)); err == nil {
		t.Fatal("unsupported cache invalidation scope succeeded")
	}
}
