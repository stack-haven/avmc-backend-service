package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// HotwordRepo 热词仓库接口。
type HotwordRepo interface {
	List(ctx context.Context, category string) ([]*pb.Hotword, error)
	Upsert(ctx context.Context, hotword *pb.Hotword) (*pb.Hotword, error)
	Delete(ctx context.Context, id uint32) error
}

// HotwordUsecase 热词业务逻辑。
type HotwordUsecase struct {
	repo HotwordRepo
	log  *log.Helper
}

// NewHotwordUsecase 创建热词 usecase。
func NewHotwordUsecase(repo HotwordRepo, logger log.Logger) *HotwordUsecase {
	return &HotwordUsecase{repo: repo, log: log.NewHelper(logger)}
}

// List 查询热词列表。
func (uc *HotwordUsecase) List(ctx context.Context, category string) ([]*pb.Hotword, error) {
	return uc.repo.List(ctx, category)
}

// Upsert 新增或更新热词。
func (uc *HotwordUsecase) Upsert(ctx context.Context, hotword *pb.Hotword) (*pb.Hotword, error) {
	if hotword.GetWord() == "" {
		return nil, pb.ErrorBadRequest("热词不能为空")
	}
	if hotword.GetCategory() == "" {
		hotword.Category = "term"
	}
	if hotword.GetWeight() == 0 {
		hotword.Weight = 5.0
	}
	return uc.repo.Upsert(ctx, hotword)
}

// Delete 删除热词。
func (uc *HotwordUsecase) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("热词ID不能为空")
	}
	return uc.repo.Delete(ctx, id)
}
