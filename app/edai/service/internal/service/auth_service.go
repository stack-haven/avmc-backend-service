package service

import (
	"context"
	"time"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/biz"
	"backend-service/pkg/auth/authn"

	"github.com/go-kratos/kratos/v2/log"
)

// AuthServiceService 本地账号认证服务。
type AuthServiceService struct {
	pb.UnimplementedAuthServiceServer
	uc  *biz.AuthUsecase
	log *log.Helper
}

// NewAuthServiceService 创建认证服务。
func NewAuthServiceService(uc *biz.AuthUsecase, logger log.Logger) *AuthServiceService {
	return &AuthServiceService{uc: uc, log: log.NewHelper(log.With(logger, "module", "edai/service/auth"))}
}

// Login 账号密码登录。
func (s *AuthServiceService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	token, user, err := s.uc.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return &pb.LoginResponse{Token: token, User: user}, nil
}

// RefreshToken 刷新访问令牌。
func (s *AuthServiceService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	token, err := s.uc.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return &pb.RefreshTokenResponse{Token: token}, nil
}

// Logout 撤销刷新令牌。
func (s *AuthServiceService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	if err := s.uc.Logout(ctx, req.GetRefreshToken()); err != nil {
		return nil, err
	}
	return &pb.LogoutResponse{}, nil
}

// GetAccessCodes 返回当前用户的权限码。
//
// 权限码按角色静态派生：前端只用于隐藏入口，后端仍逐接口校验。
func (s *AuthServiceService) GetAccessCodes(ctx context.Context, _ *pb.GetAccessCodesRequest) (*pb.GetAccessCodesResponse, error) {
	user, err := s.uc.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetAccessCodesResponse{Codes: s.uc.AccessCodesFor(user.GetRole())}, nil
}

// GetCurrentUser 返回当前登录用户。
func (s *AuthServiceService) GetCurrentUser(ctx context.Context, _ *pb.GetCurrentUserRequest) (*pb.GetCurrentUserResponse, error) {
	user, err := s.uc.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetCurrentUserResponse{User: user}, nil
}

// 编译期确认 authn 包被引用（租户/用户上下文读取在 biz 层完成）。
var _ = authn.GetAuthUserID
var _ = time.Now
