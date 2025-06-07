package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// AuthServiceService 认证服务结构体
// 包含业务用例和日志记录器
type AuthServiceService struct {
	pb.UnimplementedAuthServiceServer
	auc *biz.AuthUsecase
	uuc *biz.UserUsecase
	log *log.Helper
}

// NewAuthServiceService 创建新的认证服务实例
// 参数：uuc 业务用例实例，logger 日志记录器
// 返回值：认证服务实例指针
func NewAuthServiceService(auc *biz.AuthUsecase, uuc *biz.UserUsecase, logger log.Logger) *AuthServiceService {
	return &AuthServiceService{
		auc: auc,
		uuc: uuc,
		log: log.NewHelper(logger),
	}
}

// Login 处理后台登录请求
// 参数：ctx 上下文，req 登录请求
// 返回值：登录响应，错误信息
func (s *AuthServiceService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Name == "" || req.Password == "" {
		s.log.Errorf("用户名或密码为空")
		return nil, pb.ErrorUserIncorrectPassword("用户名或密码为空")
	}

	// 调用业务逻辑层
	resp, err := s.auc.Login(ctx, req.Name, req.Password)
	if err != nil {
		s.log.Errorf("登录失败: %v", err)
		return nil, err
	}

	return &pb.LoginResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}, nil
}

// RefreshToken 处理刷新令牌请求
// 参数：ctx 上下文，req 刷新令牌请求
// 返回值：刷新令牌响应，错误信息
func (s *AuthServiceService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	if req.RefreshToken == "" {
		s.log.Errorf("刷新令牌为空")
		return nil, pb.ErrorAuthTokenNotExist("刷新令牌为空")
	}

	// 调用业务逻辑层
	resp, err := s.auc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		s.log.Errorf("刷新令牌失败: %v", err)
		return nil, err
	}

	return &pb.RefreshTokenResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}, nil
}

// Logout 处理后台登出请求
// 参数：ctx 上下文，req 登出请求
// 返回值：登出响应，错误信息
func (s *AuthServiceService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	acccessToken := ""
	if acccessToken == "" {
		s.log.Errorf("访问令牌为空")
		return nil, pb.ErrorAuthTokenNotExist("刷新令牌为空")
	}
	// 调用业务逻辑层
	if err := s.auc.Logout(ctx, acccessToken); err != nil {
		s.log.Errorf("登出失败: %v", err)
		return nil, err
	}

	return &pb.LogoutResponse{}, nil
}
