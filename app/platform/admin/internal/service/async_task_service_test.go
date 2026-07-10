package service

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type asyncTaskRepoStub struct {
	listReq   *pbCore.ListAsyncTasksRequest
	listItems []*pbCore.AsyncTask
	listTotal int32
	getID     uint32
	getTask   *pbCore.AsyncTask
	cancelID  uint32
	retryID   uint32
	retryTask *pbCore.AsyncTask
}

func (*asyncTaskRepoStub) Enqueue(context.Context, *biz.AsyncTaskSpec) (*pbCore.AsyncTask, error) {
	return nil, nil
}

func (r *asyncTaskRepoStub) List(_ context.Context, req *pbCore.ListAsyncTasksRequest) ([]*pbCore.AsyncTask, int32, error) {
	r.listReq = req
	return r.listItems, r.listTotal, nil
}

func (r *asyncTaskRepoStub) Get(context.Context, uint32) (*pbCore.AsyncTask, error) {
	r.getID = 7
	if r.getTask != nil {
		return r.getTask, nil
	}
	return &pbCore.AsyncTask{Id: 7}, nil
}

func (r *asyncTaskRepoStub) Cancel(_ context.Context, id uint32) error {
	r.cancelID = id
	return nil
}

func (r *asyncTaskRepoStub) Retry(_ context.Context, id uint32) (*pbCore.AsyncTask, error) {
	r.retryID = id
	if r.retryTask != nil {
		return r.retryTask, nil
	}
	return &pbCore.AsyncTask{Id: id}, nil
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

func TestAsyncTaskServiceListSetsNextPageToken(t *testing.T) {
	t.Parallel()

	repo := &asyncTaskRepoStub{
		listItems: []*pbCore.AsyncTask{{Id: 1}, {Id: 2}},
		listTotal: 5,
	}
	service := NewAsyncTaskServiceService(biz.NewAsyncTaskUsecase(repo, []biz.AsyncTaskHandler{noopTaskHandler{}}, log.NewStdLogger(io.Discard)))

	resp, err := service.ListAsyncTasks(context.Background(), &pbCore.ListAsyncTasksRequest{PageToken: "2"})
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

func TestAsyncTaskServiceCommands(t *testing.T) {
	t.Parallel()

	repo := &asyncTaskRepoStub{getTask: &pbCore.AsyncTask{Id: 7}, retryTask: &pbCore.AsyncTask{Id: 8}}
	service := NewAsyncTaskServiceService(biz.NewAsyncTaskUsecase(repo, []biz.AsyncTaskHandler{noopTaskHandler{}}, log.NewStdLogger(io.Discard)))

	got, err := service.GetAsyncTask(context.Background(), &pbCore.GetAsyncTaskRequest{Id: 7})
	if err != nil {
		t.Fatalf("GetAsyncTask() error = %v", err)
	}
	if repo.getID != 7 || got.GetTask().GetId() != 7 {
		t.Fatalf("getID=%d task=%v", repo.getID, got.GetTask())
	}

	if _, err := service.CancelAsyncTask(context.Background(), &pbCore.CancelAsyncTaskRequest{Id: 7}); err != nil {
		t.Fatalf("CancelAsyncTask() error = %v", err)
	}
	if repo.cancelID != 7 {
		t.Fatalf("cancelID=%d, want 7", repo.cancelID)
	}

	retried, err := service.RetryAsyncTask(context.Background(), &pbCore.RetryAsyncTaskRequest{Id: 8})
	if err != nil {
		t.Fatalf("RetryAsyncTask() error = %v", err)
	}
	if repo.retryID != 8 || retried.GetTask().GetId() != 8 {
		t.Fatalf("retryID=%d task=%v", repo.retryID, retried.GetTask())
	}
}
