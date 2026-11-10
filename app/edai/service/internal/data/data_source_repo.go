package data

import (
	"context"
	"time"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/biz"
	"backend-service/app/edai/service/internal/data/ent/gen"
	"backend-service/app/edai/service/internal/data/ent/gen/datasource"
	"backend-service/pkg/utils/convert"

	"entgo.io/ent/dialect/sql"
	"go.einride.tech/aip/ordering"
)

type dataSourceRepo struct {
	data *Data
}

// NewDataSourceRepo 创建数据源仓库。
func NewDataSourceRepo(data *Data) *dataSourceRepo { return &dataSourceRepo{data: data} }

// List 返回数据源列表。
//
// 过滤条件按白名单字段解析，不把 AIP 表达式直接拼进 SQL。
func (r *dataSourceRepo) List(ctx context.Context, opts ...biz.DataSourceListOption) ([]*pb.DataSource, error) {
	q := resolveDataSourceQuery(opts...)
	ctx = r.data.TenantContext(ctx)
	query := r.data.Ent().DataSource.Query()
	if err := r.applyDataSourceFilter(query, q); err != nil {
		return nil, err
	}
	rows, err := query.
		Order(applyDataSourceOrder(q.OrderBy)).
		Offset(q.Offset).
		Limit(q.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.DataSource, 0, len(rows))
	for _, row := range rows {
		out = append(out, entDataSourceToProto(row))
	}
	return out, nil
}

// Count 返回满足条件的数据源数量。
func (r *dataSourceRepo) Count(ctx context.Context, opts ...biz.DataSourceListOption) (int32, error) {
	q := resolveDataSourceQuery(opts...)
	ctx = r.data.TenantContext(ctx)
	query := r.data.Ent().DataSource.Query()
	if err := r.applyDataSourceFilter(query, q); err != nil {
		return 0, err
	}
	n, err := query.Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(n), nil
}

func (r *dataSourceRepo) Get(ctx context.Context, id uint32) (*pb.DataSource, error) {
	ctx = r.data.TenantContext(ctx)
	row, err := r.data.Ent().DataSource.Get(ctx, id)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return entDataSourceToProto(row), nil
}

func (r *dataSourceRepo) Create(ctx context.Context, src *pb.DataSource) (*pb.DataSource, error) {
	ctx = r.data.TenantContext(ctx)
	b := r.data.Ent().DataSource.Create().
		SetName(src.GetName()).
		SetType(nonEmpty(src.GetType(), "local")).
		SetRootPath(src.GetRootPath()).
		SetIncludePatterns(src.GetIncludePatterns()).
		SetExcludePatterns(src.GetExcludePatterns()).
		SetConnectionStatus("unknown")
	if v := src.GetRemark(); v != "" {
		b.SetRemark(v)
	}
	row, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entDataSourceToProto(row), nil
}

func (r *dataSourceRepo) Update(ctx context.Context, id uint32, src *pb.DataSource) (*pb.DataSource, error) {
	ctx = r.data.TenantContext(ctx)
	b := r.data.Ent().DataSource.UpdateOneID(id).
		SetName(src.GetName()).
		SetType(nonEmpty(src.GetType(), "local")).
		SetRootPath(src.GetRootPath()).
		SetIncludePatterns(src.GetIncludePatterns()).
		SetExcludePatterns(src.GetExcludePatterns())
	if v := src.GetRemark(); v != "" {
		b.SetRemark(v)
	} else {
		b.ClearRemark()
	}
	row, err := b.Save(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return entDataSourceToProto(row), nil
}

func (r *dataSourceRepo) Delete(ctx context.Context, id uint32) error {
	ctx = r.data.TenantContext(ctx)
	return r.data.Ent().DataSource.DeleteOneID(id).Exec(ctx)
}

func (r *dataSourceRepo) MarkChecked(ctx context.Context, id uint32, status, message string, checkedAt time.Time) error {
	ctx = r.data.TenantContext(ctx)
	return r.data.Ent().DataSource.UpdateOneID(id).
		SetConnectionStatus(status).
		SetConnectionMessage(message).
		SetLastCheckedAt(checkedAt).
		Exec(ctx)
}

func resolveDataSourceQuery(opts ...biz.DataSourceListOption) biz.DataSourceListQuery {
	q := biz.DataSourceListQuery{Limit: 20}
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

// applyDataSourceFilter 只识别白名单字段，防止任意 SQL 片段注入。
// dataSourceFilterFields 数据源列表可过滤字段与类型。
// 与 service 层 sourceFilterDeclarations 一一对应。
var dataSourceFilterFields = fieldSet{
	datasource.FieldName:             kindString,
	datasource.FieldType:             kindString,
	datasource.FieldRootPath:         kindString,
	datasource.FieldConnectionStatus: kindString,
	datasource.FieldCreatedAt:        kindTime,
	datasource.FieldUpdatedAt:        kindTime,
}

// applyDataSourceFilter 应用 AIP-160 过滤条件；翻译失败返回错误而不是静默忽略。
func (r *dataSourceRepo) applyDataSourceFilter(q *gen.DataSourceQuery, f biz.DataSourceListQuery) error {
	where, err := buildExprFilter(f.Filter, dataSourceFilterFields)
	if err != nil {
		return err
	}
	if where != nil {
		q.Modify(where)
	}
	return nil
}

// applyDataSourceOrder 把 AIP order_by 映射到白名单字段。
func applyDataSourceOrder(orderBy ordering.OrderBy) func(*sql.Selector) {
	if len(orderBy.Fields) == 0 {
		return datasource.ByID(sql.OrderDesc())
	}
	orderFn := func(field string, desc bool) datasource.OrderOption {
		order := sql.OrderAsc()
		if desc {
			order = sql.OrderDesc()
		}
		switch field {
		case datasource.FieldName:
			return datasource.ByName(order)
		case datasource.FieldUpdatedAt:
			return datasource.ByUpdatedAt(order)
		case datasource.FieldCreatedAt:
			return datasource.ByCreatedAt(order)
		default:
			return datasource.ByID(sql.OrderDesc())
		}
	}
	return func(s *sql.Selector) {
		for _, f := range orderBy.Fields {
			orderFn(f.Path, f.Desc)(s)
		}
	}
}

func entDataSourceToProto(row *gen.DataSource) *pb.DataSource {
	if row == nil {
		return nil
	}
	out := &pb.DataSource{
		Id:                row.ID,
		Name:              row.Name,
		Type:              row.Type,
		RootPath:          row.RootPath,
		IncludePatterns:   row.IncludePatterns,
		ExcludePatterns:   row.ExcludePatterns,
		ConnectionStatus:  row.ConnectionStatus,
		ConnectionMessage: ptrIfNotEmpty(row.ConnectionMessage),
		Remark:            ptrIfNotEmpty(row.Remark),
		CreatedAt:         row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         row.UpdatedAt.Format(time.RFC3339),
	}
	if row.LastCheckedAt != nil {
		out.LastCheckedAt = convert.ToPointer(row.LastCheckedAt.Format(time.RFC3339))
	}
	if row.LastScannedAt != nil {
		out.LastScannedAt = convert.ToPointer(row.LastScannedAt.Format(time.RFC3339))
	}
	return out
}
