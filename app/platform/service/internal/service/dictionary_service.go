package service

import (
	"context"
	"strconv"

	"google.golang.org/protobuf/types/known/emptypb"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
)

type DictionaryServiceService struct {
	pb.UnimplementedDictionaryServiceServer
	uc *biz.DictionaryUsecase
}

func NewDictionaryServiceService(uc *biz.DictionaryUsecase) *DictionaryServiceService {
	return &DictionaryServiceService{uc: uc}
}
func (s *DictionaryServiceService) ListDictionaryTypes(ctx context.Context, req *pb.ListDictionaryTypesRequest) (*pb.ListDictionaryTypesResponse, error) {
	items, total, err := s.uc.ListTypes(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListDictionaryTypesResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}
func (s *DictionaryServiceService) GetDictionaryType(ctx context.Context, req *pb.GetDictionaryTypeRequest) (*pb.DictionaryType, error) {
	return s.uc.GetType(ctx, req.GetId())
}
func (s *DictionaryServiceService) CreateDictionaryType(ctx context.Context, req *pb.CreateDictionaryTypeRequest) (*pb.DictionaryType, error) {
	return s.uc.CreateType(ctx, req.GetDictionary())
}
func (s *DictionaryServiceService) UpdateDictionaryType(ctx context.Context, req *pb.UpdateDictionaryTypeRequest) (*pb.DictionaryType, error) {
	req.Dictionary.Id = req.GetId()
	return s.uc.UpdateType(ctx, req.GetDictionary())
}
func (s *DictionaryServiceService) DeleteDictionaryType(ctx context.Context, req *pb.DeleteDictionaryTypeRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteType(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
func (s *DictionaryServiceService) ListDictionaryItems(ctx context.Context, req *pb.ListDictionaryItemsRequest) (*pb.ListDictionaryItemsResponse, error) {
	items, err := s.uc.ListItems(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListDictionaryItemsResponse{Items: items}, nil
}
func (s *DictionaryServiceService) CreateDictionaryItem(ctx context.Context, req *pb.CreateDictionaryItemRequest) (*pb.DictionaryItem, error) {
	return s.uc.CreateItem(ctx, req.GetItem())
}
func (s *DictionaryServiceService) UpdateDictionaryItem(ctx context.Context, req *pb.UpdateDictionaryItemRequest) (*pb.DictionaryItem, error) {
	req.Item.Id = req.GetId()
	return s.uc.UpdateItem(ctx, req.GetItem())
}
func (s *DictionaryServiceService) DeleteDictionaryItem(ctx context.Context, req *pb.DeleteDictionaryItemRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteItem(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
