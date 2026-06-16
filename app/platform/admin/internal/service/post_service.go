package service

import (
	"context"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
)

// PostServiceService 岗位服务结构体
// 包含业务用例和日志记录器
type PostServiceService struct {
	pb.UnimplementedPostServiceServer
	puc *biz.PostUsecase
	log *log.Helper
}

// NewPostServiceService 创建新的岗位服务实例
// 参数：puc 岗位业务用例实例，logger 日志记录器
// 返回值：岗位服务实例指针
func NewPostServiceService(puc *biz.PostUsecase, logger log.Logger) *PostServiceService {
	return &PostServiceService{
		puc: puc,
		log: log.NewHelper(logger),
	}
}

// ListPost 处理岗位列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：岗位列表响应，错误信息
func (s *PostServiceService) ListPosts(ctx context.Context, req *pbCore.ListPostsRequest) (*pbCore.ListPostsResponse, error) {
	s.log.Infof("查询岗位列表分页，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	params, err := listing.ParseParams(
		req,
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	req.PageSize = int32(params.PageSize)
	count, err := s.puc.CountPosts(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := pbCore.ListPostsResponse{
		Total: count,
	}
	resp.Items, err = s.puc.ListPosts(ctx,
		listing.FilterOption(params.Filter),
		listing.OrderByOption(params.OrderBy),
		listing.LimitOption(params.PageSize),
		listing.OffsetOption(int(params.PageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= params.PageSize {
		resp.NextPageToken = params.PageToken.Next(req).String()
	}
	return &resp, nil
}

// GetPost 处理获取岗位详情请求
// 参数：ctx 上下文，req 获取岗位请求
// 返回值：岗位详情，错误信息
func (s *PostServiceService) GetPost(ctx context.Context, req *pbCore.GetPostRequest) (*pbCore.Post, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorPostInvalidId("岗位ID不能为空")
	}
	s.log.Infof("获取岗位详情，岗位ID：%v", req.GetId())
	return s.puc.Get(ctx, req.GetId())
}

// CreatePost 处理创建岗位请求
// 参数：ctx 上下文，req 创建岗位请求
// 返回值：创建岗位响应，错误信息
func (s *PostServiceService) CreatePost(ctx context.Context, req *pbCore.CreatePostRequest) (*pbCore.CreatePostResponse, error) {
	if req.GetPost() == nil {
		return nil, pb.ErrorPostInvalidId("岗位信息不能为空")
	}
	s.log.Infof("创建岗位，岗位名称：%s", req.GetPost().GetName())
	_, err := s.puc.Create(ctx, req.Post)
	if err != nil {
		return nil, err
	}
	return &pbCore.CreatePostResponse{}, nil
}

// UpdatePost 处理更新岗位请求
// 参数：ctx 上下文，req 更新岗位请求
// 返回值：更新岗位响应，错误信息
func (s *PostServiceService) UpdatePost(ctx context.Context, req *pbCore.UpdatePostRequest) (*pbCore.UpdatePostResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorPostInvalidId("岗位ID不能为空")
	}
	if req.GetPost() == nil {
		return nil, pb.ErrorPostInvalidId("岗位信息不能为空")
	}
	s.log.Infof("更新岗位，岗位ID：%v", req.GetId())
	req.Post.Id = req.GetId()
	_, err := s.puc.Update(ctx, req.GetPost())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdatePostResponse{}, nil
}

// DeletePost 处理删除岗位请求
// 参数：ctx 上下文，req 删除岗位请求
// 返回值：删除岗位响应，错误信息
func (s *PostServiceService) DeletePost(ctx context.Context, req *pbCore.DeletePostRequest) (*pbCore.DeletePostResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorPostInvalidId("岗位ID不能为空")
	}
	s.log.Infof("删除岗位，岗位ID：%v", req.GetId())
	err := s.puc.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.DeletePostResponse{}, nil
}
