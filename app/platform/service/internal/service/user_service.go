package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/filtering"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/pkg/aip/listing"
)

// UserServiceService 用户服务
type UserServiceService struct {
	pb.UnimplementedUserServiceServer
	uuc *biz.UserUsecase
	log *log.Helper
}

// NewUserServiceService 创建新的用户服务实例
func NewUserServiceService(uuc *biz.UserUsecase, logger log.Logger) *UserServiceService {
	return &UserServiceService{
		uuc: uuc,
		log: log.NewHelper(logger),
	}
}

// ListUsersSimple 用户简单列表
func (s *UserServiceService) ListUsersSimple(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	s.log.Infof("查询用户简单列表，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	params, err := parseUserListParams(req)
	if err != nil {
		return nil, err
	}
	req.PageSize = int32(params.PageSize)
	count, err := s.uuc.CountUsers(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := pb.ListUsersResponse{Total: count}
	resp.Items, err = s.uuc.ListPageSimple(ctx,
		listing.FilterOption(params.Filter), listing.OrderByOption(params.OrderBy),
		listing.LimitOption(params.PageSize), listing.OffsetOption(int(params.PageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= params.PageSize {
		resp.NextPageToken = params.PageToken.Next(req).String()
	}
	return &resp, nil
}

func parseUserListParams(req *pb.ListUsersRequest) (listing.Params, error) {
	return listing.ParseParams(
		req,
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("email", filtering.TypeString),
		filtering.DeclareIdent("phone", filtering.TypeString),
		filtering.DeclareIdent("status", filtering.TypeInt),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
}

// GetUser 获取用户详情
func (s *UserServiceService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	s.log.Infof("获取用户详情 ID: %d", req.GetId())
	return s.uuc.Get(ctx, req.GetId())
}

// CreateUser 创建用户
func (s *UserServiceService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	s.log.Infof("创建用户")
	_, err := s.uuc.Create(ctx, req.GetUser())
	if err != nil {
		return nil, err
	}
	return &pb.CreateUserResponse{}, nil
}

// UpdateUser 更新用户
func (s *UserServiceService) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	existing, err := s.GetUser(ctx, &pb.GetUserRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	fieldmask.Update(req.UpdateMask, existing, req.User)
	s.log.Infof("更新用户 ID: %d", req.GetId())

	existing.Id = req.GetId()
	_, err = s.uuc.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateUserResponse{}, nil
}

// DeleteUser 删除用户
func (s *UserServiceService) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	s.log.Infof("删除用户 ID: %d", req.GetId())
	if err := s.uuc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteUserResponse{}, nil
}

// UpdateUserByStatus 更新用户状态
func (s *UserServiceService) UpdateUserByStatus(ctx context.Context, req *pb.UpdateUserByStatusRequest) (*pb.UpdateUserByStatusResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorUserInvalidId("用户ID不能为空")
	}
	if req.GetStatus() == 0 {
		return nil, pb.ErrorUserStatusCannotBeEmpty("用户状态不能为空")
	}
	s.log.Infof("更新用户状态 ID: %d, status: %d", req.GetId(), req.GetStatus())
	_, err := s.uuc.UpdateStatus(ctx, req.GetId(), int32(req.GetStatus()))
	if err != nil {
		return nil, err
	}
	return &pb.UpdateUserByStatusResponse{}, nil
}

// ListUsers 用户完整列表
func (s *UserServiceService) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	s.log.Infof("查询用户列表，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	params, err := parseUserListParams(req)
	if err != nil {
		return nil, err
	}
	req.PageSize = int32(params.PageSize)
	count, err := s.uuc.CountUsersByDept(ctx, req.GetDeptId(), req.GetIncludeChildDepts(), listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := pb.ListUsersResponse{Total: count}
	resp.Items, err = s.uuc.ListUsersByDept(ctx, req.GetDeptId(), req.GetIncludeChildDepts(),
		listing.FilterOption(params.Filter), listing.OrderByOption(params.OrderBy),
		listing.LimitOption(params.PageSize), listing.OffsetOption(int(params.PageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= params.PageSize {
		resp.NextPageToken = params.PageToken.Next(req).String()
	}
	return &resp, nil
}
