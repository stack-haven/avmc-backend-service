package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type PostServiceService struct {
	pb.UnimplementedPostServiceServer
	puc *biz.PostUsecase
	log *log.Helper
}

func NewPostServiceService(puc *biz.PostUsecase, logger log.Logger) *PostServiceService {
	return &PostServiceService{
		puc: puc,
		log: log.NewHelper(logger),
	}
}

func (s *PostServiceService) ListPost(ctx context.Context, req *pbPagination.PagingRequest) (*pbCore.ListPostResponse, error) {
	return &pbCore.ListPostResponse{}, nil
}
func (s *PostServiceService) GetPost(ctx context.Context, req *pbCore.GetPostRequest) (*pbCore.Post, error) {
	return &pbCore.Post{}, nil
}
func (s *PostServiceService) CreatePost(ctx context.Context, req *pbCore.CreatePostRequest) (*pbCore.CreatePostResponse, error) {
	return &pbCore.CreatePostResponse{}, nil
}
func (s *PostServiceService) UpdatePost(ctx context.Context, req *pbCore.UpdatePostRequest) (*pbCore.UpdatePostResponse, error) {
	return &pbCore.UpdatePostResponse{}, nil
}
func (s *PostServiceService) DeletePost(ctx context.Context, req *pbCore.DeletePostRequest) (*pbCore.DeletePostResponse, error) {
	return &pbCore.DeletePostResponse{}, nil
}
