package data

import (
	"context"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/operationlog"
)

type operationLogRepo struct{ BaseRepo }

func NewOperationLogRepo(data *Data, logger log.Logger) biz.OperationLogRepo {
	return &operationLogRepo{BaseRepo: NewBaseRepo(data, logger)}
}
func operationLogProto(row *gen.OperationLog) *pb.OperationLog {
	created := row.CreatedAt.Format(time.DateTime)
	return &pb.OperationLog{Id: uint64(row.ID), TenantId: row.TenantID, OperatorId: row.OperatorID, OperatorName: &row.OperatorName, Module: row.Module, Action: row.Action, ResourceType: &row.ResourceType, ResourceId: &row.ResourceID, Operation: &row.Operation, Method: &row.Method, Path: &row.Path, RequestSummary: &row.RequestSummary, BeforeData: &row.BeforeData, AfterData: &row.AfterData, Ip: &row.IP, UserAgent: &row.UserAgent, TraceId: &row.TraceID, Success: row.Success, DurationMs: &row.DurationMs, ErrorMessage: &row.ErrorMessage, CreatedAt: &created}
}
func (r *operationLogRepo) Append(ctx context.Context, value *pb.OperationLog) error {
	_, err := r.Data.DB(ctx).OperationLog.Create().SetNillableOperatorID(value.OperatorId).SetOperatorName(value.GetOperatorName()).SetModule(value.GetModule()).SetAction(value.GetAction()).SetResourceType(value.GetResourceType()).SetResourceID(value.GetResourceId()).SetOperation(value.GetOperation()).SetMethod(value.GetMethod()).SetPath(value.GetPath()).SetRequestSummary(value.GetRequestSummary()).SetBeforeData(value.GetBeforeData()).SetAfterData(value.GetAfterData()).SetIP(value.GetIp()).SetUserAgent(value.GetUserAgent()).SetTraceID(value.GetTraceId()).SetSuccess(value.GetSuccess()).SetDurationMs(value.GetDurationMs()).SetErrorMessage(value.GetErrorMessage()).Save(ctx)
	return err
}
func (r *operationLogRepo) List(ctx context.Context, req *pb.ListOperationLogsRequest) ([]*pb.OperationLog, int32, error) {
	query := r.Data.DB(ctx).OperationLog.Query()
	scope, err := r.resolveDataScopeUsers(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !scope.all {
		if len(scope.userIDs) == 0 {
			query.Where(operationlog.IDEQ(0))
		} else {
			query.Where(operationlog.OperatorIDIn(scope.userIDs...))
		}
	}
	if req.Module != nil {
		query.Where(operationlog.ModuleEQ(*req.Module))
	}
	if req.Action != nil {
		query.Where(operationlog.ActionEQ(*req.Action))
	}
	if req.OperatorId != nil {
		query.Where(operationlog.OperatorIDEQ(*req.OperatorId))
	}
	if req.Success != nil {
		query.Where(operationlog.SuccessEQ(*req.Success))
	}
	if req.ResourceType != nil {
		query.Where(operationlog.ResourceTypeEQ(*req.ResourceType))
	}
	if req.ResourceId != nil {
		query.Where(operationlog.ResourceIDEQ(*req.ResourceId))
	}
	if req.TraceId != nil {
		query.Where(operationlog.TraceIDEQ(*req.TraceId))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := int(req.GetPageSize())
	if size <= 0 || size > 100 {
		size = 20
	}
	offset, _ := strconv.Atoi(req.GetPageToken())
	rows, err := query.Order(gen.Desc(operationlog.FieldCreatedAt)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.OperationLog, 0, len(rows))
	for _, row := range rows {
		result = append(result, operationLogProto(row))
	}
	return result, int32(total), nil
}
func (r *operationLogRepo) Get(ctx context.Context, id uint64) (*pb.OperationLog, error) {
	query := r.Data.DB(ctx).OperationLog.Query()
	scope, err := r.resolveDataScopeUsers(ctx)
	if err != nil {
		return nil, err
	}
	if !scope.all {
		if len(scope.userIDs) == 0 {
			query.Where(operationlog.IDEQ(0))
		} else {
			query.Where(operationlog.OperatorIDIn(scope.userIDs...))
		}
	}
	row, err := query.Where(operationlog.IDEQ(uint32(id))).Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("OPERATION_LOG_NOT_FOUND", "操作日志不存在")
	}
	if err != nil {
		return nil, err
	}
	return operationLogProto(row), nil
}
