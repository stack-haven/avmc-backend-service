package data

import (
	"context"
	"errors"

	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
)

var _ authn.SecurityUser = (*securityUser)(nil)

func NewSecurityUserCreator(logger log.Logger) authn.SecurityUserCreator {
	log := log.NewHelper(log.With(logger, "module", "auth/securityUserCreator"))
	return func(authClaims *authn.AuthClaims) authn.SecurityUser {
		if authClaims == nil {
			log.Error("auth claims creator fail ac == nil")
		}
		return &securityUser{options: securityOptions{log: log, authClaims: authClaims}}
	}
}

func NewSecurityUser(logger log.Logger, authClaims *authn.AuthClaims) authn.SecurityUser {
	log := log.NewHelper(log.With(logger, "module", "auth/securityUser"))
	return &securityUser{options: securityOptions{log: log, authClaims: authClaims}}
}

type securityOptions struct {
	log        *log.Helper
	authClaims *authn.AuthClaims
}

type securityUser struct {
	options securityOptions
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
	return "admin security"
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
