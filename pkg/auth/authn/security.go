package authn

import (
	"context"
	"errors"

	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
)

var _ SecurityUser = (*securityUser)(nil)

type SecurityUserOptions struct {
	log        *log.Helper
	authClaims *AuthClaims
}

type SecurityOption func(*SecurityUserOptions)

func WithLog(log *log.Helper) SecurityOption {
	return func(opts *SecurityUserOptions) {
		opts.log = log
	}
}

func WithAuthClaims(authClaims *AuthClaims) SecurityOption {
	return func(opts *SecurityUserOptions) {
		opts.authClaims = authClaims
	}
}

type securityUser struct {
	options SecurityUserOptions
	// 角色/主题
	subject string
	// 资源/路由
	object string
	// 方法
	action string
	// 域/租户
	tenant string
}

// GetID returns the security Name.
func (su *securityUser) Name() string {
	return "Admin Security User"
}

// ParseFromContext parses the user from the context.
func (su *securityUser) ParseFromContext(ctx context.Context) error {
	if header, ok := transport.FromServerContext(ctx); ok {
		su.object = header.Operation()
		su.action = "*"
		// if header.Kind() == transport.KindHTTP {
		// 	if ht, ok := header.(http.Transporter); ok {
		// 		su.object = ht.Request().URL.Object
		// 		su.action = ht.Request().Action
		// 	}
		// }
	} else {
		return errors.New("parse from request header")
	}

	if su.options.authClaims == nil {
		su.options.log.Error("auth claims creator fail ac == nil")
	}
	su.subject = su.options.authClaims.GetSubject()
	if su.subject == "" {
		return errors.New("subject is empty")
	}
	su.tenant = su.options.authClaims.GetTenant()
	return nil
}

// GetObject returns the object of the token.
func (su *securityUser) GetObject() string {
	return su.object
}

// GetAction returns the action of the token.
func (su *securityUser) GetAction() string {
	return su.action
}

// GetSubject returns the subject of the token.
func (su *securityUser) GetSubject() string {
	return su.subject
}

// GetTenant returns the tenant of the token.
func (su *securityUser) GetTenant() string {
	return su.tenant
}

// GetUserID returns the user id of the token.
func (su *securityUser) GetUserID() uint32 {
	return convert.StringToUnit32(su.subject)
}

// GetTenantID returns the tenant id of the token.
func (su *securityUser) GetTenantID() uint32 {
	return convert.StringToUnit32(su.tenant)
}

// Security 认证安全
type Security struct {
	log *log.Helper
}

// NewSecurity 创建新的认证安全实例
func NewSecurity(logger log.Logger) *Security {
	log := log.NewHelper(log.With(logger, "module", "auth/security/init"))
	return &Security{log: log}
}

// NewSecurityUserCreator 创建新的认证用户创建器
func (p *Security) NewSecurityUserCreator() SecurityUserCreator {
	return func(authClaims *AuthClaims) SecurityUser {
		if authClaims == nil {
			p.log.Error("auth claims creator fail ac == nil")
		}
		return &securityUser{options: SecurityUserOptions{log: p.log, authClaims: authClaims}}
	}
}

// NewAuthenticator 创建新的认证器实例
func (p *Security) NewSecurityUser(authClaims *AuthClaims) SecurityUser {
	// 创建认证声明
	user := new(securityUser)
	user.options = SecurityUserOptions{log: p.log, authClaims: authClaims}
	return user
}

// Name 获取提供者名称
func (p *Security) Name() string {
	return "auth security"
}
