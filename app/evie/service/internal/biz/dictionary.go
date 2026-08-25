package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// DictionaryRepo 词库中心仓库接口（词库 + 词条）。
type DictionaryRepo interface {
	ListDictionaries(ctx context.Context, req *pb.ListDictionariesRequest) ([]*pb.Dictionary, int32, error)
	GetDictionary(ctx context.Context, id uint32) (*pb.Dictionary, error)
	CreateDictionary(ctx context.Context, dictionary *pb.Dictionary) (*pb.Dictionary, error)
	UpdateDictionary(ctx context.Context, dictionary *pb.Dictionary) (*pb.Dictionary, error)
	DeleteDictionary(ctx context.Context, id uint32) error

	ListEntries(ctx context.Context, req *pb.ListEntriesRequest) ([]*pb.DictionaryEntry, int32, error)
	GetEntry(ctx context.Context, id uint32) (*pb.DictionaryEntry, error)
	CreateEntry(ctx context.Context, entry *pb.DictionaryEntry) (*pb.DictionaryEntry, error)
	UpdateEntry(ctx context.Context, entry *pb.DictionaryEntry) (*pb.DictionaryEntry, error)
	DeleteEntry(ctx context.Context, id uint32) error
	// ListActiveEntryTexts 返回全量启用的标准词（供文本增强引擎模糊匹配）。
	ListActiveEntryTexts(ctx context.Context) ([]string, error)

	ListRelations(ctx context.Context, req *pb.ListRelationsRequest) ([]*pb.DictionaryRelation, int32, error)
	GetRelation(ctx context.Context, id uint32) (*pb.DictionaryRelation, error)
	CreateRelation(ctx context.Context, relation *pb.DictionaryRelation) (*pb.DictionaryRelation, error)
	UpdateRelation(ctx context.Context, relation *pb.DictionaryRelation) (*pb.DictionaryRelation, error)
	DeleteRelation(ctx context.Context, id uint32) error

	ListCategories(ctx context.Context, req *pb.ListCategoriesRequest) ([]*pb.DictionaryCategory, int32, error)
	CreateCategory(ctx context.Context, category *pb.DictionaryCategory) (*pb.DictionaryCategory, error)
	UpdateCategory(ctx context.Context, category *pb.DictionaryCategory) (*pb.DictionaryCategory, error)
	DeleteCategory(ctx context.Context, id uint32) error

	ListVersions(ctx context.Context, req *pb.ListVersionsRequest) ([]*pb.DictionaryVersion, int32, error)
	GetVersion(ctx context.Context, id uint32) (*pb.DictionaryVersion, error)
	PublishDictionary(ctx context.Context, req *pb.PublishDictionaryRequest) (*pb.DictionaryVersion, error)
	// LoadVocabularyEntries 加载租户可见的全部词条+关系（含作用域），供 VocabularyContext 构建。
	LoadVocabularyEntries(ctx context.Context, tenantID uint32) ([]*pb.DictionaryEntry, []VocabularyRelationData, error)
}

// DictionaryUsecase 词库中心业务逻辑（词库 + 词条）。
type DictionaryUsecase struct {
	repo     DictionaryRepo
	confRec  DictionaryConflictRecorder
	log      *log.Helper
}

// NewDictionaryUsecase 创建词库中心 usecase。
func NewDictionaryUsecase(repo DictionaryRepo, confRec DictionaryConflictRecorder, logger log.Logger) *DictionaryUsecase {
	return &DictionaryUsecase{repo: repo, confRec: confRec, log: log.NewHelper(logger)}
}

// ListDictionaries 分页查询词库。
func (uc *DictionaryUsecase) ListDictionaries(ctx context.Context, req *pb.ListDictionariesRequest) ([]*pb.Dictionary, int32, error) {
	return uc.repo.ListDictionaries(ctx, req)
}

// GetDictionary 查询词库详情。
func (uc *DictionaryUsecase) GetDictionary(ctx context.Context, id uint32) (*pb.Dictionary, error) {
	return uc.repo.GetDictionary(ctx, id)
}

// CreateDictionary 创建词库。
func (uc *DictionaryUsecase) CreateDictionary(ctx context.Context, dictionary *pb.Dictionary) (*pb.Dictionary, error) {
	if dictionary.GetName() == "" {
		return nil, pb.ErrorBadRequest("词库名称不能为空")
	}
	if dictionary.GetScope() == "" {
		dictionary.Scope = "TENANT"
	}
	if dictionary.GetSource() == "" {
		dictionary.Source = "MANUAL"
	}
	return uc.repo.CreateDictionary(ctx, dictionary)
}

// UpdateDictionary 更新词库。
func (uc *DictionaryUsecase) UpdateDictionary(ctx context.Context, dictionary *pb.Dictionary) (*pb.Dictionary, error) {
	if dictionary.GetId() == 0 {
		return nil, pb.ErrorBadRequest("词库ID不能为空")
	}
	return uc.repo.UpdateDictionary(ctx, dictionary)
}

// DeleteDictionary 删除词库（软删除）。
func (uc *DictionaryUsecase) DeleteDictionary(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("词库ID不能为空")
	}
	return uc.repo.DeleteDictionary(ctx, id)
}

// ListEntries 分页查询词条。
func (uc *DictionaryUsecase) ListEntries(ctx context.Context, req *pb.ListEntriesRequest) ([]*pb.DictionaryEntry, int32, error) {
	return uc.repo.ListEntries(ctx, req)
}

// GetEntry 查询词条详情。
func (uc *DictionaryUsecase) GetEntry(ctx context.Context, id uint32) (*pb.DictionaryEntry, error) {
	return uc.repo.GetEntry(ctx, id)
}

// CreateEntry 创建词条。
func (uc *DictionaryUsecase) CreateEntry(ctx context.Context, entry *pb.DictionaryEntry) (*pb.DictionaryEntry, error) {
	if entry.GetDictionaryId() == 0 {
		return nil, pb.ErrorBadRequest("词条必须属于一个词库")
	}
	if entry.GetStandardText() == "" {
		return nil, pb.ErrorBadRequest("标准词不能为空")
	}
	if entry.GetEntryType() == "" {
		entry.EntryType = "WORD"
	}
	if entry.GetCategory() == "" {
		entry.Category = "OTHER"
	}
	if entry.GetSource() == "" {
		entry.Source = "MANUAL"
	}
	return uc.repo.CreateEntry(ctx, entry)
}

// UpdateEntry 更新词条。
func (uc *DictionaryUsecase) UpdateEntry(ctx context.Context, entry *pb.DictionaryEntry) (*pb.DictionaryEntry, error) {
	if entry.GetId() == 0 {
		return nil, pb.ErrorBadRequest("词条ID不能为空")
	}
	return uc.repo.UpdateEntry(ctx, entry)
}

// DeleteEntry 删除词条（软删除）。
func (uc *DictionaryUsecase) DeleteEntry(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("词条ID不能为空")
	}
	return uc.repo.DeleteEntry(ctx, id)
}

// ListRelations 分页查询词条关系。
func (uc *DictionaryUsecase) ListRelations(ctx context.Context, req *pb.ListRelationsRequest) ([]*pb.DictionaryRelation, int32, error) {
	return uc.repo.ListRelations(ctx, req)
}

// GetRelation 查询词条关系详情。
func (uc *DictionaryUsecase) GetRelation(ctx context.Context, id uint32) (*pb.DictionaryRelation, error) {
	return uc.repo.GetRelation(ctx, id)
}

// CreateRelation 创建词条关系。
func (uc *DictionaryUsecase) CreateRelation(ctx context.Context, relation *pb.DictionaryRelation) (*pb.DictionaryRelation, error) {
	if relation.GetEntryId() == 0 {
		return nil, pb.ErrorBadRequest("关系必须属于一个词条")
	}
	if relation.GetRelationType() == "" {
		return nil, pb.ErrorBadRequest("关系类型不能为空")
	}
	if relation.GetRelatedText() == "" {
		return nil, pb.ErrorBadRequest("关联表达不能为空")
	}
	if relation.GetSource() == "" {
		relation.Source = "MANUAL"
	}
	return uc.repo.CreateRelation(ctx, relation)
}

// UpdateRelation 更新词条关系。
func (uc *DictionaryUsecase) UpdateRelation(ctx context.Context, relation *pb.DictionaryRelation) (*pb.DictionaryRelation, error) {
	if relation.GetId() == 0 {
		return nil, pb.ErrorBadRequest("关系ID不能为空")
	}
	return uc.repo.UpdateRelation(ctx, relation)
}

// DeleteRelation 删除词条关系（软删除）。
func (uc *DictionaryUsecase) DeleteRelation(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("关系ID不能为空")
	}
	return uc.repo.DeleteRelation(ctx, id)
}

// ListCategories 分页查询词条分类。
func (uc *DictionaryUsecase) ListCategories(ctx context.Context, req *pb.ListCategoriesRequest) ([]*pb.DictionaryCategory, int32, error) {
	return uc.repo.ListCategories(ctx, req)
}

// CreateCategory 创建自定义分类。
func (uc *DictionaryUsecase) CreateCategory(ctx context.Context, category *pb.DictionaryCategory) (*pb.DictionaryCategory, error) {
	if category.GetCode() == "" || category.GetName() == "" {
		return nil, pb.ErrorBadRequest("分类编码和名称不能为空")
	}
	return uc.repo.CreateCategory(ctx, category)
}

// UpdateCategory 更新分类。
func (uc *DictionaryUsecase) UpdateCategory(ctx context.Context, category *pb.DictionaryCategory) (*pb.DictionaryCategory, error) {
	if category.GetId() == 0 {
		return nil, pb.ErrorBadRequest("分类ID不能为空")
	}
	return uc.repo.UpdateCategory(ctx, category)
}

// DeleteCategory 删除分类（仅自定义）。
func (uc *DictionaryUsecase) DeleteCategory(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("分类ID不能为空")
	}
	return uc.repo.DeleteCategory(ctx, id)
}

// ListVersions 分页查询词库版本。
func (uc *DictionaryUsecase) ListVersions(ctx context.Context, req *pb.ListVersionsRequest) ([]*pb.DictionaryVersion, int32, error) {
	return uc.repo.ListVersions(ctx, req)
}

// GetVersion 查询词库版本详情。
func (uc *DictionaryUsecase) GetVersion(ctx context.Context, id uint32) (*pb.DictionaryVersion, error) {
	return uc.repo.GetVersion(ctx, id)
}

// PublishDictionary 发布词库版本（生成时点快照）。
func (uc *DictionaryUsecase) PublishDictionary(ctx context.Context, req *pb.PublishDictionaryRequest) (*pb.DictionaryVersion, error) {
	if req.GetDictionaryId() == 0 {
		return nil, pb.ErrorBadRequest("词库ID不能为空")
	}
	return uc.repo.PublishDictionary(ctx, req)
}

// ListConflicts 查询词库冲突记录。
func (uc *DictionaryUsecase) ListConflicts(ctx context.Context) ([]*pb.DictionaryConflict, error) {
	if uc.confRec == nil {
		return nil, nil
	}
	conflicts, err := uc.confRec.ListConflicts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*pb.DictionaryConflict, 0, len(conflicts))
	for _, c := range conflicts {
		result = append(result, &pb.DictionaryConflict{
			Input:             c.Input,
			Candidate:         c.Candidate,
			SourceScope:       c.SourceScope,
			SourceDictionary:  c.SourceDictionary,
			Priority:          c.Priority,
			ResolvedCandidate: c.ResolvedCandidate,
		})
	}
	return result, nil
}
