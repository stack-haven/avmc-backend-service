package data

import (
	"context"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/fileaccesslog"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"
)

var _ biz.FileAccessLogRepo = (*fileAccessLogRepo)(nil)

type fileAccessLogRepo struct {
	BaseRepo
}

func NewFileAccessLogRepo(data *Data, logger log.Logger) biz.FileAccessLogRepo {
	return &fileAccessLogRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func fileAccessLogToProto(row *gen.FileAccessLog) *pbCore.FileAccessLog {
	if row == nil {
		return nil
	}
	return &pbCore.FileAccessLog{
		Id:           row.ID,
		TenantId:     &row.TenantID,
		FileId:       row.FileID,
		FileName:     &row.FileName,
		Action:       &row.Action,
		OperatorId:   row.OperatorID,
		OperatorName: &row.OperatorName,
		ClientIp:     &row.ClientIP,
		UserAgent:    &row.UserAgent,
		Result:       &row.Result,
		Message:      &row.Message,
		CreatedAt:    convert.TimeValueToString(&row.CreatedAt, time.DateTime),
	}
}

func (r *fileAccessLogRepo) Append(ctx context.Context, value *pbCore.FileAccessLog) error {
	if value == nil || value.GetFileId() == 0 {
		return pb.ErrorBadRequest("文件访问日志缺少文件ID")
	}
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	builder := r.Data.DB(ctx).FileAccessLog.Create().
		SetFileID(value.GetFileId()).
		SetFileName(value.GetFileName()).
		SetAction(value.GetAction()).
		SetOperatorName(value.GetOperatorName()).
		SetClientIP(value.GetClientIp()).
		SetUserAgent(value.GetUserAgent()).
		SetResult(value.GetResult()).
		SetMessage(value.GetMessage())
	operatorID := value.GetOperatorId()
	if operatorID == 0 {
		operatorID = authn.GetAuthUserID(ctx)
	}
	if operatorID > 0 {
		builder.SetOperatorID(operatorID)
	}
	_, err := builder.Save(ctx)
	return err
}

func (r *fileAccessLogRepo) List(ctx context.Context, opts ...listing.Option) ([]*pbCore.FileAccessLog, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	options := applyFileListOptions(opts...)
	rows, err := r.Data.DB(ctx).FileAccessLog.Query().
		Where(ents.ApplyFilter(options.Filter)).
		Order(ents.ApplyOrderBy(options.OrderBy)).
		Offset(options.Offset).Limit(options.Limit).
		Order(gen.Desc(fileaccesslog.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(rows, fileAccessLogToProto), nil
}

func (r *fileAccessLogRepo) Count(ctx context.Context, opts ...listing.Option) (int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	options := applyFileListOptions(opts...)
	count, err := r.Data.DB(ctx).FileAccessLog.Query().
		Where(ents.ApplyFilter(options.Filter)).
		Count(ctx)
	return int32(count), err
}
