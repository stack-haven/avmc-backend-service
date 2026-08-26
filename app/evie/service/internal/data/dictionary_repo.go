package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionary"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionarycategory"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionarychangelog"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryentry"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryrelation"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryversion"
	entviewer "backend-service/app/evie/service/internal/data/ent/viewer"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/aip/listing"
)

type dictionaryRepo struct{ BaseRepo }

// NewDictionaryRepo 创建词库中心仓库。
func NewDictionaryRepo(data *Data, logger log.Logger) biz.DictionaryRepo {
	return &dictionaryRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// dictionaryProto converts an Ent Dictionary to a proto Dictionary.
func dictionaryProto(row *gen.Dictionary) *pb.Dictionary {
	status := int32(0)
	if row.Status != nil {
		status = *row.Status
	}
	return &pb.Dictionary{
		Id:          row.ID,
		Name:        row.Name,
		Scope:       row.Scope,
		Source:      row.Source,
		Description: row.Description,
		Status:      status,
		CreatedAt:   row.CreatedAt.Format(time.DateTime),
		UpdatedAt:   row.UpdatedAt.Format(time.DateTime),
	}
}

// dictionaryEntryProto converts an Ent DictionaryEntry to a proto DictionaryEntry.
func dictionaryEntryProto(row *gen.DictionaryEntry) *pb.DictionaryEntry {
	status := int32(0)
	if row.Status != nil {
		status = *row.Status
	}
	return &pb.DictionaryEntry{
		Id:            row.ID,
		DictionaryId:  row.DictionaryID,
		StandardText:  row.StandardText,
		EntryType:     row.EntryType,
		Category:      row.Category,
		Description:   row.Description,
		Source:        row.Source,
		SourceId:      row.SourceID,
		Priority:      row.Priority,
		Pinyin:        row.Pinyin,
		PinyinInitial: row.PinyinInitial,
		NormalizedText: row.NormalizedText,
		Status:        status,
		CreatedAt:     row.CreatedAt.Format(time.DateTime),
		UpdatedAt:     row.UpdatedAt.Format(time.DateTime),
	}
}

// ListDictionaries 分页查询词库。
func (r *dictionaryRepo) ListDictionaries(ctx context.Context, req *pb.ListDictionariesRequest) ([]*pb.Dictionary, int32, error) {
	query := r.Data.DB(ctx).Dictionary.Query().Where(dictionary.DeletedAtIsNil())
	if req.GetScope() != "" {
		query.Where(dictionary.ScopeEQ(req.GetScope()))
	}
	if req.GetKeyword() != "" {
		query.Where(dictionary.NameContains(req.GetKeyword()))
	}
	if req.GetStatus() != 0 {
		query.Where(dictionary.StatusEQ(req.GetStatus()))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(dictionary.FieldID)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.Dictionary, 0, len(rows))
	for _, row := range rows {
		result = append(result, dictionaryProto(row))
	}
	return result, int32(total), nil
}

// GetDictionary 查询词库详情。
func (r *dictionaryRepo) GetDictionary(ctx context.Context, id uint32) (*pb.Dictionary, error) {
	row, err := r.Data.DB(ctx).Dictionary.Query().
		Where(dictionary.IDEQ(id), dictionary.DeletedAtIsNil()).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_NOT_FOUND", "词库不存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryProto(row), nil
}

// CreateDictionary 创建词库。
func (r *dictionaryRepo) CreateDictionary(ctx context.Context, d *pb.Dictionary) (*pb.Dictionary, error) {
	row, err := r.Data.DB(ctx).Dictionary.Create().
		SetName(d.GetName()).
		SetScope(d.GetScope()).
		SetSource(d.GetSource()).
		SetDescription(d.GetDescription()).
		Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_EXISTS", "同租户下词库名称已存在")
	}
	if err != nil {
		return nil, err
	}
	return r.GetDictionary(ctx, row.ID)
}

// UpdateDictionary 更新词库。
func (r *dictionaryRepo) UpdateDictionary(ctx context.Context, d *pb.Dictionary) (*pb.Dictionary, error) {
	update := r.Data.DB(ctx).Dictionary.UpdateOneID(d.GetId())
	if d.GetName() != "" {
		update.SetName(d.GetName())
	}
	if d.GetDescription() != "" {
		update.SetDescription(d.GetDescription())
	}
	if d.GetStatus() != 0 {
		update.SetStatus(d.GetStatus())
	}
	if _, err := update.Save(ctx); gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_NOT_FOUND", "词库不存在")
	} else if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_EXISTS", "同租户下词库名称已存在")
	} else if err != nil {
		return nil, err
	}
	return r.GetDictionary(ctx, d.GetId())
}

// DeleteDictionary 软删除词库。
func (r *dictionaryRepo) DeleteDictionary(ctx context.Context, id uint32) error {
	now := time.Now()
	if err := r.Data.DB(ctx).Dictionary.UpdateOneID(id).SetDeletedAt(now).Exec(ctx); gen.IsNotFound(err) {
		return errors.NotFound("DICTIONARY_NOT_FOUND", "词库不存在")
	} else if err != nil {
		return err
	}
	return nil
}

// ListEntries 分页查询词条。
func (r *dictionaryRepo) ListEntries(ctx context.Context, req *pb.ListEntriesRequest) ([]*pb.DictionaryEntry, int32, error) {
	query := r.Data.DB(ctx).DictionaryEntry.Query().Where(dictionaryentry.DeletedAtIsNil())
	if req.GetDictionaryId() != 0 {
		query.Where(dictionaryentry.DictionaryIDEQ(req.GetDictionaryId()))
	} else {
		// 全部词库：只查未删除词库下的词条（词库已删除则其词条不再展示）
		dictIDs, err := r.activeDictionaryIDs(ctx)
		if err != nil {
			return nil, 0, err
		}
		query.Where(dictionaryentry.DictionaryIDIn(dictIDs...))
	}
	if req.GetCategory() != "" {
		query.Where(dictionaryentry.CategoryEQ(req.GetCategory()))
	}
	if req.GetKeyword() != "" {
		query.Where(dictionaryentry.StandardTextContains(req.GetKeyword()))
	}
	if req.GetStatus() != 0 {
		query.Where(dictionaryentry.StatusEQ(req.GetStatus()))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.
		Order(gen.Desc(dictionaryentry.FieldPriority), gen.Desc(dictionaryentry.FieldID)).
		Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	// JOIN 词库名称（列表展示用）
	dictIDs := make([]uint32, 0, len(rows))
	for _, row := range rows {
		dictIDs = append(dictIDs, row.DictionaryID)
	}
	dictsByID, err := r.loadDictionariesByIDs(ctx, dictIDs)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.DictionaryEntry, 0, len(rows))
	for _, row := range rows {
		item := dictionaryEntryProto(row)
		if d, ok := dictsByID[row.DictionaryID]; ok && d != nil {
			item.DictionaryName = d.Name
		}
		result = append(result, item)
	}
	return result, int32(total), nil
}

// GetEntry 查询词条详情。
func (r *dictionaryRepo) GetEntry(ctx context.Context, id uint32) (*pb.DictionaryEntry, error) {
	row, err := r.Data.DB(ctx).DictionaryEntry.Query().
		Where(dictionaryentry.IDEQ(id), dictionaryentry.DeletedAtIsNil()).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_ENTRY_NOT_FOUND", "词条不存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryEntryProto(row), nil
}

// CreateEntry 创建词条。
func (r *dictionaryRepo) CreateEntry(ctx context.Context, e *pb.DictionaryEntry) (*pb.DictionaryEntry, error) {
	create := r.Data.DB(ctx).DictionaryEntry.Create().
		SetDictionaryID(e.GetDictionaryId()).
		SetStandardText(e.GetStandardText()).
		SetEntryType(e.GetEntryType()).
		SetCategory(e.GetCategory()).
		SetSource(e.GetSource()).
		SetPriority(e.GetPriority())
	if e.GetDescription() != "" {
		create.SetDescription(e.GetDescription())
	}
	if e.GetSourceId() != "" {
		create.SetSourceID(e.GetSourceId())
	}
	if e.GetPinyin() != "" {
		create.SetPinyin(e.GetPinyin())
	}
	if e.GetPinyinInitial() != "" {
		create.SetPinyinInitial(e.GetPinyinInitial())
	}
	if e.GetNormalizedText() != "" {
		create.SetNormalizedText(e.GetNormalizedText())
	}
	row, err := create.Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_ENTRY_EXISTS", "同词库下标准词已存在")
	}
	if err != nil {
		return nil, err
	}
	return r.GetEntry(ctx, row.ID)
}

// UpdateEntry 更新词条。
func (r *dictionaryRepo) UpdateEntry(ctx context.Context, e *pb.DictionaryEntry) (*pb.DictionaryEntry, error) {
	update := r.Data.DB(ctx).DictionaryEntry.UpdateOneID(e.GetId())
	if e.GetStandardText() != "" {
		update.SetStandardText(e.GetStandardText())
	}
	if e.GetEntryType() != "" {
		update.SetEntryType(e.GetEntryType())
	}
	if e.GetCategory() != "" {
		update.SetCategory(e.GetCategory())
	}
	if e.GetDescription() != "" {
		update.SetDescription(e.GetDescription())
	}
	if e.GetPriority() != 0 {
		update.SetPriority(e.GetPriority())
	}
	if e.GetPinyin() != "" {
		update.SetPinyin(e.GetPinyin())
	}
	if e.GetPinyinInitial() != "" {
		update.SetPinyinInitial(e.GetPinyinInitial())
	}
	if e.GetNormalizedText() != "" {
		update.SetNormalizedText(e.GetNormalizedText())
	}
	if e.GetStatus() != 0 {
		update.SetStatus(e.GetStatus())
	}
	if _, err := update.Save(ctx); gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_ENTRY_NOT_FOUND", "词条不存在")
	} else if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_ENTRY_EXISTS", "同词库下标准词已存在")
	} else if err != nil {
		return nil, err
	}
	return r.GetEntry(ctx, e.GetId())
}

// DeleteEntry 软删除词条。
func (r *dictionaryRepo) DeleteEntry(ctx context.Context, id uint32) error {
	now := time.Now()
	if err := r.Data.DB(ctx).DictionaryEntry.UpdateOneID(id).SetDeletedAt(now).Exec(ctx); gen.IsNotFound(err) {
		return errors.NotFound("DICTIONARY_ENTRY_NOT_FOUND", "词条不存在")
	} else if err != nil {
		return err
	}
	return nil
}

// ListActiveEntryTexts 返回全量启用的标准词（供文本增强引擎模糊匹配）。
func (r *dictionaryRepo) ListActiveEntryTexts(ctx context.Context) ([]string, error) {
	rows, err := r.Data.DB(ctx).DictionaryEntry.Query().
		Where(dictionaryentry.DeletedAtIsNil(), dictionaryentry.StatusEQ(1)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.StandardText)
	}
	return result, nil
}

// dictionaryRelationProto converts an Ent DictionaryRelation to a proto DictionaryRelation.
// 包含 JOIN 后的 entry_standard_text / entry_category / dictionary_name /
// related_standard_text / related_tenant_id 字段，用于前端「您 → ALIAS → 客服您好」自然语言化展示。
func dictionaryRelationProto(row *gen.DictionaryRelation, joined *relationJoin) *pb.DictionaryRelation {
	status := int32(0)
	if row.Status != nil {
		status = *row.Status
	}
	out := &pb.DictionaryRelation{
		Id:            row.ID,
		EntryId:       row.EntryID,
		RelationType:  row.RelationType,
		RelatedText:   row.RelatedText,
		TargetEntryId: row.TargetEntryID,
		Source:        row.Source,
		Description:   row.Description,
		Status:        status,
		CreatedAt:     row.CreatedAt.Format(time.DateTime),
		UpdatedAt:     row.UpdatedAt.Format(time.DateTime),
	}
	if joined != nil {
		out.EntryStandardText = joined.EntryStandardText
		out.EntryCategory = joined.EntryCategory
		out.DictionaryName = joined.DictionaryName
		out.RelatedStandardText = joined.RelatedStandardText
		out.RelatedTenantId = joined.RelatedTenantID
	}
	return out
}

// relationJoin 记录查询返回的关联字段（避免 N+1）。
// 与 DictionaryRelation 一一对应，由 listRelationsByEntryAndID/listRelationsByDictionary 填充。
type relationJoin struct {
	EntryStandardText   string
	EntryCategory       string
	DictionaryName      string
	RelatedStandardText string
	RelatedTenantID     uint32
}

// fillRelationJoins 一次性预加载所需 entry / dictionary 字段，避免 N+1。
// entriesByID 与 dictsByID 由调用方提供（已聚合到 IN 查询结果）。
func fillRelationJoins(relations []*gen.DictionaryRelation, entriesByID map[uint32]*gen.DictionaryEntry, dictsByID map[uint32]*gen.Dictionary, targetEntryByID map[uint32]*gen.DictionaryEntry) map[uint32]*relationJoin {
	out := make(map[uint32]*relationJoin, len(relations))
	for _, rel := range relations {
		join := &relationJoin{}
		if entry, ok := entriesByID[rel.EntryID]; ok && entry != nil {
			join.EntryStandardText = entry.StandardText
			join.EntryCategory = entry.Category
			if d, ok := dictsByID[entry.DictionaryID]; ok && d != nil {
				join.DictionaryName = d.Name
			}
		}
		if rel.TargetEntryID != 0 {
			if t, ok := targetEntryByID[rel.TargetEntryID]; ok && t != nil {
				join.RelatedStandardText = t.StandardText
				join.RelatedTenantID = t.TenantID
			}
		}
		out[rel.ID] = join
	}
	return out
}

// loadEntriesByIDs 预加载 entry，返回 ID → entry 映射。
func (r *dictionaryRepo) loadEntriesByIDs(ctx context.Context, ids []uint32) (map[uint32]*gen.DictionaryEntry, error) {
	if len(ids) == 0 {
		return map[uint32]*gen.DictionaryEntry{}, nil
	}
	rows, err := r.Data.DB(ctx).DictionaryEntry.Query().
		Where(dictionaryentry.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uint32]*gen.DictionaryEntry, len(rows))
	for _, e := range rows {
		out[e.ID] = e
	}
	return out, nil
}

// loadDictionariesByIDs 预加载 dictionary，返回 ID → dictionary 映射。
func (r *dictionaryRepo) loadDictionariesByIDs(ctx context.Context, ids []uint32) (map[uint32]*gen.Dictionary, error) {
	if len(ids) == 0 {
		return map[uint32]*gen.Dictionary{}, nil
	}
	rows, err := r.Data.DB(ctx).Dictionary.Query().
		Where(dictionary.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uint32]*gen.Dictionary, len(rows))
	for _, d := range rows {
		out[d.ID] = d
	}
	return out, nil
}

// activeDictionaryIDs 返回未删除词库的 ID 列表（用于 ListEntries 全部查询时过滤已删除词库）。
func (r *dictionaryRepo) activeDictionaryIDs(ctx context.Context) ([]uint32, error) {
	rows, err := r.Data.DB(ctx).Dictionary.Query().
		Where(dictionary.DeletedAtIsNil()).
		Select(dictionary.FieldID).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(rows))
	for _, d := range rows {
		ids = append(ids, d.ID)
	}
	return ids, nil
}

// ListRelations 分页查询词条关系（响应含 JOIN 字段）。
func (r *dictionaryRepo) ListRelations(ctx context.Context, req *pb.ListRelationsRequest) ([]*pb.DictionaryRelation, int32, error) {
	query := r.Data.DB(ctx).DictionaryRelation.Query().Where(dictionaryrelation.DeletedAtIsNil())
	if req.GetEntryId() != 0 {
		query.Where(dictionaryrelation.EntryIDEQ(req.GetEntryId()))
	}
	if req.GetRelationType() != "" {
		query.Where(dictionaryrelation.RelationTypeEQ(req.GetRelationType()))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Asc(dictionaryrelation.FieldID)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	// JOIN：收集 entry IDs + target entry IDs，批量预加载。
	entryIDs := make([]uint32, 0, len(rows))
	targetIDs := make([]uint32, 0, len(rows))
	for _, rel := range rows {
		entryIDs = append(entryIDs, rel.EntryID)
		if rel.TargetEntryID != 0 {
			targetIDs = append(targetIDs, rel.TargetEntryID)
		}
	}
	entriesByID, err := r.loadEntriesByIDs(ctx, entryIDs)
	if err != nil {
		return nil, 0, err
	}
	dictIDs := make([]uint32, 0, len(entriesByID))
	for _, e := range entriesByID {
		dictIDs = append(dictIDs, e.DictionaryID)
	}
	dictsByID, err := r.loadDictionariesByIDs(ctx, dictIDs)
	if err != nil {
		return nil, 0, err
	}
	targetByID, err := r.loadEntriesByIDs(ctx, targetIDs)
	if err != nil {
		return nil, 0, err
	}
	joins := fillRelationJoins(rows, entriesByID, dictsByID, targetByID)

	result := make([]*pb.DictionaryRelation, 0, len(rows))
	for _, row := range rows {
		result = append(result, dictionaryRelationProto(row, joins[row.ID]))
	}
	return result, int32(total), nil
}

// ListRelationsByDictionary 词库级别关系列表（跨 entryId，一次性返回词库下所有词条的关系）。
// SQL 等价：SELECT r.* FROM dictionary_relations r
//   INNER JOIN dictionary_entries e ON r.entry_id = e.id WHERE e.dictionary_id = ?
// 响应含 JOIN 后的 entry_standard_text / dictionary_name 等字段，与 ListRelations 保持一致。
func (r *dictionaryRepo) ListRelationsByDictionary(ctx context.Context, req *pb.ListRelationsByDictionaryRequest) ([]*pb.DictionaryRelation, int32, error) {
	// 1. 校验词库存在 + scope 可见性（复用 GetDictionary，错误码 DICTIONARY_NOT_FOUND）
	if _, err := r.GetDictionary(ctx, req.GetDictionaryId()); err != nil {
		return nil, 0, err
	}

	// 2. 总数
	baseQuery := r.Data.DB(ctx).DictionaryRelation.Query().
		Where(dictionaryrelation.DeletedAtIsNil()).
		Where(dictionaryrelation.HasEntryWith(dictionaryentry.DictionaryIDEQ(req.GetDictionaryId())))
	if req.GetRelationType() != "" {
		baseQuery.Where(dictionaryrelation.RelationTypeEQ(req.GetRelationType()))
	}
	if req.GetKeyword() != "" {
		// 模糊搜索：entry.standard_text 或 relation.related_text
		baseQuery.Where(dictionaryrelation.Or(
			dictionaryrelation.HasEntryWith(dictionaryentry.StandardTextContains(req.GetKeyword())),
			dictionaryrelation.RelatedTextContains(req.GetKeyword()),
		))
	}
	total, err := baseQuery.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 3. 分页 + 排序
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := baseQuery.Order(gen.Asc(dictionaryrelation.FieldID)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 4. JOIN 字段预加载（与 ListRelations 同样的填充方式）
	entryIDs := make([]uint32, 0, len(rows))
	targetIDs := make([]uint32, 0, len(rows))
	for _, rel := range rows {
		entryIDs = append(entryIDs, rel.EntryID)
		if rel.TargetEntryID != 0 {
			targetIDs = append(targetIDs, rel.TargetEntryID)
		}
	}
	entriesByID, err := r.loadEntriesByIDs(ctx, entryIDs)
	if err != nil {
		return nil, 0, err
	}
	dictIDs := make([]uint32, 0, len(entriesByID))
	for _, e := range entriesByID {
		dictIDs = append(dictIDs, e.DictionaryID)
	}
	dictsByID, err := r.loadDictionariesByIDs(ctx, dictIDs)
	if err != nil {
		return nil, 0, err
	}
	targetByID, err := r.loadEntriesByIDs(ctx, targetIDs)
	if err != nil {
		return nil, 0, err
	}
	joins := fillRelationJoins(rows, entriesByID, dictsByID, targetByID)

	result := make([]*pb.DictionaryRelation, 0, len(rows))
	for _, row := range rows {
		result = append(result, dictionaryRelationProto(row, joins[row.ID]))
	}
	return result, int32(total), nil
}

// GetRelation 查询词条关系详情。
func (r *dictionaryRepo) GetRelation(ctx context.Context, id uint32) (*pb.DictionaryRelation, error) {
	row, err := r.Data.DB(ctx).DictionaryRelation.Query().
		Where(dictionaryrelation.IDEQ(id), dictionaryrelation.DeletedAtIsNil()).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_RELATION_NOT_FOUND", "词条关系不存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryRelationProto(row, nil), nil
}

// CreateRelation 创建词条关系。
func (r *dictionaryRepo) CreateRelation(ctx context.Context, rel *pb.DictionaryRelation) (*pb.DictionaryRelation, error) {
	create := r.Data.DB(ctx).DictionaryRelation.Create().
		SetEntryID(rel.GetEntryId()).
		SetRelationType(rel.GetRelationType()).
		SetRelatedText(rel.GetRelatedText()).
		SetSource(rel.GetSource())
	if rel.GetTargetEntryId() != 0 {
		create.SetTargetEntryID(rel.GetTargetEntryId())
	}
	if rel.GetDescription() != "" {
		create.SetDescription(rel.GetDescription())
	}
	row, err := create.Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_RELATION_EXISTS", "同词条下同类型关系已存在")
	}
	if err != nil {
		return nil, err
	}
	return r.GetRelation(ctx, row.ID)
}

// UpdateRelation 更新词条关系。
func (r *dictionaryRepo) UpdateRelation(ctx context.Context, rel *pb.DictionaryRelation) (*pb.DictionaryRelation, error) {
	update := r.Data.DB(ctx).DictionaryRelation.UpdateOneID(rel.GetId())
	if rel.GetRelationType() != "" {
		update.SetRelationType(rel.GetRelationType())
	}
	if rel.GetRelatedText() != "" {
		update.SetRelatedText(rel.GetRelatedText())
	}
	if rel.GetTargetEntryId() != 0 {
		update.SetTargetEntryID(rel.GetTargetEntryId())
	}
	if rel.GetDescription() != "" {
		update.SetDescription(rel.GetDescription())
	}
	if rel.GetStatus() != 0 {
		update.SetStatus(rel.GetStatus())
	}
	if _, err := update.Save(ctx); gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_RELATION_NOT_FOUND", "词条关系不存在")
	} else if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_RELATION_EXISTS", "同词条下同类型关系已存在")
	} else if err != nil {
		return nil, err
	}
	return r.GetRelation(ctx, rel.GetId())
}

// DeleteRelation 软删除词条关系。
func (r *dictionaryRepo) DeleteRelation(ctx context.Context, id uint32) error {
	now := time.Now()
	if err := r.Data.DB(ctx).DictionaryRelation.UpdateOneID(id).SetDeletedAt(now).Exec(ctx); gen.IsNotFound(err) {
		return errors.NotFound("DICTIONARY_RELATION_NOT_FOUND", "词条关系不存在")
	} else if err != nil {
		return err
	}
	return nil
}

// dictionaryCategoryProto converts an Ent DictionaryCategory to a proto DictionaryCategory.
func dictionaryCategoryProto(row *gen.DictionaryCategory) *pb.DictionaryCategory {
	status := int32(0)
	if row.Status != nil {
		status = *row.Status
	}
	return &pb.DictionaryCategory{
		Id:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		Builtin:   row.Builtin,
		Sort:      row.Sort,
		Status:    status,
		CreatedAt: row.CreatedAt.Format(time.DateTime),
		UpdatedAt: row.UpdatedAt.Format(time.DateTime),
	}
}

// ListCategories 分页查询词条分类（含内置 + 自定义）。
func (r *dictionaryRepo) ListCategories(ctx context.Context, req *pb.ListCategoriesRequest) ([]*pb.DictionaryCategory, int32, error) {
	query := r.Data.DB(ctx).DictionaryCategory.Query()
	if req.GetKeyword() != "" {
		query.Where(dictionarycategory.Or(
			dictionarycategory.CodeContains(req.GetKeyword()),
			dictionarycategory.NameContains(req.GetKeyword()),
		))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Asc(dictionarycategory.FieldSort), gen.Asc(dictionarycategory.FieldID)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.DictionaryCategory, 0, len(rows))
	for _, row := range rows {
		result = append(result, dictionaryCategoryProto(row))
	}
	return result, int32(total), nil
}

// CreateCategory 创建自定义分类。
func (r *dictionaryRepo) CreateCategory(ctx context.Context, cat *pb.DictionaryCategory) (*pb.DictionaryCategory, error) {
	row, err := r.Data.DB(ctx).DictionaryCategory.Create().
		SetCode(cat.GetCode()).
		SetName(cat.GetName()).
		SetBuiltin(false).
		SetSort(cat.GetSort()).
		Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_CATEGORY_EXISTS", "分类编码已存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryCategoryProto(row), nil
}

// UpdateCategory 更新分类（仅自定义，内置只读）。
func (r *dictionaryRepo) UpdateCategory(ctx context.Context, cat *pb.DictionaryCategory) (*pb.DictionaryCategory, error) {
	row, err := r.Data.DB(ctx).DictionaryCategory.Query().
		Where(dictionarycategory.IDEQ(cat.GetId())).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_CATEGORY_NOT_FOUND", "分类不存在")
	}
	if err != nil {
		return nil, err
	}
	if row.Builtin {
		return nil, errors.Forbidden("DICTIONARY_CATEGORY_BUILTIN", "内置分类不可修改")
	}
	update := r.Data.DB(ctx).DictionaryCategory.UpdateOneID(cat.GetId())
	if cat.GetName() != "" {
		update.SetName(cat.GetName())
	}
	if cat.GetSort() != 0 {
		update.SetSort(cat.GetSort())
	}
	if cat.GetStatus() != 0 {
		update.SetStatus(cat.GetStatus())
	}
	if _, err := update.Save(ctx); err != nil {
		return nil, err
	}
	return r.categoryByID(ctx, cat.GetId())
}

// DeleteCategory 删除分类（仅自定义，内置只读）。
func (r *dictionaryRepo) DeleteCategory(ctx context.Context, id uint32) error {
	row, err := r.Data.DB(ctx).DictionaryCategory.Query().
		Where(dictionarycategory.IDEQ(id)).
		Only(ctx)
	if gen.IsNotFound(err) {
		return errors.NotFound("DICTIONARY_CATEGORY_NOT_FOUND", "分类不存在")
	}
	if err != nil {
		return err
	}
	if row.Builtin {
		return errors.Forbidden("DICTIONARY_CATEGORY_BUILTIN", "内置分类不可删除")
	}
	return r.Data.DB(ctx).DictionaryCategory.DeleteOneID(id).Exec(ctx)
}

func (r *dictionaryRepo) categoryByID(ctx context.Context, id uint32) (*pb.DictionaryCategory, error) {
	row, err := r.Data.DB(ctx).DictionaryCategory.Query().Where(dictionarycategory.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return dictionaryCategoryProto(row), nil
}

// dictionaryVersionProto converts an Ent DictionaryVersion to a proto DictionaryVersion.
func dictionaryVersionProto(row *gen.DictionaryVersion) *pb.DictionaryVersion {
	status := int32(0)
	if row.Status != nil {
		status = *row.Status
	}
	return &pb.DictionaryVersion{
		Id:           row.ID,
		DictionaryId: row.DictionaryID,
		VersionNo:    row.VersionNo,
		Snapshot:     row.Snapshot,
		Description:  row.Description,
		Status:       status,
		CreatedAt:    row.CreatedAt.Format(time.DateTime),
		UpdatedAt:    row.UpdatedAt.Format(time.DateTime),
	}
}

// ListVersions 分页查询词库版本。
func (r *dictionaryRepo) ListVersions(ctx context.Context, req *pb.ListVersionsRequest) ([]*pb.DictionaryVersion, int32, error) {
	query := r.Data.DB(ctx).DictionaryVersion.Query()
	if req.GetDictionaryId() != 0 {
		query.Where(dictionaryversion.DictionaryIDEQ(req.GetDictionaryId()))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(dictionaryversion.FieldVersionNo)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.DictionaryVersion, 0, len(rows))
	for _, row := range rows {
		result = append(result, dictionaryVersionProto(row))
	}
	return result, int32(total), nil
}

// GetVersion 查询词库版本详情。
func (r *dictionaryRepo) GetVersion(ctx context.Context, id uint32) (*pb.DictionaryVersion, error) {
	row, err := r.Data.DB(ctx).DictionaryVersion.Query().
		Where(dictionaryversion.IDEQ(id)).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_VERSION_NOT_FOUND", "词库版本不存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryVersionProto(row), nil
}

// PublishDictionary 发布词库版本：快照当前词条/关系，生成递增版本号。
func (r *dictionaryRepo) PublishDictionary(ctx context.Context, req *pb.PublishDictionaryRequest) (*pb.DictionaryVersion, error) {
	entries, err := r.Data.DB(ctx).DictionaryEntry.Query().
		Where(dictionaryentry.DictionaryIDEQ(req.GetDictionaryId()), dictionaryentry.DeletedAtIsNil()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	relations, err := r.Data.DB(ctx).DictionaryRelation.Query().
		Where(dictionaryrelation.DeletedAtIsNil()).
		Where(dictionaryrelation.HasEntryWith(dictionaryentry.DictionaryIDEQ(req.GetDictionaryId()))).
		All(ctx)
	if err != nil {
		return nil, err
	}

	snapshot := struct {
		Entries   []*pb.DictionaryEntry    `json:"entries"`
		Relations []*pb.DictionaryRelation `json:"relations"`
	}{
		Entries:   make([]*pb.DictionaryEntry, 0, len(entries)),
		Relations: make([]*pb.DictionaryRelation, 0, len(relations)),
	}
	for _, e := range entries {
		snapshot.Entries = append(snapshot.Entries, dictionaryEntryProto(e))
	}
	for _, rel := range relations {
		snapshot.Relations = append(snapshot.Relations, dictionaryRelationProto(rel, nil))
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}

	// 计算递增版本号
	maxNo, err := r.Data.DB(ctx).DictionaryVersion.Query().
		Where(dictionaryversion.DictionaryIDEQ(req.GetDictionaryId())).
		Aggregate(gen.Max(dictionaryversion.FieldVersionNo)).
		Int(ctx)
	if err != nil {
		maxNo = 0
	}
	nextNo := maxNo + 1

	row, err := r.Data.DB(ctx).DictionaryVersion.Create().
		SetDictionaryID(req.GetDictionaryId()).
		SetVersionNo(int32(nextNo)).
		SetSnapshot(string(snapshotJSON)).
		SetDescription(req.GetDescription()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetVersion(ctx, row.ID)
}

// LoadVocabularyEntries 加载租户可见的全部词条+关系（含作用域），供 VocabularyContext 构建。
// 包含 Platform/System（tenant_id=0 全局）+ Tenant（tenant_id=当前租户）三个作用域。
func (r *dictionaryRepo) LoadVocabularyEntries(ctx context.Context, tenantID uint32) ([]*pb.DictionaryEntry, []biz.VocabularyRelationData, error) {
	sysCtx := entviewer.NewSystemContext(ctx)

	// 1. 查询所有未删除的词库，过滤出可见作用域
	dicts, err := r.Data.DB(sysCtx).Dictionary.Query().
		Where(dictionary.DeletedAtIsNil()).
		All(sysCtx)
	if err != nil {
		return nil, nil, err
	}
	var dictIDs []uint32
	scopeMap := make(map[uint32]string)
	for _, d := range dicts {
		if d.Scope == "TENANT" && d.TenantID != tenantID {
			continue // 其他租户的词库不可见
		}
		dictIDs = append(dictIDs, d.ID)
		scopeMap[d.ID] = d.Scope
	}
	if len(dictIDs) == 0 {
		return nil, nil, nil
	}

	// 2. 查询这些词库的词条（显式 Limit 避免 ent 全局默认 1000 限制丢失大数据）
	entryRows, err := r.Data.DB(sysCtx).DictionaryEntry.Query().
		Where(dictionaryentry.DictionaryIDIn(dictIDs...), dictionaryentry.DeletedAtIsNil()).
		Limit(100000).
		All(sysCtx)
	if err != nil {
		return nil, nil, err
	}
	entries := make([]*pb.DictionaryEntry, 0, len(entryRows))
	entryScope := make(map[uint32]string, len(entryRows))
	for _, e := range entryRows {
		entries = append(entries, dictionaryEntryProto(e))
		entryScope[e.ID] = scopeMap[e.DictionaryID]
	}

	// 3. 查询这些词条的关系（带 scope）
	entryIDs := make([]uint32, 0, len(entryRows))
	for _, e := range entryRows {
		entryIDs = append(entryIDs, e.ID)
	}
	relationRows, err := r.Data.DB(sysCtx).DictionaryRelation.Query().
		Where(dictionaryrelation.DeletedAtIsNil()).
		Where(dictionaryrelation.HasEntryWith(dictionaryentry.IDIn(entryIDs...))).
		Limit(100000).
		All(sysCtx)
	if err != nil {
		return nil, nil, err
	}
	relations := make([]biz.VocabularyRelationData, 0, len(relationRows))
	for _, rel := range relationRows {
		relations = append(relations, biz.VocabularyRelationData{
			EntryID:       rel.EntryID,
			RelationType:  rel.RelationType,
			RelatedText:   rel.RelatedText,
			TargetEntryID: rel.TargetEntryID,
			Scope:         entryScope[rel.EntryID],
		})
	}
	return entries, relations, nil
}

// GetStats 查询词库统计指标（词库详情页顶部 + 工作台健康度聚合）。
// 多租户隐私：调用 GetDictionary 触发 scope 可见性校验。
//
// 健康度字段说明（1.0 占位）：
//   - HitRate、AvgRecognitionConfidence 需 JOIN EnhancementLog + AsrRecord，1.0 返回 0，
//     前端以「数据收集中」提示；2.0 加入 Schema 补强后返回实际值。
//   - UnresolvedConflictCount：DictionaryConflict 表以 source_dictionary 字符串存储（无外键），
//     1.0 返回全局未解决冲突数；2.0 补 dictionary_id 字段后改为按词库统计。
func (r *dictionaryRepo) GetStats(ctx context.Context, id uint32) (*pb.DictionaryStats, error) {
	// 1. scope 可见性校验（复用 GetDictionary 返回 DICTIONARY_NOT_FOUND）
	if _, err := r.GetDictionary(ctx, id); err != nil {
		return nil, err
	}

	db := r.Data.DB(ctx)

	// 2. 词条计数（含禁用 + 仅启用）
	entryTotal, err := db.DictionaryEntry.Query().
		Where(dictionaryentry.DictionaryIDEQ(id), dictionaryentry.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	enabledEntry, err := db.DictionaryEntry.Query().
		Where(dictionaryentry.DictionaryIDEQ(id), dictionaryentry.DeletedAtIsNil(), dictionaryentry.StatusEQ(1)).
		Count(ctx)
	if err != nil {
		return nil, err
	}

	// 3. 关系计数（跨词条）
	relationTotal, err := db.DictionaryRelation.Query().
		Where(dictionaryrelation.DeletedAtIsNil()).
		Where(dictionaryrelation.HasEntryWith(dictionaryentry.DictionaryIDEQ(id))).
		Count(ctx)
	if err != nil {
		return nil, err
	}

	// 4. 版本计数
	versionTotal, err := db.DictionaryVersion.Query().
		Where(dictionaryversion.DictionaryIDEQ(id)).
		Count(ctx)
	if err != nil {
		return nil, err
	}

	// 5. lastModifiedAt：MAX(updated_at) from entries + relations，取较大者。
	lastModifiedAt, err := r.maxDictionaryModifiedAt(ctx, id)
	if err != nil {
		r.Log.Warnf("compute last_modified_at failed for dictionary %d: %v", id, err)
	}

	stats := &pb.DictionaryStats{
		EntryCount:                 int32(entryTotal),
		EnabledEntryCount:          int32(enabledEntry),
		RelationCount:              int32(relationTotal),
		VersionCount:               int32(versionTotal),
		UnresolvedConflictCount:    0, // 1.0 占位：返回 0，前端展示「数据收集中」
		HitRate:                    0, // 1.0 占位
		AvgRecognitionConfidence:   0, // 1.0 占位
		LastModifiedAt:             lastModifiedAt,
		DictionaryId:               id,
	}
	return stats, nil
}

// maxDictionaryModifiedAt 返回词库下词条 + 关系最后一次更新的时间戳。
// 实现方式：分别取 entries 和 relations 按 UpdatedAt 降序的首条记录，取较大者。
// 避免用 Max(time.Time) 聚合——ent 0.14.5 selector 未提供 Time() 提取方法（仅 Int/Float64/String）。
func (r *dictionaryRepo) maxDictionaryModifiedAt(ctx context.Context, dictionaryID uint32) (string, error) {
	db := r.Data.DB(ctx)
	var latest time.Time

	entry, err := db.DictionaryEntry.Query().
		Where(dictionaryentry.DictionaryIDEQ(dictionaryID), dictionaryentry.DeletedAtIsNil()).
		Order(gen.Desc(dictionaryentry.FieldUpdatedAt)).
		First(ctx)
	if err != nil && !gen.IsNotFound(err) {
		return "", err
	}
	if entry != nil {
		latest = entry.UpdatedAt
	}

	relation, err := db.DictionaryRelation.Query().
		Where(dictionaryrelation.DeletedAtIsNil()).
		Where(dictionaryrelation.HasEntryWith(dictionaryentry.DictionaryIDEQ(dictionaryID))).
		Order(gen.Desc(dictionaryrelation.FieldUpdatedAt)).
		First(ctx)
	if err != nil && !gen.IsNotFound(err) {
		return "", err
	}
	if relation != nil && relation.UpdatedAt.After(latest) {
		latest = relation.UpdatedAt
	}

	if latest.IsZero() {
		return "", nil
	}
	return latest.UTC().Format(time.DateTime), nil
}

// ===== Backend-1 词库中心交互优化工作台接口实现（2026-08-25） =====

// tenantIDFromContext 从 ctx 安全提取当前租户。
// 修复说明：原先用 ctx.Value("tenant_id").(uint32) 错误，因为 kratos authn 把 tenant_id
// 存在 SecurityUser 里，需要走 authn.GetAuthUserTenantID(ctx)。原断言失败时返回 0，
// 导致多租户过滤失效，dashboard 数据为空。
func (r *dictionaryRepo) tenantIDFromContext(ctx context.Context) uint32 {
	return authn.GetAuthUserTenantID(ctx)
}

// dashboardMyDictCard 从 Dictionary ent 生成我的词库卡片。
func dashboardMyDictCard(d *gen.Dictionary, entryCount int32, relationCount int32, conflictCount int32) *pb.DashboardMyDictionary {
	lastModified := ""
	if !d.UpdatedAt.IsZero() {
		lastModified = d.UpdatedAt.Format(time.DateTime)
	}
	return &pb.DashboardMyDictionary{
		Id:                     d.ID,
		Name:                   d.Name,
		Scope:                  d.Scope,
		EntryCount:             entryCount,
		RelationCount:          relationCount,
		UnresolvedConflictCount: conflictCount,
		LastModifiedAt:         lastModified,
	}
}

// dashboardSystemDictCard 从 Dictionary ent 生成系统词库卡片（PLATFORM/SYSTEM scope）。
func dashboardSystemDictCard(d *gen.Dictionary, entryCount int32) *pb.DashboardSystemDictionary {
	lastModified := ""
	if !d.UpdatedAt.IsZero() {
		lastModified = d.UpdatedAt.Format(time.DateTime)
	}
	return &pb.DashboardSystemDictionary{
		Id:             d.ID,
		Name:           d.Name,
		Scope:          d.Scope,
		EntryCount:     entryCount,
		LastModifiedAt: lastModified,
	}
}

// GetDashboardOverview 查询词库中心工作台总览。
// 多租户隐私：
//   - 我的词库：ctx 租户的 scope=TENANT 词库
//   - 系统词库：PLATFORM/SYSTEM 词库（全租户可见）
//   - 最近活动：ctx 租户 + PLATFORM scope 词库的事件（跨租户可见）
func (r *dictionaryRepo) GetDashboardOverview(ctx context.Context, activitiesLimit int32) (*pb.DashboardOverview, error) {
	if activitiesLimit <= 0 {
		activitiesLimit = 5
	}
	if activitiesLimit > 50 {
		activitiesLimit = 50
	}

	db := r.Data.DB(ctx)
	tenantID := r.tenantIDFromContext(ctx)

	// 1. 我的词库（scope=TENANT, tenant_id=ctx）
	myDicts, err := db.Dictionary.Query().
		Where(dictionary.DeletedAtIsNil(), dictionary.ScopeEQ("TENANT"), dictionary.TenantIDEQ(tenantID)).
		Order(gen.Desc(dictionary.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	myCards := make([]*pb.DashboardMyDictionary, 0, len(myDicts))
	totalMyEntries := int32(0)
	totalMyRelations := int32(0)
	totalMyConflicts := int32(0)
	for _, d := range myDicts {
		entryCnt, err := db.DictionaryEntry.Query().
			Where(dictionaryentry.DictionaryIDEQ(d.ID), dictionaryentry.DeletedAtIsNil()).
			Count(ctx)
		if err != nil {
			return nil, err
		}
		relCnt, err := db.DictionaryRelation.Query().
			Where(dictionaryrelation.DeletedAtIsNil()).
			Where(dictionaryrelation.HasEntryWith(dictionaryentry.DictionaryIDEQ(d.ID))).
			Count(ctx)
		if err != nil {
			return nil, err
		}
		// 冲突数：1.0 占位返回 0（DictionaryConflict.source_dictionary 为字符串，无法按 dictionary_id 过滤）
		myCards = append(myCards, dashboardMyDictCard(d, int32(entryCnt), int32(relCnt), 0))
		totalMyEntries += int32(entryCnt)
		totalMyRelations += int32(relCnt)
	}

	// 2. 系统词库（scope in PLATFORM, SYSTEM）—— 使用 sys ctx 跳过 tenant filter
	sysCtx := entviewer.NewSystemContext(ctx)
	sysDicts, err := r.Data.DB(sysCtx).Dictionary.Query().
		Where(dictionary.DeletedAtIsNil(), dictionary.ScopeIn("PLATFORM", "SYSTEM")).
		Order(gen.Desc(dictionary.FieldUpdatedAt)).
		Limit(50).
		All(sysCtx)
	if err != nil {
		return nil, err
	}
	sysCards := make([]*pb.DashboardSystemDictionary, 0, len(sysDicts))
	totalSysEntries := int32(0)
	for _, d := range sysDicts {
		entryCnt, err := r.Data.DB(sysCtx).DictionaryEntry.Query().
			Where(dictionaryentry.DictionaryIDEQ(d.ID), dictionaryentry.DeletedAtIsNil()).
			Count(sysCtx)
		if err != nil {
			return nil, err
		}
		sysCards = append(sysCards, dashboardSystemDictCard(d, int32(entryCnt)))
		totalSysEntries += int32(entryCnt)
	}

	// 3. 健康度汇总
	totalEntries := totalMyEntries + totalSysEntries
	enabledEntries := totalEntries // 1.0 不区分启用（需后续 scheme 加 status 全局聚合）
	totalRelations := totalMyRelations
	totalDicts := int32(len(myDicts) + len(sysDicts))
	health := &pb.DashboardHealthSummary{
		TotalDictionaries:       totalDicts,
		TotalEntries:            totalEntries,
		EnabledEntries:          enabledEntries,
		TotalRelations:          totalRelations,
		UnresolvedConflicts:     totalMyConflicts,
		HitRate:                 0, // 1.0 占位（需 JOIN EnhancementLog，2.0 补 schema）
		AvgRecognitionConfidence: 0,
		CoverageDictionaryCount: int32(len(myDicts)),
		TotalDictionaryCount:    totalDicts,
	}

	// 4. 最近活动（从 DictionaryChangeLog 拉取前 N 条，限制为 ctx 租户 + PLATFORM 可见）
	activities, err := r.loadRecentActivities(ctx, sysCtx, int(activitiesLimit))
	if err != nil {
		// 活动加载失败不影响主响应返回，降级为空
		r.Log.Warnf("load recent activities failed: %v", err)
		activities = nil
	}

	return &pb.DashboardOverview{
		MyDictionaries:     myCards,
		SystemDictionaries: sysCards,
		Health:             health,
		RecentActivities:   activities,
	}, nil
}

// loadRecentActivities 加载最近 N 条全局事件。
// 多租户隐私：ctx 租户的事件 + PLATFORM scope 词库的事件（跨租户可见）。
// 1.0 实现：从 DictionaryChangeLog 拉取最近 N 条，包含 entity_type/action/operator_id。
// title/summary 用 entity_type.action 模板生成；后续可在 change_log 加 actor_name、entity_name 字段优化。
func (r *dictionaryRepo) loadRecentActivities(ctx context.Context, sysCtx context.Context, limit int) ([]*pb.DashboardActivity, error) {
	if limit <= 0 {
		limit = 5
	}
	tenantID := r.tenantIDFromContext(ctx)

	// 使用 sys ctx 获取全平台变更记录，依赖 entviewer + tenant filter 双机制
	rows, err := r.Data.DB(sysCtx).DictionaryChangeLog.Query().
		Order(gen.Desc(dictionarychangelog.FieldCreatedAt)).
		Limit(limit * 2). // 预取 2 倍，FIL 过滤后取 N 条
		All(sysCtx)
	if err != nil {
		return nil, err
	}
	activities := make([]*pb.DashboardActivity, 0, limit)
	for _, row := range rows {
		// 多租户过滤：ctx 租户的事件 + PLATFORM scope 词库事件可见
		visible := row.TenantID == tenantID
		if !visible && row.TenantID != 0 { // tenantID=0 是 PLATFORM scope
			continue
		}
		scope := "TENANT"
		if row.TenantID == 0 {
			scope = "PLATFORM"
		}
		activities = append(activities, &pb.DashboardActivity{
			Id:         uint64(row.ID),
			Type:       row.EntityType + "." + row.Action,
			Title:      formatActivityTitle(row.EntityType, row.Action),
			Summary:    extractActivitySummary(row.AfterSnapshot),
			ActorId:    row.OperatorID,
			TargetType: row.EntityType,
			TargetId:   row.EntityID,
			Scope:      scope,
			CreatedAt:  row.CreatedAt.Format(time.DateTime),
		})
		if len(activities) >= limit {
			break
		}
	}
	return activities, nil
}

// formatActivityTitle 生成活动标题（如「创建了词条」）。
func formatActivityTitle(entityType, action string) string {
	switch entityType {
	case "dictionary":
		switch action {
		case "create":
			return "创建了词库"
		case "update":
			return "更新了词库"
		case "publish":
			return "发布了词库版本"
		default:
			return "词库变更"
		}
	case "entry":
		switch action {
		case "create":
			return "添加了词条"
		case "update":
			return "更新了词条"
		case "disable":
			return "停用了词条"
		default:
			return "词条变更"
		}
	case "relation":
		return "维护了词条关系"
	default:
		return action + " " + entityType
	}
}

// extractActivitySummary 从变更后快照 JSON 中提取名称（最多 80 字符）。
func extractActivitySummary(snapshot string) string {
	if snapshot == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(snapshot), &m); err != nil {
		return ""
	}
	for _, key := range []string{"name", "standard_text", "related_text"} {
		if v, ok := m[key].(string); ok && v != "" {
			if len(v) > 80 {
				return v[:80] + "..."
			}
			return v
		}
	}
	if len(snapshot) > 80 {
		return snapshot[:80] + "..."
	}
	return snapshot
}

// GetVocabularyHealth 查询词库健康度详细指标。
// 1.0 占位实现：hit_rate / avg_recognition_confidence 返回 0。
// 后续 2.0 需要：
//   - EnhancementLog 增加 dictionary_id 外键（当前只有 session_id）
//   - 通过 ASRRecord → session_id 间接 JOIN 词库
// 多租户隐私：ctx 租户的 scope=TENANT + PLATFORM/SYSTEM 词库。
func (r *dictionaryRepo) GetVocabularyHealth(ctx context.Context, scope string, recentDays int32) ([]*pb.VocabularyHealthDetail, error) {
	if recentDays <= 0 {
		recentDays = 7
	}
	if recentDays > 90 {
		recentDays = 90
	}
	tenantID := r.tenantIDFromContext(ctx)
	db := r.Data.DB(ctx)

	// 1. 查询可见词库
	query := db.Dictionary.Query().Where(dictionary.DeletedAtIsNil())
	switch scope {
	case "PLATFORM":
		query.Where(dictionary.ScopeEQ("PLATFORM"))
	case "TENANT":
		query.Where(dictionary.ScopeEQ("TENANT"), dictionary.TenantIDEQ(tenantID))
	case "ALL", "":
		// 全部可见词库：ctx 租户 + PLATFORM/SYSTEM
		query.Where(dictionary.Or(
			dictionary.ScopeEQ("PLATFORM"),
			dictionary.ScopeEQ("SYSTEM"),
			dictionary.And(dictionary.ScopeEQ("TENANT"), dictionary.TenantIDEQ(tenantID)),
		))
	default:
		return nil, errors.BadRequest("SCOPE_INVALID", "scope 参数必须是 PL/ATFORM/TENANT/ALL 之一")
	}

	dicts, err := query.Order(gen.Desc(dictionary.FieldUpdatedAt)).All(ctx)
	if err != nil {
		return nil, err
	}
	details := make([]*pb.VocabularyHealthDetail, 0, len(dicts))
	for _, d := range dicts {
		entryCnt, _ := db.DictionaryEntry.Query().
			Where(dictionaryentry.DictionaryIDEQ(d.ID), dictionaryentry.DeletedAtIsNil()).
			Count(ctx)
		relCnt, _ := db.DictionaryRelation.Query().
			Where(dictionaryrelation.DeletedAtIsNil()).
			Where(dictionaryrelation.HasEntryWith(dictionaryentry.DictionaryIDEQ(d.ID))).
			Count(ctx)
		details = append(details, &pb.VocabularyHealthDetail{
			DictionaryId:             d.ID,
			DictionaryName:           d.Name,
			EntryCount:               int32(entryCnt),
			RelationCount:            int32(relCnt),
			HitRate:                  0, // 1.0 占位
			AvgRecognitionConfidence: 0, // 1.0 占位
			SampleCount:              0,
		})
	}
	return details, nil
}
