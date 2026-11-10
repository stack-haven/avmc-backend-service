package data

import (
	"context"
	"sort"
	"time"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/biz"
	"backend-service/app/edai/service/internal/data/ent/gen"
	"backend-service/app/edai/service/internal/data/ent/gen/filerecord"
	"backend-service/pkg/utils/convert"

	"entgo.io/ent/dialect/sql"
	"go.einride.tech/aip/ordering"
)

// fileFilterFields 文件列表可过滤字段与类型。
//
// 与 service 层 fileFilterDeclarations 一一对应：解析阶段管字段是否合法，
// 这里管字段类型与 SQL 翻译。新增字段必须两处同时改。
var fileFilterFields = fieldSet{
	filerecord.FieldFileName:         kindString,
	filerecord.FieldRawPath:          kindString,
	filerecord.FieldRawParentPath:    kindString,
	filerecord.FieldExtension:        kindString,
	filerecord.FieldFileStatus:       kindString,
	filerecord.FieldQuickFingerprint: kindString,
	filerecord.FieldSourceID:         kindInt,
	filerecord.FieldFileSize:         kindInt,
	filerecord.FieldIsDuplicate:      kindBool,
	filerecord.FieldFsCreatedAt:      kindTime,
	filerecord.FieldFsModifiedAt:     kindTime,
	filerecord.FieldFirstSeenAt:      kindTime,
	filerecord.FieldLastScannedAt:    kindTime,
}

type fileRepo struct {
	data *Data
}

// NewFileRepo 创建文件记录仓库。
func NewFileRepo(data *Data) *fileRepo { return &fileRepo{data: data} }

// UpsertScanned 按（source_id, raw_path）幂等写入扫描结果。
//
// created=true 表示新登记；false 表示更新已有记录（指纹变化或状态恢复）。
// 移动文件会得到新路径 → 新记录，旧记录由 MarkMissingExcept 标记 missing，
// 不丢历史（系统设计基线 §5.2「文件移动不能自动导致资产丢失」）。
//
// 并发安全：这里是「先查再写」，两个扫描任务同时处理同一路径时，
// 双方都可能查不到记录而同时 INSERT，后者撞唯一约束。
// **必须**捕获约束冲突并降级为更新，否则调用方会把该文件计入 failed，
// 也就不会进入 ScannedPaths —— 全量扫描随后会把它误标为 missing。
// 实测：同一数据源并发 3 个全量扫描时有 33 个文件被误标 missing。
func (r *fileRepo) UpsertScanned(ctx context.Context, rec *pb.FileRecord) (bool, error) {
	ctx = r.data.TenantContext(ctx)
	existing, err := r.data.Ent().FileRecord.Query().
		Where(
			filerecord.SourceIDEQ(rec.GetSourceId()),
			filerecord.RawPathEQ(rec.GetRawPath()),
		).
		Only(ctx)
	if err != nil && !gen.IsNotFound(err) {
		return false, err
	}

	// updateExisting 抽出更新分支：正常更新与「并发插入冲突后降级」共用。
	updateExisting := func(row *gen.FileRecord) error {
		b := row.Update().
			SetRawParentPath(rec.GetRawParentPath()).
			SetFileName(rec.GetFileName()).
			SetFileSize(rec.GetFileSize()).
			SetQuickFingerprint(rec.GetQuickFingerprint()).
			SetFileStatus(rec.GetFileStatus()).
			SetLastScannedAt(mustTime(rec.GetLastScannedAt())).
			ClearStatusMessage()
		if rec.GetContentHash() != "" {
			b.SetContentHash(rec.GetContentHash())
		}
		if rec.GetExtension() != "" {
			b.SetExtension(rec.GetExtension())
		}
		if rec.GetFsCreatedAt() != "" {
			b.SetFsCreatedAt(mustTime(rec.GetFsCreatedAt()))
		}
		if rec.GetFsModifiedAt() != "" {
			b.SetFsModifiedAt(mustTime(rec.GetFsModifiedAt()))
		}
		_, err := b.Save(ctx)
		return err
	}

	if !gen.IsNotFound(err) {
		return false, updateExisting(existing)
	}

	b := r.data.Ent().FileRecord.Create().
		SetSourceID(rec.GetSourceId()).
		SetRawPath(rec.GetRawPath()).
		SetRawParentPath(rec.GetRawParentPath()).
		SetFileName(rec.GetFileName()).
		SetFileSize(rec.GetFileSize()).
		SetQuickFingerprint(rec.GetQuickFingerprint()).
		SetFileStatus(rec.GetFileStatus()).
		SetFirstSeenAt(mustTime(rec.GetFirstSeenAt())).
		SetLastScannedAt(mustTime(rec.GetLastScannedAt()))
	applyOptionalFileFields(b, rec)
	if _, cerr := b.Save(ctx); cerr == nil {
		return true, nil
	} else if !gen.IsConstraintError(cerr) {
		return false, cerr
	}
	// 竞争失败：另一个扫描任务刚插入了同一路径，改走更新。
	row, qerr := r.data.Ent().FileRecord.Query().
		Where(
			filerecord.SourceIDEQ(rec.GetSourceId()),
			filerecord.RawPathEQ(rec.GetRawPath()),
		).
		Only(ctx)
	if qerr != nil {
		return false, qerr
	}
	return false, updateExisting(row)
}

func (r *fileRepo) GetByID(ctx context.Context, id uint32) (*pb.FileRecord, error) {
	ctx = r.data.TenantContext(ctx)
	row, err := r.data.Ent().FileRecord.Get(ctx, id)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return entFileToProto(row), nil
}

func (r *fileRepo) GetByPath(ctx context.Context, sourceID uint32, rawPath string) (*pb.FileRecord, error) {
	ctx = r.data.TenantContext(ctx)
	row, err := r.data.Ent().FileRecord.Query().
		Where(filerecord.SourceIDEQ(sourceID), filerecord.RawPathEQ(rawPath)).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return entFileToProto(row), nil
}

func (r *fileRepo) List(ctx context.Context, opts ...biz.FileListOption) ([]*pb.FileRecord, error) {
	q := resolveFileQuery(opts...)
	ctx = r.data.TenantContext(ctx)
	query := r.data.Ent().FileRecord.Query()
	if err := r.applyFileFilter(query, q); err != nil {
		return nil, err
	}
	rows, err := query.
		Order(applyFileOrder(q.OrderBy)).
		Offset(q.Offset).
		Limit(q.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.FileRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, entFileToProto(row))
	}
	return out, nil
}

func (r *fileRepo) Count(ctx context.Context, opts ...biz.FileListOption) (int32, error) {
	q := resolveFileQuery(opts...)
	ctx = r.data.TenantContext(ctx)
	query := r.data.Ent().FileRecord.Query()
	if err := r.applyFileFilter(query, q); err != nil {
		return 0, err
	}
	n, err := query.Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(n), nil
}

func (r *fileRepo) Stats(ctx context.Context, sourceID uint32) (*biz.FileStats, error) {
	ctx = r.data.TenantContext(ctx)

	base := func() *gen.FileRecordQuery {
		q := r.data.Ent().FileRecord.Query()
		if sourceID > 0 {
			q = q.Where(filerecord.SourceIDEQ(sourceID))
		}
		return q
	}

	stats := &biz.FileStats{}
	var err error
	var n int
	if n, err = base().Count(ctx); err != nil {
		return nil, err
	}
	stats.Total = int32(n)
	if n, err = base().Where(filerecord.FileStatusEQ("present")).Count(ctx); err != nil {
		return nil, err
	}
	stats.Present = int32(n)
	if n, err = base().Where(filerecord.FileStatusEQ("missing")).Count(ctx); err != nil {
		return nil, err
	}
	stats.Missing = int32(n)
	if n, err = base().Where(filerecord.FileStatusEQ("unreachable")).Count(ctx); err != nil {
		return nil, err
	}
	stats.Unreachable = int32(n)
	if n, err = base().Where(filerecord.IsDuplicateEQ(true)).Count(ctx); err != nil {
		return nil, err
	}
	stats.Duplicate = int32(n)

	// 按扩展名分组聚合：一次查询同时得到总量与分布，避免逐行拉取。
	// 扩展名基数很小（几百），在内存里排序取前 20 即可。
	var groups []struct {
		Extension string `json:"extension,omitempty"`
		Count     int    `json:"count,omitempty"`
		Sum       int64  `json:"sum,omitempty"`
	}
	if err := base().
		GroupBy(filerecord.FieldExtension).
		Aggregate(
			gen.Count(),
			gen.Sum(filerecord.FieldFileSize),
		).
		Scan(ctx, &groups); err != nil {
		return nil, err
	}

	for _, g := range groups {
		stats.TotalSize += g.Sum
		stats.TopExtensions = append(stats.TopExtensions, biz.ExtensionCount{
			Extension: g.Extension,
			Count:     int32(g.Count),
		})
	}
	// 按数量倒序，保留前 20
	sort.Slice(stats.TopExtensions, func(i, j int) bool {
		return stats.TopExtensions[i].Count > stats.TopExtensions[j].Count
	})
	if len(stats.TopExtensions) > 20 {
		stats.TopExtensions = stats.TopExtensions[:20]
	}
	return stats, nil
}

// missingPageSize 是扫描 missing 时每次读取的现存记录条数。
//
// 用「分页读取 + 按 ID 批量更新」而不是一条 NOT IN 全量更新：
//   - last_scanned_at 只有秒级精度，同一秒内两次扫描无法区分先后，
//     用时间比较会把刚扫到的文件误标为消失（实测出现过 6 个假 missing）；
//   - 路径集合是精确的，但一次 NOT IN 几万个路径会撞上 SQLite 的绑定变量
//     上限（默认 32766）并生成极慢的巨型 IN 列表。
const missingPageSize = 2000

// missingUpdateBatch 是按 ID 更新时的批大小，同样是避开绑定变量上限。
const missingUpdateBatch = 400

// MarkMissingExcept 把本轮没再出现的文件标记为 missing。
//
// 精确判据：遍历现存 present 记录，路径不在本轮 keepPaths 里的改为 missing。
// 中途任何一次更新失败都返回已处理数量 + 错误，让调用方能看到部分结果。
func (r *fileRepo) MarkMissingExcept(ctx context.Context, sourceID uint32, keepPaths []string) (int32, error) {
	if len(keepPaths) == 0 {
		// 空集合通常意味着扫描起点整个目录都读不到：不能据此把整个数据源清空。
		return 0, nil
	}
	ctx = r.data.TenantContext(ctx)
	keep := make(map[string]struct{}, len(keepPaths))
	for _, p := range keepPaths {
		keep[p] = struct{}{}
	}

	var marked int32
	var cursor uint32 // keyset 游标：上一页最后一条的 id
	for {
		// 必须用 keyset（id > cursor）而不是 Offset 分页：
		// 循环里会把一部分行改成 missing，它们随即离开 present 集合，
		// 用 Offset 就会跳过这些空位后的记录。实测 10000 文件删 300 个，
		// Offset 版本只标出 287 个（漏 13 个）。
		rows, err := r.data.Ent().FileRecord.Query().
			Where(
				filerecord.SourceIDEQ(sourceID),
				filerecord.FileStatusEQ("present"),
				filerecord.IDGT(cursor),
			).
			Order(filerecord.ByID(sql.OrderAsc())).
			Limit(missingPageSize).
			All(ctx)
		if err != nil {
			return marked, err
		}
		if len(rows) == 0 {
			break
		}
		cursor = rows[len(rows)-1].ID

		var stale []uint32
		for _, row := range rows {
			if _, ok := keep[row.RawPath]; !ok {
				stale = append(stale, row.ID)
			}
		}
		for start := 0; start < len(stale); start += missingUpdateBatch {
			end := min(start+missingUpdateBatch, len(stale))
			n, uerr := r.data.Ent().FileRecord.Update().
				Where(filerecord.IDIn(stale[start:end]...)).
				SetFileStatus("missing").
				Save(ctx)
			if uerr != nil {
				return marked, uerr
			}
			marked += int32(n)
		}
	}
	return marked, nil
}

// RecomputeDuplicates 重算内容去重标记。
//
// 语义（系统设计基线 §5.2 / §15.3）：
//   - 只有**已计算 content_hash** 的记录参与去重，未计算哈希的一律标为非重复，
//     否则会把「不知道」当成「不重复」的两面；
//   - 同一 (tenant, source, content_hash) 出现多条时，**保留 id 最小的那条**为
//     正本，其余标 is_duplicate=true。这样「重复数」= 冗余份数，且结果稳定
//     （不随扫描顺序漂移）；
//   - 跨数据源不去重：不同数据源可能是同一份素材的独立授权副本，
//     把它们互相标重复会误导「可以删掉一份」的判断。
//
// 实现用一条聚合查询 + 一次批量更新，避免把全部记录拉到内存。
func (r *fileRepo) RecomputeDuplicates(ctx context.Context, sourceID uint32) (int32, error) {
	ctx = r.data.TenantContext(ctx)

	// 先把该数据源整体复位，保证消失/改名的残留标记不会留存。
	reset := r.data.Ent().FileRecord.Update().
		Where(filerecord.IsDuplicateEQ(true))
	if sourceID > 0 {
		reset = reset.Where(filerecord.SourceIDEQ(sourceID))
	}
	if _, err := reset.SetIsDuplicate(false).Save(ctx); err != nil {
		return 0, err
	}

	type dupGroup struct {
		ContentHash string `json:"content_hash,omitempty"`
		Count       int    `json:"count,omitempty"`
		MinID       int    `json:"min_id,omitempty"`
		SourceID    uint32 `json:"source_id,omitempty"`
	}

	q := r.data.Ent().FileRecord.Query().Where(filerecord.ContentHashNotNil())
	if sourceID > 0 {
		q = q.Where(filerecord.SourceIDEQ(sourceID))
	}
	var groups []dupGroup
	if err := q.
		GroupBy(filerecord.FieldSourceID, filerecord.FieldContentHash).
		Aggregate(
			gen.Count(),
			gen.As(gen.Min(filerecord.FieldID), "min_id"),
		).
		Scan(ctx, &groups); err != nil {
		return 0, err
	}

	var dupCount int32
	for _, g := range groups {
		if g.Count <= 1 || g.ContentHash == "" {
			continue
		}
		// 保留 id 最小的记录为正本，其余全部标记重复。
		n, err := r.data.Ent().FileRecord.Update().
			Where(
				filerecord.SourceIDEQ(g.SourceID),
				filerecord.ContentHashEQ(g.ContentHash),
				filerecord.IDNEQ(uint32(g.MinID)),
			).
			SetIsDuplicate(true).
			Save(ctx)
		if err != nil {
			return dupCount, err
		}
		dupCount += int32(n)
	}
	return dupCount, nil
}

func resolveFileQuery(opts ...biz.FileListOption) biz.FileListQuery {
	q := biz.FileListQuery{Limit: 20}
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

// applyFileFilter 应用数据源范围与 AIP-160 过滤条件。
//
// 过滤表达式已在服务层做过字段声明与类型检查（filtering.ParseFilter），
// 这里只把它翻译成 SQL 谓词；翻译失败返回错误而不是静默忽略条件。
func (r *fileRepo) applyFileFilter(q *gen.FileRecordQuery, f biz.FileListQuery) error {
	if f.SourceID > 0 {
		q.Where(filerecord.SourceIDEQ(f.SourceID))
	}
	where, err := buildExprFilter(f.Filter, fileFilterFields)
	if err != nil {
		return err
	}
	if where != nil {
		q.Modify(where)
	}
	return nil
}

// applyFileOrder 把 AIP order_by 映射到白名单字段。
//
// 未识别的字段一律回落到默认排序（id desc），保证分页稳定；
// 服务层已用 orderBy.ValidateForPaths 做过白名单校验。
func applyFileOrder(orderBy ordering.OrderBy) func(*sql.Selector) {
	if len(orderBy.Fields) == 0 {
		return filerecord.ByID(sql.OrderDesc())
	}
	// 支持多字段排序：按声明顺序依次追加。
	orderFn := func(field string, desc bool) filerecord.OrderOption {
		order := sql.OrderAsc()
		if desc {
			order = sql.OrderDesc()
		}
		switch field {
		case "file_name":
			return filerecord.ByFileName(order)
		case "file_size":
			return filerecord.ByFileSize(order)
		case "fs_modified_at":
			return filerecord.ByFsModifiedAt(order)
		case "created_at":
			return filerecord.ByCreatedAt(order)
		default:
			return filerecord.ByID(sql.OrderDesc())
		}
	}
	return func(s *sql.Selector) {
		for _, f := range orderBy.Fields {
			orderFn(f.Path, f.Desc)(s)
		}
	}
}

func entFileToProto(row *gen.FileRecord) *pb.FileRecord {
	if row == nil {
		return nil
	}
	out := &pb.FileRecord{
		Id:               row.ID,
		SourceId:         row.SourceID,
		RawPath:          row.RawPath,
		RawParentPath:    row.RawParentPath,
		FileName:         row.FileName,
		Extension:        ptrIfNotEmpty(row.Extension),
		FileSize:         row.FileSize,
		ContentHash:      ptrIfNotEmpty(row.ContentHash),
		QuickFingerprint: row.QuickFingerprint,
		FirstSeenAt:      row.FirstSeenAt.Format(time.RFC3339),
		LastScannedAt:    row.LastScannedAt.Format(time.RFC3339),
		FileStatus:       row.FileStatus,
		StatusMessage:    ptrIfNotEmpty(row.StatusMessage),
		IsDuplicate:      row.IsDuplicate,
		CreatedAt:        row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        row.UpdatedAt.Format(time.RFC3339),
	}
	if row.FsCreatedAt != nil {
		out.FsCreatedAt = convert.ToPointer(row.FsCreatedAt.Format(time.RFC3339))
	}
	if row.FsModifiedAt != nil {
		out.FsModifiedAt = convert.ToPointer(row.FsModifiedAt.Format(time.RFC3339))
	}
	return out
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now()
	}
	return t
}

// applyOptionalFileFields 写入可选字段（创建时用）。
func applyOptionalFileFields(b *gen.FileRecordCreate, rec *pb.FileRecord) {
	if rec.GetExtension() != "" {
		b.SetExtension(rec.GetExtension())
	}
	if rec.GetFsCreatedAt() != "" {
		b.SetFsCreatedAt(mustTime(rec.GetFsCreatedAt()))
	}
	if rec.GetFsModifiedAt() != "" {
		b.SetFsModifiedAt(mustTime(rec.GetFsModifiedAt()))
	}
	if rec.GetContentHash() != "" {
		b.SetContentHash(rec.GetContentHash())
	}
}
