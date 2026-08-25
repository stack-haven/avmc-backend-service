package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/enhancementlog"
	"backend-service/pkg/aip/listing"
)

type enhancementLogRepo struct{ BaseRepo }

// NewEnhancementLogRepo 创建增强记录仓库。
func NewEnhancementLogRepo(data *Data, logger log.Logger) biz.EnhancementLogRepo {
	return &enhancementLogRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func enhancementLogProto(row *gen.EnhancementLog) *pb.EnhancementLog {
	status := int32(0)
	if row.Status != nil {
		status = *row.Status
	}
	return &pb.EnhancementLog{
		Id:                  row.ID,
		RequestId:           row.RequestID,
		SessionId:           row.SessionID,
		PolicyId:            row.PolicyID,
		PolicyMode:          row.PolicyMode,
		ContextVersion:      row.ContextVersion,
		RawText:             row.RawText,
		EnhancedText:        row.EnhancedText,
		ChangesJson:         row.ChangesJSON,
		ProcessingTimeMs:    row.ProcessingTimeMs,
		CleaningTimeMs:      row.CleaningTimeMs,
		FillerTimeMs:        row.FillerTimeMs,
		VocabMatchTimeMs:    row.VocabMatchTimeMs,
		AliasTimeMs:         row.AliasTimeMs,
		DeterministicTimeMs: row.DeterministicTimeMs,
		PinyinTimeMs:        row.PinyinTimeMs,
		FuzzyTimeMs:         row.FuzzyTimeMs,
		ContextTimeMs:       row.ContextTimeMs,
		Status:              status,
		ErrorMessage:        row.ErrorMessage,
		CreatedAt:           row.CreatedAt.Format(time.DateTime),
	}
}

func (r *enhancementLogRepo) List(ctx context.Context, req *pb.ListEnhancementLogsRequest) ([]*pb.EnhancementLog, int32, error) {
	query := r.Data.DB(ctx).EnhancementLog.Query()
	if req.GetSessionId() != "" {
		query.Where(enhancementlog.SessionIDEQ(req.GetSessionId()))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(enhancementlog.FieldCreatedAt)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.EnhancementLog, 0, len(rows))
	for _, row := range rows {
		result = append(result, enhancementLogProto(row))
	}
	return result, int32(total), nil
}

func (r *enhancementLogRepo) Get(ctx context.Context, id uint32) (*pb.EnhancementLog, error) {
	row, err := r.Data.DB(ctx).EnhancementLog.Query().Where(enhancementlog.IDEQ(id)).Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("ENHANCEMENT_LOG_NOT_FOUND", "增强记录不存在")
	}
	if err != nil {
		return nil, err
	}
	return enhancementLogProto(row), nil
}

func (r *enhancementLogRepo) Save(ctx context.Context, data *biz.EnhancementLogData) error {
	if data == nil || data.RawText == "" {
		return nil
	}
	_, err := r.Data.DB(ctx).EnhancementLog.Create().
		SetRequestID(data.RequestID).
		SetNillableSessionID(nilIfEmpty(data.SessionID)).
		SetNillablePolicyID(nilIfZero(data.PolicyID)).
		SetNillablePolicyMode(nilIfEmpty(data.PolicyMode)).
		SetNillableContextVersion(nilIfEmpty(data.ContextVersion)).
		SetRawText(data.RawText).
		SetNillableEnhancedText(nilIfEmpty(data.EnhancedText)).
		SetNillableChangesJSON(nilIfEmpty(data.ChangesJSON)).
		SetProcessingTimeMs(data.ProcessingTimeMs).
		SetCleaningTimeMs(data.CleaningTimeMs).
		SetFillerTimeMs(data.FillerTimeMs).
		SetVocabMatchTimeMs(data.VocabMatchTimeMs).
		SetAliasTimeMs(data.AliasTimeMs).
		SetDeterministicTimeMs(data.DeterministicTimeMs).
		SetPinyinTimeMs(data.PinyinTimeMs).
		SetFuzzyTimeMs(data.FuzzyTimeMs).
		SetContextTimeMs(data.ContextTimeMs).
		SetStatus(data.Status).
		SetNillableErrorMessage(nilIfEmpty(data.ErrorMessage)).
		Save(ctx)
	return err
}

func nilIfEmpty(s string) *string {
	if s == "" { return nil }
	return &s
}

func nilIfZero(v uint32) *uint32 {
	if v == 0 { return nil }
	return &v
}