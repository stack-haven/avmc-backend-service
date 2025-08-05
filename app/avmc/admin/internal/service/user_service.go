package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	pb "backend-service/api/avmc/admin/v1"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// UserServiceService 用户服务结构体
// 包含业务用例和日志记录器
type UserServiceService struct {
	pb.UnimplementedUserServiceServer
	uuc *biz.UserUsecase
	log *log.Helper
}

// NewUserServiceService 创建新的用户服务实例
// 参数：uuc 业务用例实例，logger 日志记录器
// 返回值：用户服务实例指针
func NewUserServiceService(uuc *biz.UserUsecase, logger log.Logger) *UserServiceService {
	return &UserServiceService{
		uuc: uuc,
		log: log.NewHelper(logger),
	}
}

// ListUserSimple 处理用户简单列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：用户列表响应，错误信息
func (s *UserServiceService) ListUserSimple(ctx context.Context, req *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	s.log.Infof("查询用户简单列表分页，分页请求：%v", req)
	return s.uuc.ListPageSimple(ctx, req)
}

// ListUser 处理用户列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：用户列表响应，错误信息
func (s *UserServiceService) ListUser(ctx context.Context, req *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	s.log.Infof("查询用户列表分页，分页请求：%v", req)
	return s.uuc.ListPage(ctx, req)
}

// GetUser 处理获取用户详情请求
// 参数：ctx 上下文，req 获取用户请求
// 返回值：用户详情响应，错误信息
func (s *UserServiceService) GetUser(ctx context.Context, req *pbCore.GetUserRequest) (*pbCore.User, error) {
	if req.Id == 0 {
		return nil, errors.New(1001, "用户ID不能为空", "user id is required")
	}
	s.log.Infof("获取用户详情，用户ID：%v", req.Id)
	return s.uuc.Get(ctx, req.Id)
}

// CreateUser 处理创建用户请求
// 参数：ctx 上下文，req 创建用户请求
// 返回值：创建用户响应，错误信息
func (s *UserServiceService) CreateUser(ctx context.Context, req *pbCore.CreateUserRequest) (*pbCore.CreateUserResponse, error) {
	if req.GetUser() == nil {
		return nil, pb.ErrorUserInvalidId("用户信息不能为空")
	}
	s.log.Infof("创建用户，用户信息：%v", req.User)
	_, err := s.uuc.Create(ctx, req.User)
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateUserResponse{}, nil
}

// UpdateUser 处理更新用户请求
// 参数：ctx 上下文，req 更新用户请求
// 返回值：更新用户响应，错误信息
func (s *UserServiceService) UpdateUser(ctx context.Context, req *pbCore.UpdateUserRequest) (*pbCore.UpdateUserResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorUserInvalidId("用户ID不能为空")
	}
	if req.GetUser() == nil {
		return nil, pb.ErrorUserInvalidId("用户信息不能为空")
	}
	req.User.Id = req.GetId()
	if req.GetOperatorId() == 0 {
		// return nil, pb.ErrorUserInvalidOperatorId("操作人ID不能为空")
	}
	s.log.Infof("更新用户，用户信息：%v", req.GetUser())
	_, err := s.uuc.Update(ctx, req.User)
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateUserResponse{}, nil
}

// DeleteUser 处理删除用户请求
// 参数：ctx 上下文，req 删除用户请求
// 返回值：删除用户响应，错误信息
func (s *UserServiceService) DeleteUser(ctx context.Context, req *pbCore.DeleteUserRequest) (*pbCore.DeleteUserResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorUserInvalidId("用户ID不能为空")
	}
	s.log.Infof("删除用户，用户ID：%v", req.GetId())
	err := s.uuc.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.DeleteUserResponse{}, nil
}
