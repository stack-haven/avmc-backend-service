package biz

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	pb "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type panicTaskHandler struct{}

func (panicTaskHandler) Type() string { return "test.panic" }
func (panicTaskHandler) Handle(context.Context, json.RawMessage) (string, error) {
	panic("boom")
}

type permissionCacheInvalidatorStub struct {
	menuCalls   int
	tenantCalls []uint32
}

func (s *permissionCacheInvalidatorStub) InvalidateMenuPermissionCache(context.Context) error {
	s.menuCalls++
	return nil
}

func (s *permissionCacheInvalidatorStub) InvalidateTenantPackagePermissionCache(_ context.Context, tenantID uint32) error {
	s.tenantCalls = append(s.tenantCalls, tenantID)
	return nil
}

type taskRepoStub struct {
	claimed *AsyncTaskExecution
	failed  bool
}

func (r *taskRepoStub) Enqueue(context.Context, *AsyncTaskSpec) (*pb.AsyncTask, error) {
	return &pb.AsyncTask{Id: 1}, nil
}
func (r *taskRepoStub) List(context.Context, *pb.ListAsyncTasksRequest) ([]*pb.AsyncTask, int32, error) {
	return nil, 0, nil
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
func (r *taskRepoStub) Complete(context.Context, uint32, string, string) error { return nil }
func (r *taskRepoStub) Fail(context.Context, uint32, string, string, time.Time) error {
	r.failed = true
	return nil
}
func (r *taskRepoStub) PurgeTerminal(context.Context, time.Time, int) (int, error) {
	return 0, nil
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

func TestAsyncTaskUsecaseRecoversHandlerPanic(t *testing.T) {
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

func TestPermissionCacheInvalidationHandler(t *testing.T) {
	invalidator := &permissionCacheInvalidatorStub{}
	handler := &permissionCacheInvalidationHandler{invalidator: invalidator}

	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"menu"}`)); err != nil {
		t.Fatalf("invalidate menu cache: %v", err)
	}
	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"tenant_package","tenantId":7}`)); err != nil {
		t.Fatalf("invalidate tenant package cache: %v", err)
	}
	if invalidator.menuCalls != 1 {
		t.Fatalf("menu invalidation calls = %d, want 1", invalidator.menuCalls)
	}
	if len(invalidator.tenantCalls) != 1 || invalidator.tenantCalls[0] != 7 {
		t.Fatalf("tenant invalidation calls = %v, want [7]", invalidator.tenantCalls)
	}
	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"tenant_package"}`)); err == nil {
		t.Fatal("tenant package invalidation without tenant id succeeded")
	}
	if _, err := handler.Handle(context.Background(), json.RawMessage(`{"scope":"arbitrary"}`)); err == nil {
		t.Fatal("unsupported cache invalidation scope succeeded")
	}
}
