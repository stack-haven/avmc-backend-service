package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type MenuServiceService struct {
	pb.UnimplementedMenuServiceServer
	muc *biz.MenuUsecase
	log *log.Helper
}

func NewMenuServiceService(muc *biz.MenuUsecase, logger log.Logger) *MenuServiceService {
	return &MenuServiceService{
		muc: muc,
		log: log.NewHelper(logger),
	}
}

func (s *MenuServiceService) ListMenu(ctx context.Context, req *pbPagination.PagingRequest) (*pbCore.ListMenuResponse, error) {
	return &pbCore.ListMenuResponse{}, nil
}
func (s *MenuServiceService) GetMenu(ctx context.Context, req *pbCore.GetMenuRequest) (*pbCore.Menu, error) {
	return &pbCore.Menu{}, nil
}
func (s *MenuServiceService) CreateMenu(ctx context.Context, req *pbCore.CreateMenuRequest) (*pbCore.CreateMenuResponse, error) {
	return &pbCore.CreateMenuResponse{}, nil
}
func (s *MenuServiceService) UpdateMenu(ctx context.Context, req *pbCore.UpdateMenuRequest) (*pbCore.UpdateMenuResponse, error) {
	return &pbCore.UpdateMenuResponse{}, nil
}
func (s *MenuServiceService) DeleteMenu(ctx context.Context, req *pbCore.DeleteMenuRequest) (*pbCore.DeleteMenuResponse, error) {
	return &pbCore.DeleteMenuResponse{}, nil
}
