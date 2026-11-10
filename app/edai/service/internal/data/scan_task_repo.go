package data

import (
	"context"
	"time"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/biz"
	"backend-service/app/edai/service/internal/data/ent/gen"
	"backend-service/app/edai/service/internal/data/ent/gen/scantask"

	"entgo.io/ent/dialect/sql"
	"go.einride.tech/aip/ordering"
)

type scanTaskRepo struct {
	data *Data
}

// NewScanTaskRepo 创建扫描任务仓库。
func NewScanTaskRepo(data *Data) *scanTaskRepo { return &scanTaskRepo{data: data} }

func (r *scanTaskRepo) Create(ctx context.Context, task *pb.ScanTask) (*pb.ScanTask, error) {
	ctx = r.data.TenantContext(ctx)
	b := r.data.Ent().ScanTask.Create().
		SetSourceID(task.GetSourceId()).
		SetStatus(nonEmpty(task.GetStatus(), biz.ScanStatusPending)).
		SetRootPath(task.GetRootPath()).
		SetFullScan(task.GetFullScan()).
		SetComputeHash(task.GetComputeHash()).
		SetStartedAt(mustTime(task.GetStartedAt()))
	if sp := task.GetSubPath(); sp != "" {
		b.SetSubPath(sp)
	}
	row, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entScanTaskToProto(row), nil
}

func (r *scanTaskRepo) Get(ctx context.Context, id uint32) (*pb.ScanTask, error) {
	ctx = r.data.TenantContext(ctx)
	row, err := r.data.Ent().ScanTask.Get(ctx, id)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return entScanTaskToProto(row), nil
}

func (r *scanTaskRepo) List(ctx context.Context, opts ...biz.ScanTaskListOption) ([]*pb.ScanTask, error) {
	q := resolveScanQuery(opts...)
	ctx = r.data.TenantContext(ctx)
	query := r.data.Ent().ScanTask.Query()
	if err := r.applyScanFilter(query, q); err != nil {
		return nil, err
	}
	rows, err := query.
		Order(applyScanOrder(q.OrderBy)).
		Offset(q.Offset).
		Limit(q.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.ScanTask, 0, len(rows))
	for _, row := range rows {
		out = append(out, entScanTaskToProto(row))
	}
	return out, nil
}

func (r *scanTaskRepo) Count(ctx context.Context, opts ...biz.ScanTaskListOption) (int32, error) {
	q := resolveScanQuery(opts...)
	ctx = r.data.TenantContext(ctx)
	query := r.data.Ent().ScanTask.Query()
	if err := r.applyScanFilter(query, q); err != nil {
		return 0, err
	}
	n, err := query.Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(n), nil
}

// TransitionStatus 以「当前状态 = from」为条件更新，避免并发迁移错乱。
func (r *scanTaskRepo) TransitionStatus(ctx context.Context, id uint32, from, to string) (bool, error) {
	ctx = r.data.TenantContext(ctx)
	n, err := r.data.Ent().ScanTask.Update().
		Where(
			scantask.IDEQ(id),
			scantask.StatusEQ(from),
		).
		SetStatus(to).
		Save(ctx)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *scanTaskRepo) UpdateProgress(ctx context.Context, id uint32, p biz.ScanProgress) error {
	ctx = r.data.TenantContext(ctx)
	return r.data.Ent().ScanTask.UpdateOneID(id).
		SetDiscovered(p.Discovered).
		SetCreatedCount(p.Created).
		SetUpdatedCount(p.Updated).
		SetMarkedMissing(p.MarkedMissing).
		SetSkipped(p.Skipped).
		SetFailed(p.Failed).
		Exec(ctx)
}

func (r *scanTaskRepo) Finish(ctx context.Context, id uint32, status string, p biz.ScanProgress, errMsg string) error {
	ctx = r.data.TenantContext(ctx)
	b := r.data.Ent().ScanTask.UpdateOneID(id).
		SetStatus(status).
		SetDiscovered(p.Discovered).
		SetCreatedCount(p.Created).
		SetUpdatedCount(p.Updated).
		SetMarkedMissing(p.MarkedMissing).
		SetSkipped(p.Skipped).
		SetFailed(p.Failed).
		SetFinishedAt(time.Now())
	if errMsg != "" {
		b.SetErrorMessage(errMsg)
	}
	return b.Exec(ctx)
}

// RecoverInterrupted 服务启动时把遗留的 running / cancel_requested 标记失败。
func (r *scanTaskRepo) RecoverInterrupted(ctx context.Context) (int32, error) {
	ctx = r.data.TenantContext(ctx)
	n, err := r.data.Ent().ScanTask.Update().
		Where(scantask.StatusIn(biz.ScanStatusRunning, biz.ScanStatusCancelRequested)).
		SetStatus(biz.ScanStatusFailed).
		SetErrorMessage("服务重启导致扫描中断，请重新发起").
		SetFinishedAt(time.Now()).
		Save(ctx)
	return int32(n), err
}

func resolveScanQuery(opts ...biz.ScanTaskListOption) biz.ScanTaskListQuery {
	q := biz.ScanTaskListQuery{Limit: 20}
	for _, opt := range opts {
		opt(&q)
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	return q
}

// scanTaskFilterFields 扫描任务列表可过滤字段与类型。
// 与 service 层 scanTaskFilterDeclarations 一一对应。
var scanTaskFilterFields = fieldSet{
	scantask.FieldStatus:     kindString,
	scantask.FieldSourceID:   kindInt,
	scantask.FieldFullScan:   kindBool,
	scantask.FieldFailed:     kindInt,
	scantask.FieldDiscovered: kindInt,
	scantask.FieldStartedAt:  kindTime,
	scantask.FieldFinishedAt: kindTime,
	scantask.FieldCreatedAt:  kindTime,
}

// applyScanFilter 应用数据源范围与 AIP-160 过滤条件。
func (r *scanTaskRepo) applyScanFilter(q *gen.ScanTaskQuery, f biz.ScanTaskListQuery) error {
	if f.SourceID > 0 {
		q.Where(scantask.SourceIDEQ(f.SourceID))
	}
	where, err := buildExprFilter(f.Filter, scanTaskFilterFields)
	if err != nil {
		return err
	}
	if where != nil {
		q.Modify(where)
	}
	return nil
}

// applyScanOrder 把 AIP order_by 映射到白名单字段。
func applyScanOrder(orderBy ordering.OrderBy) func(*sql.Selector) {
	if len(orderBy.Fields) == 0 {
		return scantask.ByID(sql.OrderDesc())
	}
	orderFn := func(field string, desc bool) scantask.OrderOption {
		order := sql.OrderAsc()
		if desc {
			order = sql.OrderDesc()
		}
		switch field {
		case scantask.FieldStatus:
			return scantask.ByStatus(order)
		case scantask.FieldCreatedAt:
			return scantask.ByCreatedAt(order)
		default:
			return scantask.ByID(sql.OrderDesc())
		}
	}
	return func(s *sql.Selector) {
		for _, f := range orderBy.Fields {
			orderFn(f.Path, f.Desc)(s)
		}
	}
}

func entScanTaskToProto(row *gen.ScanTask) *pb.ScanTask {
	if row == nil {
		return nil
	}
	out := &pb.ScanTask{
		Id:            row.ID,
		SourceId:      row.SourceID,
		Status:        row.Status,
		RootPath:      row.RootPath,
		SubPath:       ptrIfNotEmpty(row.SubPath),
		Discovered:    row.Discovered,
		Created:       row.CreatedCount,
		Updated:       row.UpdatedCount,
		MarkedMissing: row.MarkedMissing,
		Skipped:       row.Skipped,
		Failed:        row.Failed,
		FullScan:      row.FullScan,
		ComputeHash:   row.ComputeHash,
		ErrorMessage:  ptrIfNotEmpty(row.ErrorMessage),
		StartedAt:     row.StartedAt.Format(time.RFC3339),
		CreatedAt:     row.CreatedAt.Format(time.RFC3339),
	}
	if row.FinishedAt != nil {
		finished := row.FinishedAt.Format(time.RFC3339)
		out.FinishedAt = &finished
	}
	return out
}
