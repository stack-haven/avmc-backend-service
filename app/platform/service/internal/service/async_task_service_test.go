package service

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/platform/service/internal/biz"

	pb "backend-service/api/platform/service/v1"
)

var discardLogger = log.NewStdLogger(io.Discard)

type asyncTaskRepoStub struct {
	listReq   *pb.ListAsyncTasksRequest
	listItems []*pb.AsyncTask
	listTotal int32
	statsReq  *pb.GetAsyncTaskStatsRequest
	stats     *pb.AsyncTaskStats
	getID     uint32
	getTask   *pb.AsyncTask
	cancelID  uint32
	retryID   uint32
	retryTask *pb.AsyncTask
}

func (*asyncTaskRepoStub) Enqueue(context.Context, *biz.AsyncTaskSpec) (*pb.AsyncTask, error) {
	return nil, nil
}

func (r *asyncTaskRepoStub) List(_ context.Context, req *pb.ListAsyncTasksRequest) ([]*pb.AsyncTask, int32, error) {
	r.listReq = req
	return r.listItems, r.listTotal, nil
}

func (r *asyncTaskRepoStub) Stats(_ context.Context, req *pb.GetAsyncTaskStatsRequest) (*pb.AsyncTaskStats, error) {
	r.statsReq = req
	if r.stats != nil {
		return r.stats, nil
	}
	return &pb.AsyncTaskStats{}, nil
}

func (r *asyncTaskRepoStub) Get(context.Context, uint32) (*pb.AsyncTask, error) {
	r.getID = 7
	if r.getTask != nil {
		return r.getTask, nil
	}
	return &pb.AsyncTask{Id: 7}, nil
}

func (r *asyncTaskRepoStub) Cancel(_ context.Context, id uint32) error {
	r.cancelID = id
	return nil
}

func (r *asyncTaskRepoStub) Retry(_ context.Context, id uint32) (*pb.AsyncTask, error) {
	r.retryID = id
	if r.retryTask != nil {
		return r.retryTask, nil
	}
	return &pb.AsyncTask{Id: id}, nil
}

func (*asyncTaskRepoStub) Claim(context.Context, string, string, time.Duration) (*biz.AsyncTaskExecution, error) {
	return nil, nil
}

func (*asyncTaskRepoStub) Complete(context.Context, uint32, string, string) error { return nil }
func (*asyncTaskRepoStub) Fail(context.Context, uint32, string, string, time.Time) error {
	return nil
}
func (*asyncTaskRepoStub) PurgeTerminal(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

type noopTaskHandler struct{}

func (noopTaskHandler) Type() string { return "noop" }
func (noopTaskHandler) Handle(context.Context, json.RawMessage) (string, error) {
	return "", nil
}

func ptrString(value string) *string { return &value }

func newTestAsyncTaskService(repo biz.AsyncTaskRepo) *AsyncTaskServiceService {
	return NewAsyncTaskServiceService(
		biz.NewAsyncTaskUsecase(repo, []biz.AsyncTaskHandler{noopTaskHandler{}}, discardLogger),
		discardLogger,
	)
}

func TestAsyncTaskServiceListSetsNextPageToken(t *testing.T) {
	t.Parallel()

	repo := &asyncTaskRepoStub{
		listItems: []*pb.AsyncTask{{Id: 1}, {Id: 2}},
		listTotal: 5,
	}
	service := newTestAsyncTaskService(repo)

	resp, err := service.ListAsyncTasks(context.Background(), &pb.ListAsyncTasksRequest{PageToken: "2"})
	if err != nil {
		t.Fatalf("ListAsyncTasks() error = %v", err)
	}
	if repo.listReq.GetPageToken() != "2" {
		t.Fatalf("page token passed to usecase = %q, want 2", repo.listReq.GetPageToken())
	}
	if len(resp.GetItems()) != 2 || resp.GetTotal() != 5 || resp.GetNextPageToken() != "4" {
		t.Fatalf("response items=%d total=%d next=%q", len(resp.GetItems()), resp.GetTotal(), resp.GetNextPageToken())
	}
}

func TestAsyncTaskServiceGetStats(t *testing.T) {
	t.Parallel()

	repo := &asyncTaskRepoStub{
		stats: &pb.AsyncTaskStats{Total: 9, PendingOverdue: 2},
	}
	service := newTestAsyncTaskService(repo)

	resp, err := service.GetAsyncTaskStats(context.Background(), &pb.GetAsyncTaskStatsRequest{Queue: ptrString("maintenance"), PendingOverdueSeconds: 60})
	if err != nil {
		t.Fatalf("GetAsyncTaskStats() error = %v", err)
	}
	if repo.statsReq.GetQueue() != "maintenance" || repo.statsReq.GetPendingOverdueSeconds() != 60 {
		t.Fatalf("stats request = %+v", repo.statsReq)
	}
	if resp.GetStats().GetTotal() != 9 || resp.GetStats().GetPendingOverdue() != 2 {
		t.Fatalf("stats response = %+v", resp.GetStats())
	}
}

func TestAsyncTaskServiceCommands(t *testing.T) {
	t.Parallel()

	repo := &asyncTaskRepoStub{getTask: &pb.AsyncTask{Id: 7}, retryTask: &pb.AsyncTask{Id: 8}}
	service := newTestAsyncTaskService(repo)

	got, err := service.GetAsyncTask(context.Background(), &pb.GetAsyncTaskRequest{Id: 7})
	if err != nil {
		t.Fatalf("GetAsyncTask() error = %v", err)
	}
	if repo.getID != 7 || got.GetTask().GetId() != 7 {
		t.Fatalf("getID=%d task=%v", repo.getID, got.GetTask())
	}

	if _, err := service.CancelAsyncTask(context.Background(), &pb.CancelAsyncTaskRequest{Id: 7}); err != nil {
		t.Fatalf("CancelAsyncTask() error = %v", err)
	}
	if repo.cancelID != 7 {
		t.Fatalf("cancelID=%d, want 7", repo.cancelID)
	}

	retried, err := service.RetryAsyncTask(context.Background(), &pb.RetryAsyncTaskRequest{Id: 8})
	if err != nil {
		t.Fatalf("RetryAsyncTask() error = %v", err)
	}
	if repo.retryID != 8 || retried.GetTask().GetId() != 8 {
		t.Fatalf("retryID=%d task=%v", repo.retryID, retried.GetTask())
	}
}
