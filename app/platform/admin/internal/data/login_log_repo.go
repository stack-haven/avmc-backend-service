package data

import (
	"context"
	"time"

	pb "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/loginlog"
	"backend-service/pkg/aip/listing"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type loginLogRepo struct{ BaseRepo }

func NewLoginLogRepo(data *Data, logger log.Logger) biz.LoginLogRepo {
	return &loginLogRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func loginLogProto(row *gen.LoginLog) *pb.LoginLog {
	createdAt := row.CreatedAt.Format(time.DateTime)
	return &pb.LoginLog{
		Id:            uint64(row.ID),
		TenantId:      row.TenantID,
		UserId:        row.UserID,
		Identity:      row.Identity,
		LoginType:     row.LoginType,
		Result:        row.Result,
		FailureReason: &row.FailureReason,
		Ip:            &row.IP,
		UserAgent:     &row.UserAgent,
		TraceId:       &row.TraceID,
		SessionId:     &row.SessionID,
		CreatedAt:     &createdAt,
	}
}

func (r *loginLogRepo) Append(ctx context.Context, value *pb.LoginLog) error {
	_, err := r.Data.DB(ctx).LoginLog.Create().
		SetNillableUserID(value.UserId).
		SetIdentity(value.GetIdentity()).
		SetLoginType(value.GetLoginType()).
		SetResult(value.GetResult()).
		SetFailureReason(value.GetFailureReason()).
		SetIP(value.GetIp()).
		SetUserAgent(value.GetUserAgent()).
		SetTraceID(value.GetTraceId()).
		SetSessionID(value.GetSessionId()).
		Save(ctx)
	return err
}

func (r *loginLogRepo) List(ctx context.Context, req *pb.ListLoginLogsRequest) ([]*pb.LoginLog, int32, error) {
	query := r.Data.DB(ctx).LoginLog.Query()
	scope, err := r.resolveDataScopeUsers(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !scope.all {
		if len(scope.userIDs) == 0 {
			query.Where(loginlog.IDEQ(0))
		} else {
			query.Where(loginlog.UserIDIn(scope.userIDs...))
		}
	}
	if req.UserId != nil {
		query.Where(loginlog.UserIDEQ(*req.UserId))
	}
	if req.Identity != nil {
		query.Where(loginlog.IdentityContains(*req.Identity))
	}
	if req.LoginType != nil {
		query.Where(loginlog.LoginTypeEQ(*req.LoginType))
	}
	if req.Result != nil {
		query.Where(loginlog.ResultEQ(*req.Result))
	}
	if req.Ip != nil {
		query.Where(loginlog.IPEQ(*req.Ip))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(loginlog.FieldCreatedAt)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.LoginLog, 0, len(rows))
	for _, row := range rows {
		result = append(result, loginLogProto(row))
	}
	return result, int32(total), nil
}

func (r *loginLogRepo) Get(ctx context.Context, id uint64) (*pb.LoginLog, error) {
	query := r.Data.DB(ctx).LoginLog.Query()
	scope, err := r.resolveDataScopeUsers(ctx)
	if err != nil {
		return nil, err
	}
	if !scope.all {
		if len(scope.userIDs) == 0 {
			query.Where(loginlog.IDEQ(0))
		} else {
			query.Where(loginlog.UserIDIn(scope.userIDs...))
		}
	}
	row, err := query.Where(loginlog.IDEQ(uint32(id))).Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("LOGIN_LOG_NOT_FOUND", "登录日志不存在")
	}
	if err != nil {
		return nil, err
	}
	return loginLogProto(row), nil
}
