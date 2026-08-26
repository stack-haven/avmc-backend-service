package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/pkg/auth/authn"
)

// DictionaryServiceService 词库中心服务（词库 + 词条）。
type DictionaryServiceService struct {
	pb.UnimplementedDictionaryServiceServer
	uc    *biz.DictionaryUsecase
	vocab *biz.VocabularyBuilder
	log   *log.Helper
}

// NewDictionaryServiceService 创建词库中心服务实例。
func NewDictionaryServiceService(uc *biz.DictionaryUsecase, vocab *biz.VocabularyBuilder, logger log.Logger) *DictionaryServiceService {
	return &DictionaryServiceService{uc: uc, vocab: vocab, log: log.NewHelper(logger)}
}

// invalidateVocab 词库/词条/关系变更后失效词库上下文缓存，保证下一次增强请求使用最新词条。
func (s *DictionaryServiceService) invalidateVocab(ctx context.Context) {
	if s.vocab != nil {
		s.vocab.Invalidate(authn.GetAuthUserTenantID(ctx))
	}
}

// ListDictionaries 分页查询词库列表。
func (s *DictionaryServiceService) ListDictionaries(ctx context.Context, req *pb.ListDictionariesRequest) (*pb.ListDictionariesResponse, error) {
	dictionaries, total, err := s.uc.ListDictionaries(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListDictionariesResponse{Dictionaries: dictionaries, Total: total}, nil
}

// GetDictionary 查询词库详情。
func (s *DictionaryServiceService) GetDictionary(ctx context.Context, req *pb.GetDictionaryRequest) (*pb.Dictionary, error) {
	return s.uc.GetDictionary(ctx, req.GetId())
}

// CreateDictionary 创建词库。
func (s *DictionaryServiceService) CreateDictionary(ctx context.Context, req *pb.CreateDictionaryRequest) (*pb.Dictionary, error) {
	result, err := s.uc.CreateDictionary(ctx, &pb.Dictionary{
		Name:        req.GetName(),
		Scope:       req.GetScope(),
		Source:      req.GetSource(),
		Description: req.GetDescription(),
	})
	if err == nil {
		s.invalidateVocab(ctx)
	}
	return result, err
}

// UpdateDictionary 更新词库。
func (s *DictionaryServiceService) UpdateDictionary(ctx context.Context, req *pb.UpdateDictionaryRequest) (*pb.Dictionary, error) {
	result, err := s.uc.UpdateDictionary(ctx, &pb.Dictionary{
		Id:          req.GetId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Status:      req.GetStatus(),
	})
	if err == nil {
		s.invalidateVocab(ctx)
	}
	return result, err
}

// DeleteDictionary 删除词库。
func (s *DictionaryServiceService) DeleteDictionary(ctx context.Context, req *pb.DeleteDictionaryRequest) (*pb.DeleteDictionaryResponse, error) {
	if err := s.uc.DeleteDictionary(ctx, req.GetId()); err != nil {
		return nil, err
	}
	s.invalidateVocab(ctx)
	return &pb.DeleteDictionaryResponse{}, nil
}

// ListEntries 分页查询词条列表。
func (s *DictionaryServiceService) ListEntries(ctx context.Context, req *pb.ListEntriesRequest) (*pb.ListEntriesResponse, error) {
	entries, total, err := s.uc.ListEntries(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListEntriesResponse{Entries: entries, Total: total}, nil
}

// GetEntry 查询词条详情。
func (s *DictionaryServiceService) GetEntry(ctx context.Context, req *pb.GetEntryRequest) (*pb.DictionaryEntry, error) {
	return s.uc.GetEntry(ctx, req.GetId())
}

// CreateEntry 创建词条。
func (s *DictionaryServiceService) CreateEntry(ctx context.Context, req *pb.CreateEntryRequest) (*pb.DictionaryEntry, error) {
	result, err := s.uc.CreateEntry(ctx, &pb.DictionaryEntry{
		DictionaryId:   req.GetDictionaryId(),
		StandardText:   req.GetStandardText(),
		EntryType:      req.GetEntryType(),
		Category:       req.GetCategory(),
		Description:    req.GetDescription(),
		Source:         req.GetSource(),
		SourceId:       req.GetSourceId(),
		Priority:       req.GetPriority(),
		Pinyin:         req.GetPinyin(),
		PinyinInitial:  req.GetPinyinInitial(),
		NormalizedText: req.GetNormalizedText(),
	})
	if err == nil {
		s.invalidateVocab(ctx)
	}
	return result, err
}

// UpdateEntry 更新词条。
func (s *DictionaryServiceService) UpdateEntry(ctx context.Context, req *pb.UpdateEntryRequest) (*pb.DictionaryEntry, error) {
	result, err := s.uc.UpdateEntry(ctx, &pb.DictionaryEntry{
		Id:             req.GetId(),
		StandardText:   req.GetStandardText(),
		EntryType:      req.GetEntryType(),
		Category:       req.GetCategory(),
		Description:    req.GetDescription(),
		Priority:       req.GetPriority(),
		Pinyin:         req.GetPinyin(),
		PinyinInitial:  req.GetPinyinInitial(),
		NormalizedText: req.GetNormalizedText(),
		Status:         req.GetStatus(),
	})
	if err == nil {
		s.invalidateVocab(ctx)
	}
	return result, err
}

// DeleteEntry 删除词条。
func (s *DictionaryServiceService) DeleteEntry(ctx context.Context, req *pb.DeleteEntryRequest) (*pb.DeleteEntryResponse, error) {
	if err := s.uc.DeleteEntry(ctx, req.GetId()); err != nil {
		return nil, err
	}
	s.invalidateVocab(ctx)
	return &pb.DeleteEntryResponse{}, nil
}

// ListRelations 分页查询词条关系列表。
func (s *DictionaryServiceService) ListRelations(ctx context.Context, req *pb.ListRelationsRequest) (*pb.ListRelationsResponse, error) {
	relations, total, err := s.uc.ListRelations(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListRelationsResponse{Relations: relations, Total: total}, nil
}

// ListRelationsByDictionary 词库级别关系列表（不需先选 entryId，响应含 entry_standard_text 等 JOIN 字段）。
func (s *DictionaryServiceService) ListRelationsByDictionary(ctx context.Context, req *pb.ListRelationsByDictionaryRequest) (*pb.ListRelationsByDictionaryResponse, error) {
	relations, total, err := s.uc.ListRelationsByDictionary(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListRelationsByDictionaryResponse{Relations: relations, Total: total}, nil
}

// GetDictionaryStats 查询词库统计指标（词库详情页顶部 + 工作台健康度聚合）。
func (s *DictionaryServiceService) GetDictionaryStats(ctx context.Context, req *pb.GetDictionaryStatsRequest) (*pb.GetDictionaryStatsResponse, error) {
	stats, err := s.uc.GetStats(ctx, req.GetDictionaryId())
	if err != nil {
		return nil, err
	}
	return &pb.GetDictionaryStatsResponse{Stats: stats}, nil
}

// GetRelation 查询词条关系详情。
func (s *DictionaryServiceService) GetRelation(ctx context.Context, req *pb.GetRelationRequest) (*pb.DictionaryRelation, error) {
	return s.uc.GetRelation(ctx, req.GetId())
}

// CreateRelation 创建词条关系。
func (s *DictionaryServiceService) CreateRelation(ctx context.Context, req *pb.CreateRelationRequest) (*pb.DictionaryRelation, error) {
	result, err := s.uc.CreateRelation(ctx, &pb.DictionaryRelation{
		EntryId:       req.GetEntryId(),
		RelationType:  req.GetRelationType(),
		RelatedText:   req.GetRelatedText(),
		TargetEntryId: req.GetTargetEntryId(),
		Source:        req.GetSource(),
		Description:   req.GetDescription(),
	})
	if err == nil {
		s.invalidateVocab(ctx)
	}
	return result, err
}

// UpdateRelation 更新词条关系。
func (s *DictionaryServiceService) UpdateRelation(ctx context.Context, req *pb.UpdateRelationRequest) (*pb.DictionaryRelation, error) {
	result, err := s.uc.UpdateRelation(ctx, &pb.DictionaryRelation{
		Id:            req.GetId(),
		RelationType:  req.GetRelationType(),
		RelatedText:   req.GetRelatedText(),
		TargetEntryId: req.GetTargetEntryId(),
		Description:   req.GetDescription(),
		Status:        req.GetStatus(),
	})
	if err == nil {
		s.invalidateVocab(ctx)
	}
	return result, err
}

// DeleteRelation 删除词条关系。
func (s *DictionaryServiceService) DeleteRelation(ctx context.Context, req *pb.DeleteRelationRequest) (*pb.DeleteRelationResponse, error) {
	if err := s.uc.DeleteRelation(ctx, req.GetId()); err != nil {
		return nil, err
	}
	s.invalidateVocab(ctx)
	return &pb.DeleteRelationResponse{}, nil
}

// ListCategories 分页查询词条分类列表。
func (s *DictionaryServiceService) ListCategories(ctx context.Context, req *pb.ListCategoriesRequest) (*pb.ListCategoriesResponse, error) {
	categories, total, err := s.uc.ListCategories(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListCategoriesResponse{Categories: categories, Total: total}, nil
}

// CreateCategory 创建词条分类。
func (s *DictionaryServiceService) CreateCategory(ctx context.Context, req *pb.CreateCategoryRequest) (*pb.DictionaryCategory, error) {
	return s.uc.CreateCategory(ctx, &pb.DictionaryCategory{
		Code: req.GetCode(),
		Name: req.GetName(),
		Sort: req.GetSort(),
	})
}

// UpdateCategory 更新词条分类。
func (s *DictionaryServiceService) UpdateCategory(ctx context.Context, req *pb.UpdateCategoryRequest) (*pb.DictionaryCategory, error) {
	return s.uc.UpdateCategory(ctx, &pb.DictionaryCategory{
		Id:     req.GetId(),
		Name:   req.GetName(),
		Sort:   req.GetSort(),
		Status: req.GetStatus(),
	})
}

// DeleteCategory 删除词条分类。
func (s *DictionaryServiceService) DeleteCategory(ctx context.Context, req *pb.DeleteCategoryRequest) (*pb.DeleteCategoryResponse, error) {
	if err := s.uc.DeleteCategory(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteCategoryResponse{}, nil
}

// ListVersions 分页查询词库版本列表。
func (s *DictionaryServiceService) ListVersions(ctx context.Context, req *pb.ListVersionsRequest) (*pb.ListVersionsResponse, error) {
	versions, total, err := s.uc.ListVersions(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListVersionsResponse{Versions: versions, Total: total}, nil
}

// GetVersion 查询词库版本详情。
func (s *DictionaryServiceService) GetVersion(ctx context.Context, req *pb.GetVersionRequest) (*pb.DictionaryVersion, error) {
	return s.uc.GetVersion(ctx, req.GetId())
}

// PublishDictionary 发布词库版本。
func (s *DictionaryServiceService) PublishDictionary(ctx context.Context, req *pb.PublishDictionaryRequest) (*pb.DictionaryVersion, error) {
	result, err := s.uc.PublishDictionary(ctx, req)
	if err == nil {
		s.invalidateVocab(ctx)
	}
	return result, err
}

// ListConflicts 查询词库冲突记录。
func (s *DictionaryServiceService) ListConflicts(ctx context.Context, _ *pb.ListConflictsRequest) (*pb.ListConflictsResponse, error) {
	conflicts, err := s.uc.ListConflicts(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListConflictsResponse{Conflicts: conflicts, Total: int32(len(conflicts))}, nil
}
