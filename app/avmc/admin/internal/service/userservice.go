package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type UserServiceService struct {
	pb.UnimplementedUserServiceServer
	uuc *biz.UserUsecase
	log *log.Helper
}

func NewUserServiceService(uuc *biz.UserUsecase, logger log.Logger) *UserServiceService {
	return &UserServiceService{
		uuc: uuc,
		log: log.NewHelper(logger),
	}
}

func (s *UserServiceService) ListUser(ctx context.Context, req *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	return s.uuc.ListPage(ctx, req)
}
func (s *UserServiceService) GetUser(ctx context.Context, req *pbCore.GetUserRequest) (*pbCore.User, error) {
	return s.uuc.Get(ctx, req.Id)
}
func (s *UserServiceService) CreateUser(ctx context.Context, req *pbCore.CreateUserRequest) (*pbCore.CreateUserResponse, error) {
	_, err := s.uuc.Create(ctx, req.User)
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateUserResponse{}, nil
}
func (s *UserServiceService) UpdateUser(ctx context.Context, req *pbCore.UpdateUserRequest) (*pbCore.UpdateUserResponse, error) {
	_, err := s.uuc.Update(ctx, req.User)
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateUserResponse{}, nil
}
func (s *UserServiceService) DeleteUser(ctx context.Context, req *pbCore.DeleteUserRequest) (*pbCore.DeleteUserResponse, error) {
	err := s.uuc.Delete(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pbCore.DeleteUserResponse{}, nil
}
