package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// DictionaryRepo 字典中心仓库接口。
type DictionaryRepo interface {
	ListWords(ctx context.Context, req *pb.ListWordsRequest) ([]*pb.DictionaryWord, int32, error)
	GetWord(ctx context.Context, id uint32) (*pb.DictionaryWord, error)
	CreateWord(ctx context.Context, word *pb.DictionaryWord) (*pb.DictionaryWord, error)
	UpdateWord(ctx context.Context, word *pb.DictionaryWord) (*pb.DictionaryWord, error)
	DeleteWord(ctx context.Context, id uint32) error
	// ListActiveWords 返回全量启用的标准词与别名（供纠错器模糊匹配）。
	ListActiveWords(ctx context.Context) ([]string, error)
}

// DictionaryUsecase 字典中心业务逻辑。
type DictionaryUsecase struct {
	repo DictionaryRepo
	log  *log.Helper
}

// NewDictionaryUsecase 创建字典中心 usecase。
func NewDictionaryUsecase(repo DictionaryRepo, logger log.Logger) *DictionaryUsecase {
	return &DictionaryUsecase{repo: repo, log: log.NewHelper(logger)}
}

// ListWords 分页查询标准词。
func (uc *DictionaryUsecase) ListWords(ctx context.Context, req *pb.ListWordsRequest) ([]*pb.DictionaryWord, int32, error) {
	return uc.repo.ListWords(ctx, req)
}

// GetWord 查询标准词详情。
func (uc *DictionaryUsecase) GetWord(ctx context.Context, id uint32) (*pb.DictionaryWord, error) {
	return uc.repo.GetWord(ctx, id)
}

// CreateWord 创建标准词（可同时指定别名）。
func (uc *DictionaryUsecase) CreateWord(ctx context.Context, word *pb.DictionaryWord) (*pb.DictionaryWord, error) {
	if word.GetWord() == "" {
		return nil, pb.ErrorBadRequest("标准词不能为空")
	}
	if word.GetCategory() == "" {
		word.Category = "term"
	}
	if word.GetLevel() == "" {
		word.Level = "tenant"
	}
	if word.GetSource() == "" {
		word.Source = "manual"
	}
	for _, alias := range word.GetAliases() {
		if alias.GetSource() == "" {
			alias.Source = "manual"
		}
		if alias.GetWeight() == 0 {
			alias.Weight = 1.0
		}
	}
	return uc.repo.CreateWord(ctx, word)
}

// UpdateWord 更新标准词。
func (uc *DictionaryUsecase) UpdateWord(ctx context.Context, word *pb.DictionaryWord) (*pb.DictionaryWord, error) {
	if word.GetId() == 0 {
		return nil, pb.ErrorBadRequest("标准词ID不能为空")
	}
	return uc.repo.UpdateWord(ctx, word)
}

// DeleteWord 删除标准词（级联软删除别名）。
func (uc *DictionaryUsecase) DeleteWord(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("标准词ID不能为空")
	}
	return uc.repo.DeleteWord(ctx, id)
}
