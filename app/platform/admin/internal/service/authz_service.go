package service

import (
	"context"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	corepb "backend-service/api/core/service/v1"
	authzEngine "backend-service/pkg/auth/authz"
)

// AuthzService 授权服务，为产品服务（如 evie）提供跨服务的鉴权委托。
// 它实现了 core.service.v1.AuthService 的 IsAuthorized RPC，
// 将鉴权判断委托给本服务的本地 Casbin 授权器。
type AuthzService struct {
	corepb.UnimplementedAuthServiceServer
	authorizer authzEngine.Authorizer
	log        *log.Helper
}

// NewAuthzService 创建授权服务实例。
func NewAuthzService(authorizer authzEngine.Authorizer, logger log.Logger) *AuthzService {
	return &AuthzService{
		authorizer: authorizer,
		log:        log.NewHelper(logger),
	}
}

// IsAuthorized 执行授权检查。
// 请求字段与 authz 语义的映射（与 pkg/auth/authz/grpc 客户端一致）：
//   - subject  → 主体（用户 ID）
//   - resource → 对象（操作/资源路径）
//   - action   → 操作（HTTP 方法或 gRPC 方法名）
//   - project  → 租户 ID
//
// 返回：无错误表示允许；错误（FORBIDDEN）表示拒绝。
func (s *AuthzService) IsAuthorized(ctx context.Context, req *corepb.IsAuthorizedRequest) (*corepb.IsAuthorizedResponse, error) {
	if req == nil {
		return nil, kerrors.BadRequest("AUTHZ_REQUEST_REQUIRED", "authorization request is required")
	}
	sub := authzEngine.Subject(req.GetSubject())
	obj := authzEngine.Object(req.GetResource())
	act := authzEngine.Action(req.GetAction())
	tenant := authzEngine.Tenant(req.GetProject())
	if sub == "" || obj == "" || act == "" {
		return nil, kerrors.BadRequest("AUTHZ_INFO_INCOMPLETE", "subject, resource and action must not be empty")
	}

	allowed, err := s.authorizer.Enforce(ctx, sub, obj, act, tenant)
	if err != nil {
		s.log.WithContext(ctx).Warnf("authorize enforce failed: sub=%s obj=%s act=%s tenant=%s err=%v", sub, obj, act, tenant, err)
		return nil, kerrors.Forbidden("FORBIDDEN", err.Error())
	}
	if !allowed {
		return nil, kerrors.Forbidden("FORBIDDEN", "permission denied")
	}
	return &corepb.IsAuthorizedResponse{}, nil
}
