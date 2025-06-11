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
		return NewSecurityUser(logger, authClaims)
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
	subject uint32
	// 资源/路由
	object string
	// 方法
	action string
	// 域/租户
	domain uint32
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
	// su.GetAction()
	// // subject := convert.StringToUnit32(su.options.authClaims["sub"].(string))
	// subject := convert.StringToUnit32("1")
	// authTokenRepo := su.options.authTokenRepo.GetAccessToken(ctx, subject)
	// if authTokenRepo == "" {
	// 	err := errors.New("result auth user fail: auth token null")
	// 	su.options.log.Error(err)
	// 	return err
	// }
	// su.domain = su.options.authClaims
	// su.subject = authTokenRepo.LastUseRoleID
	// authn.ContextWithAuthUser(ctx, su)
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
	return convert.Unit32ToString(su.subject)
}

// GetDomain returns the domain of the token.
func (su *securityUser) GetDomain() string {
	return convert.Unit32ToString(su.domain)
}
