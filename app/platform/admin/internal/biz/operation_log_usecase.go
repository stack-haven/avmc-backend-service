package biz

import (
	"context"
	"encoding/json"
	"strings"

	pb "backend-service/api/core/service/v1"
)

type OperationLogRepo interface {
	Append(context.Context, *pb.OperationLog) error
	List(context.Context, *pb.ListOperationLogsRequest) ([]*pb.OperationLog, int32, error)
	Get(context.Context, uint64) (*pb.OperationLog, error)
}

type OperationLogUsecase struct{ repo OperationLogRepo }

func NewOperationLogUsecase(repo OperationLogRepo) *OperationLogUsecase {
	return &OperationLogUsecase{repo: repo}
}
func (uc *OperationLogUsecase) Record(ctx context.Context, event *pb.OperationLog) error {
	event.RequestSummary = auditStringPtr(redactJSON(event.GetRequestSummary()))
	event.BeforeData = auditStringPtr(redactJSON(event.GetBeforeData()))
	event.AfterData = auditStringPtr(redactJSON(event.GetAfterData()))
	return uc.repo.Append(ctx, event)
}
func (uc *OperationLogUsecase) List(ctx context.Context, req *pb.ListOperationLogsRequest) ([]*pb.OperationLog, int32, error) {
	return uc.repo.List(ctx, req)
}
func (uc *OperationLogUsecase) Get(ctx context.Context, id uint64) (*pb.OperationLog, error) {
	return uc.repo.Get(ctx, id)
}

var sensitiveKeys = map[string]struct{}{
	"password": {}, "token": {}, "access_token": {}, "refresh_token": {},
	"secret": {}, "client_secret": {}, "captcha": {}, "captcha_code": {},
	"sms_code": {}, "verification_code": {},
}

func redactJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return "[unavailable]"
	}
	redactValue(value)
	data, err := json.Marshal(value)
	if err != nil {
		return "[unavailable]"
	}
	return string(data)
}
func redactValue(value any) {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			if _, ok := sensitiveKeys[strings.ToLower(key)]; ok {
				current[key] = "***"
				continue
			}
			redactValue(child)
		}
	case []any:
		for _, child := range current {
			redactValue(child)
		}
	}
}
func auditStringPtr(value string) *string { return &value }
