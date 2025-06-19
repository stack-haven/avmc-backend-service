package data

import (
	"context"
	"errors"

	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
)

// AuthSecurity 认证安全
type AuthSecurity struct {
	log *log.Helper
}

// NewAuthSecurity 创建新的认证安全实例
func NewAuthSecurity(logger log.Logger) *AuthSecurity {
	log := log.NewHelper(log.With(logger, "module", "auth/security/init"))
	return &AuthSecurity{log: log}
}

// Name 获取提供者名称
func (p *AuthSecurity) Name() string {
	return "admin auth security"
}

// NewSecurityUserCreator 创建新的认证用户创建器
func (p *AuthSecurity) NewSecurityUserCreator() authn.SecurityUserCreator {
	return func(authClaims *authn.AuthClaims) authn.SecurityUser {
		if authClaims == nil {
			p.log.Error("auth claims creator fail ac == nil")
		}
		return &securityUser{options: SecurityUserOptions{log: p.log, authClaims: authClaims}}
	}
}

// NewAuthenticator 创建新的认证器实例
func (p *AuthSecurity) NewSecurityUser(authClaims *authn.AuthClaims) authn.SecurityUser {
	// 创建认证声明
	user := new(securityUser)
	user.options = SecurityUserOptions{log: p.log, authClaims: authClaims}
	return user
}

var _ authn.SecurityUser = (*securityUser)(nil)

type SecurityUserOptions struct {
	log        *log.Helper
	authClaims *authn.AuthClaims
}

type Option func(*SecurityUserOptions)

func WithLog(log *log.Helper) Option {
	return func(opts *SecurityUserOptions) {
		opts.log = log
	}
}

func WithAuthClaims(authClaims *authn.AuthClaims) Option {
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
	domain string
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
	su.domain = su.options.authClaims.GetDomain()
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

// GetDomain returns the domain of the token.
func (su *securityUser) GetDomain() string {
	return su.domain
}

// GetUserID returns the user id of the token.
func (su *securityUser) GetUserID() uint32 {
	return convert.StringToUnit32(su.subject)
}

// GetDomainID returns the domain id of the token.
func (su *securityUser) GetDomainID() uint32 {
	return convert.StringToUnit32(su.domain)
}
