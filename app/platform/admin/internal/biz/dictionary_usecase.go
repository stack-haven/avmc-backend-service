package biz

import (
	"context"

	pb "backend-service/api/core/service/v1"
)

type DictionaryRepo interface {
	ListTypes(context.Context, *pb.ListDictionaryTypesRequest) ([]*pb.DictionaryType, int32, error)
	GetType(context.Context, uint32) (*pb.DictionaryType, error)
	CreateType(context.Context, *pb.DictionaryType) (*pb.DictionaryType, error)
	UpdateType(context.Context, *pb.DictionaryType) (*pb.DictionaryType, error)
	DeleteType(context.Context, uint32) error
	ListItems(context.Context, *pb.ListDictionaryItemsRequest) ([]*pb.DictionaryItem, error)
	CreateItem(context.Context, *pb.DictionaryItem) (*pb.DictionaryItem, error)
	UpdateItem(context.Context, *pb.DictionaryItem) (*pb.DictionaryItem, error)
	DeleteItem(context.Context, uint32) error
}

type DictionaryUsecase struct{ repo DictionaryRepo }

func NewDictionaryUsecase(repo DictionaryRepo) *DictionaryUsecase {
	return &DictionaryUsecase{repo: repo}
}
func (uc *DictionaryUsecase) ListTypes(ctx context.Context, req *pb.ListDictionaryTypesRequest) ([]*pb.DictionaryType, int32, error) {
	return uc.repo.ListTypes(ctx, req)
}
func (uc *DictionaryUsecase) GetType(ctx context.Context, id uint32) (*pb.DictionaryType, error) {
	return uc.repo.GetType(ctx, id)
}
func (uc *DictionaryUsecase) CreateType(ctx context.Context, item *pb.DictionaryType) (*pb.DictionaryType, error) {
	return uc.repo.CreateType(ctx, item)
}
func (uc *DictionaryUsecase) UpdateType(ctx context.Context, item *pb.DictionaryType) (*pb.DictionaryType, error) {
	return uc.repo.UpdateType(ctx, item)
}
func (uc *DictionaryUsecase) DeleteType(ctx context.Context, id uint32) error {
	return uc.repo.DeleteType(ctx, id)
}
func (uc *DictionaryUsecase) ListItems(ctx context.Context, req *pb.ListDictionaryItemsRequest) ([]*pb.DictionaryItem, error) {
	return uc.repo.ListItems(ctx, req)
}
func (uc *DictionaryUsecase) CreateItem(ctx context.Context, item *pb.DictionaryItem) (*pb.DictionaryItem, error) {
	return uc.repo.CreateItem(ctx, item)
}
func (uc *DictionaryUsecase) UpdateItem(ctx context.Context, item *pb.DictionaryItem) (*pb.DictionaryItem, error) {
	return uc.repo.UpdateItem(ctx, item)
}
func (uc *DictionaryUsecase) DeleteItem(ctx context.Context, id uint32) error {
	return uc.repo.DeleteItem(ctx, id)
}
