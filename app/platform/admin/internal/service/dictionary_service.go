package service

import (
	"context"
	"strconv"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"google.golang.org/protobuf/types/known/emptypb"
)

type DictionaryServiceService struct {
	pb.UnimplementedDictionaryServiceServer
	uc *biz.DictionaryUsecase
}

func NewDictionaryServiceService(uc *biz.DictionaryUsecase) *DictionaryServiceService {
	return &DictionaryServiceService{uc: uc}
}
func (s *DictionaryServiceService) ListDictionaryTypes(ctx context.Context, req *pbCore.ListDictionaryTypesRequest) (*pbCore.ListDictionaryTypesResponse, error) {
	items, total, err := s.uc.ListTypes(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListDictionaryTypesResponse{Items: items, Total: total}
	offset, _ := strconv.Atoi(req.GetPageToken())
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}
func (s *DictionaryServiceService) GetDictionaryType(ctx context.Context, req *pbCore.GetDictionaryTypeRequest) (*pbCore.DictionaryType, error) {
	return s.uc.GetType(ctx, req.GetId())
}
func (s *DictionaryServiceService) CreateDictionaryType(ctx context.Context, req *pbCore.CreateDictionaryTypeRequest) (*pbCore.DictionaryType, error) {
	return s.uc.CreateType(ctx, req.GetDictionary())
}
func (s *DictionaryServiceService) UpdateDictionaryType(ctx context.Context, req *pbCore.UpdateDictionaryTypeRequest) (*pbCore.DictionaryType, error) {
	req.Dictionary.Id = req.GetId()
	return s.uc.UpdateType(ctx, req.GetDictionary())
}
func (s *DictionaryServiceService) DeleteDictionaryType(ctx context.Context, req *pbCore.DeleteDictionaryTypeRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteType(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
func (s *DictionaryServiceService) ListDictionaryItems(ctx context.Context, req *pbCore.ListDictionaryItemsRequest) (*pbCore.ListDictionaryItemsResponse, error) {
	items, err := s.uc.ListItems(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.ListDictionaryItemsResponse{Items: items}, nil
}
func (s *DictionaryServiceService) CreateDictionaryItem(ctx context.Context, req *pbCore.CreateDictionaryItemRequest) (*pbCore.DictionaryItem, error) {
	return s.uc.CreateItem(ctx, req.GetItem())
}
func (s *DictionaryServiceService) UpdateDictionaryItem(ctx context.Context, req *pbCore.UpdateDictionaryItemRequest) (*pbCore.DictionaryItem, error) {
	req.Item.Id = req.GetId()
	return s.uc.UpdateItem(ctx, req.GetItem())
}
func (s *DictionaryServiceService) DeleteDictionaryItem(ctx context.Context, req *pbCore.DeleteDictionaryItemRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteItem(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
