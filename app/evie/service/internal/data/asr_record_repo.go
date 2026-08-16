package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/asrrecord"
	"backend-service/pkg/aip/listing"
)

type asrRecordRepo struct{ BaseRepo }

// NewASRRecordRepo 创建 ASR 识别记录仓库。
func NewASRRecordRepo(data *Data, logger log.Logger) biz.ASRRecordRepo {
	return &asrRecordRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// asrRecordProto converts an Ent AsrRecord to a proto AsrRecord.
func asrRecordProto(row *gen.AsrRecord) *pb.AsrRecord {
	return &pb.AsrRecord{
		Id:              row.ID,
		UserId:          row.UserID,
		SessionId:       row.SessionID,
		RawText:         row.RawText,
		Confidence:      float32(row.Confidence),
		DurationMs:      row.DurationMs,
		AudioDurationMs: int32(row.AudioDurationMs),
		AudioUrl:        row.AudioURL,
		AudioFormat:     row.AudioFormat,
		Engine:          row.Engine,
		CreatedAt:       row.CreatedAt.Format(time.DateTime),
	}
}

// Save 保存一条 ASR 识别记录（只追加），返回记录 ID。
func (r *asrRecordRepo) Save(ctx context.Context, record *biz.ASRRecord) (uint32, error) {
	create := r.Data.DB(ctx).AsrRecord.Create().
		SetSessionID(record.SessionID).
		SetRawText(record.RawText).
		SetConfidence(record.Confidence).
		SetDurationMs(record.DurationMs).
		SetAudioDurationMs(record.AudioDurationMs).
		SetAudioFormat(record.AudioFormat).
		SetEngine(record.Engine)
	if record.UserID > 0 {
		create.SetUserID(record.UserID)
	}
	if record.AudioURL != "" {
		create.SetAudioURL(record.AudioURL)
	}
	row, err := create.Save(ctx)
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}

// List 分页查询识别记录。
func (r *asrRecordRepo) List(ctx context.Context, req *pb.ListAsrRecordsRequest) ([]*pb.AsrRecord, int32, error) {
	query := r.Data.DB(ctx).AsrRecord.Query()
	if req.GetSessionId() != "" {
		query.Where(asrrecord.SessionIDEQ(req.GetSessionId()))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(asrrecord.FieldID)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.AsrRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, asrRecordProto(row))
	}
	return result, int32(total), nil
}

// Get 查询识别记录详情。
func (r *asrRecordRepo) Get(ctx context.Context, id uint32) (*pb.AsrRecord, error) {
	row, err := r.Data.DB(ctx).AsrRecord.Query().Where(asrrecord.IDEQ(id)).Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("ASR_RECORD_NOT_FOUND", "识别记录不存在")
	}
	if err != nil {
		return nil, err
	}
	return asrRecordProto(row), nil
}
