package biz

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pb "backend-service/api/platform/service/v1"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"
)

type notificationRepoStub struct {
	template *pb.NotificationTemplate
	messages []*pb.NotificationMessage
	marked   []uint32
}

type notificationProviderRepoStub struct{}

func (*notificationProviderRepoStub) Create(context.Context, *pb.NotificationProvider) (*pb.NotificationProvider, error) {
	return nil, nil
}
func (*notificationProviderRepoStub) Update(context.Context, *pb.NotificationProvider) (*pb.NotificationProvider, error) {
	return nil, nil
}
func (*notificationProviderRepoStub) Delete(context.Context, uint32) error { return nil }
func (*notificationProviderRepoStub) Get(context.Context, uint32) (*pb.NotificationProvider, error) {
	return nil, nil
}
func (*notificationProviderRepoStub) List(context.Context, *pb.ListNotificationProvidersRequest) ([]*pb.NotificationProvider, int32, error) {
	return nil, 0, nil
}
func (*notificationProviderRepoStub) SetDefault(context.Context, uint32) (*pb.NotificationProvider, error) {
	return nil, nil
}
func (*notificationProviderRepoStub) ResolveChannel(_ context.Context, channel string) (*pb.NotificationProvider, json.RawMessage, error) {
	return &pb.NotificationProvider{Channel: channel, Status: convert.ToPointer(StorageProviderStatusEnabled)}, json.RawMessage("{}"), nil
}

func (r *notificationRepoStub) ListTemplates(context.Context, *pb.ListNotificationTemplatesRequest) ([]*pb.NotificationTemplate, int32, error) {
	return nil, 0, nil
}

func (r *notificationRepoStub) GetTemplate(context.Context, uint32) (*pb.NotificationTemplate, error) {
	return r.template, nil
}

func (r *notificationRepoStub) GetEnabledTemplateByCode(context.Context, uint32, string) (*pb.NotificationTemplate, error) {
	return r.template, nil
}

func (r *notificationRepoStub) CreateTemplate(_ context.Context, value *pb.NotificationTemplate) (*pb.NotificationTemplate, error) {
	r.template = value
	return value, nil
}

func (r *notificationRepoStub) UpdateTemplate(_ context.Context, value *pb.NotificationTemplate) (*pb.NotificationTemplate, error) {
	r.template = value
	return value, nil
}

func (*notificationRepoStub) DeleteTemplate(context.Context, uint32) error { return nil }

func (r *notificationRepoStub) CreateMessages(_ context.Context, values []*pb.NotificationMessage) (int, error) {
	r.messages = append(r.messages, values...)
	return len(values), nil
}

func (r *notificationRepoStub) ListMessages(_ context.Context, req *pb.ListNotificationMessagesRequest) ([]*pb.NotificationMessage, int32, error) {
	items := make([]*pb.NotificationMessage, 0, len(r.messages))
	for _, item := range r.messages {
		if req.RecipientUserId != nil && item.GetRecipientUserId() != req.GetRecipientUserId() {
			continue
		}
		items = append(items, item)
	}
	return items, int32(len(items)), nil
}

func (*notificationRepoStub) GetMessage(context.Context, uint32) (*pb.NotificationMessage, error) {
	return nil, nil
}

func (r *notificationRepoStub) CountUnread(_ context.Context, recipientUserID uint32) (int32, error) {
	var count int32
	for _, item := range r.messages {
		if item.GetRecipientUserId() == recipientUserID &&
			item.GetStatus() == pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD {
			count++
		}
	}
	return count, nil
}

func (r *notificationRepoStub) MarkRead(_ context.Context, ids []uint32, recipientUserID uint32) error {
	r.marked = append(r.marked, ids...)
	for _, item := range r.messages {
		if item.GetRecipientUserId() != recipientUserID {
			continue
		}
		if len(ids) > 0 && item.GetId() != ids[0] {
			continue
		}
		item.Status = pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_READ
	}
	return nil
}

type notificationTaskRepoStub struct {
	spec *AsyncTaskSpec
}

func (r *notificationTaskRepoStub) Enqueue(_ context.Context, spec *AsyncTaskSpec) (*pb.AsyncTask, error) {
	clone := *spec
	r.spec = &clone
	return &pb.AsyncTask{Id: 9, TaskType: spec.TaskType}, nil
}

func (*notificationTaskRepoStub) List(context.Context, *pb.ListAsyncTasksRequest) ([]*pb.AsyncTask, int32, error) {
	return nil, 0, nil
}

func (*notificationTaskRepoStub) Stats(context.Context, *pb.GetAsyncTaskStatsRequest) (*pb.AsyncTaskStats, error) {
	return nil, nil
}

func (*notificationTaskRepoStub) Get(context.Context, uint32) (*pb.AsyncTask, error) { return nil, nil }
func (*notificationTaskRepoStub) Cancel(context.Context, uint32) error               { return nil }
func (*notificationTaskRepoStub) Retry(context.Context, uint32) (*pb.AsyncTask, error) {
	return nil, nil
}
func (*notificationTaskRepoStub) Claim(context.Context, string, string, time.Duration) (*AsyncTaskExecution, error) {
	return nil, nil
}
func (*notificationTaskRepoStub) Complete(context.Context, uint32, string, string) error { return nil }
func (*notificationTaskRepoStub) Fail(context.Context, uint32, string, string, time.Time) error {
	return nil
}
func (*notificationTaskRepoStub) PurgeTerminal(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func TestNotificationUsecaseSendInAppEnqueuesAsyncTask(t *testing.T) {
	t.Parallel()

	repo := &notificationRepoStub{}
	tasks := &notificationTaskRepoStub{}
	uc := NewNotificationUsecase(repo, tasks, log.NewStdLogger(io.Discard))
	ctx := authn.ContextWithAuthUser(context.Background(), resourceQuotaTestUser{subject: "7", tenant: "10"})

	resp, err := uc.SendInApp(ctx, &pb.SendInAppNotificationRequest{
		RecipientUserIds: []uint32{7, 8},
		TemplateCode:     convert.ToPointer("system.welcome"),
		Variables:        convert.ToPointer(`{"userName":"admin"}`),
		IdempotencyKey:   convert.ToPointer("welcome:7"),
	})
	if err != nil {
		t.Fatalf("SendInApp() error = %v", err)
	}
	if resp.GetTaskId() != 9 || tasks.spec == nil || tasks.spec.TaskType != AsyncTaskTypeNotificationSend || tasks.spec.Queue != "notification" {
		t.Fatalf("queued task = %#v resp=%#v", tasks.spec, resp)
	}
	if tasks.spec.TenantID == nil || *tasks.spec.TenantID != 10 || tasks.spec.CreatedBy == nil || *tasks.spec.CreatedBy != 7 {
		t.Fatalf("queued identity = tenant:%v createdBy:%v", tasks.spec.TenantID, tasks.spec.CreatedBy)
	}
	var payload inAppNotificationPayload
	if err := json.Unmarshal(tasks.spec.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.TenantID != 10 || len(payload.RecipientUserIDs) != 2 || payload.TemplateCode != "system.welcome" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestNotificationInAppHandlerCreatesRenderedMessages(t *testing.T) {
	t.Parallel()

	repo := &notificationRepoStub{template: &pb.NotificationTemplate{
		Id:      3,
		Code:    "system.welcome",
		Channel: pb.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP,
		Title:   "欢迎 {{userName}}",
		Content: "租户 {{tenantName}} 已开通",
		Status:  enum.Status_STATUS_ENABLED.Enum(),
	}}
	providerRepo := &notificationProviderRepoStub{}
	resolver := NewNotificationSenderResolver(providerRepo, repo)
	handler := NewNotificationAsyncTaskHandler(repo, resolver)
	payload := []byte(`{"tenantId":10,"recipientUserIds":[7,8],"templateCode":"system.welcome","variables":"{\"userName\":\"admin\",\"tenantName\":\"演示租户\"}"}`)

	result, err := handler.Handle(context.Background(), payload)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !strings.Contains(result, "已发送 2 条通知") || len(repo.messages) != 2 {
		t.Fatalf("result=%q messages=%d", result, len(repo.messages))
	}
	if got := repo.messages[0]; got.GetTenantId() != 10 || got.GetTitle() != "欢迎 admin" || got.GetContent() != "租户 演示租户 已开通" || got.GetStatus() != pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD {
		t.Fatalf("message = %#v", got)
	}
}

func TestNotificationUsecaseMyNotifications(t *testing.T) {
	t.Parallel()

	repo := &notificationRepoStub{messages: []*pb.NotificationMessage{
		{Id: 1, RecipientUserId: 7, Status: pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD},
		{Id: 2, RecipientUserId: 8, Status: pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD},
	}}
	uc := NewNotificationUsecase(repo, &notificationTaskRepoStub{}, log.NewStdLogger(io.Discard))
	ctx := authn.ContextWithAuthUser(context.Background(), resourceQuotaTestUser{subject: "7", tenant: "10"})

	items, total, err := uc.ListMyNotifications(ctx, &pb.ListNotificationMessagesRequest{})
	if err != nil {
		t.Fatalf("ListMyNotifications() error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].GetRecipientUserId() != 7 {
		t.Fatalf("my notifications total=%d items=%#v", total, items)
	}
	unread, err := uc.CountMyUnread(ctx)
	if err != nil || unread != 1 {
		t.Fatalf("CountMyUnread() = %d, %v", unread, err)
	}
	if err := uc.MarkMyRead(ctx, []uint32{1}); err != nil {
		t.Fatalf("MarkMyRead() error = %v", err)
	}
	unread, _ = uc.CountMyUnread(ctx)
	if unread != 0 {
		t.Fatalf("unread after mark = %d, want 0", unread)
	}
}
