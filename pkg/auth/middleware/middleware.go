package middleware

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/authz"
)

// 错误码定义
const (
	// ErrUnauthorized 未授权错误码
	ErrUnauthorized = 401
	// ErrForbidden 禁止访问错误码
	ErrForbidden = 403
)

// 错误信息定义
var (
	// ErrMissingToken 缺少令牌错误
	ErrMissingToken = errors.New(ErrUnauthorized, "UNAUTHORIZED", "missing token")
	// ErrInvalidToken 无效令牌错误
	ErrInvalidToken = errors.New(ErrUnauthorized, "UNAUTHORIZED", "invalid token")
	// ErrExpiredToken 令牌过期错误
	ErrExpiredToken = errors.New(ErrUnauthorized, "UNAUTHORIZED", "token has expired")
	// ErrPermissionDenied 权限被拒绝错误
	ErrPermissionDenied = errors.New(ErrForbidden, "FORBIDDEN", "permission denied")
)

// AuthnMiddleware 创建身份验证中间件
func AuthnMiddleware(authenticator authn.Authenticator) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 执行身份验证
			claims, err := authenticator.Authenticate(ctx)
			if err != nil {
				// 处理身份验证错误
				var authErr *authn.AuthError
				if errors.As(err, &authErr) {
					switch authErr.Code {
					case authn.ErrCodeMissingToken:
						return nil, ErrMissingToken
					case authn.ErrCodeExpiredToken:
						return nil, ErrExpiredToken
					case authn.ErrCodeInvalidToken, authn.ErrCodeInvalidSignature, authn.ErrCodeInvalidClaims:
						return nil, ErrInvalidToken
					default:
						return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", authErr.Error())
					}
				}
				return nil, ErrInvalidToken
			}

			// 将认证声明注入上下文
			ctx = authn.ContextWithAuthClaims(ctx, claims)
			// 将用户信息注入上下文
			securityUser := authenticator.Options().UserFactory(claims)
			if securityUser != nil {
				err := securityUser.ParseFromContext(ctx)
				if err != nil {
					return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", err.Error())
				}
			} else {
				return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", "security user parse fail")
			}
			ctx = authn.ContextWithAuthUser(ctx, securityUser)
			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// AuthzMiddleware 创建身份鉴权中间件
func AuthzMiddleware(authorizer authz.Authorizer) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 从上下文中提取授权信息
			sub, obj, act, dom, ok := authz.ExtractAuthzInfo(ctx)
			if !ok {
				// 如果上下文中没有授权信息，尝试从请求中提取
				if tr, ok := transport.FromServerContext(ctx); ok {
					// 从请求路径和方法中提取授权信息
					path := tr.Operation()
					method := ""
					switch tr.Kind() {
					case transport.KindHTTP:
						method = tr.RequestHeader().Get("X-HTTP-Method")
					case transport.KindGRPC:
						// 从gRPC方法中提取操作
						parts := strings.Split(path, "/")
						if len(parts) > 0 {
							method = parts[len(parts)-1]
						}
					}

					// 从认证声明中提取主体
					claims, ok := authn.AuthClaimsFromContext(ctx)
					if !ok || claims == nil {
						return nil, ErrInvalidToken
					}

					// 设置授权信息
					sub = authz.Subject(claims.GetSubject())
					obj = authz.Object(path)
					act = authz.Action(method)
					dom = authz.Domain(claims.GetIssuer())
				}
			}

			// 执行授权检查
			if sub != "" && obj != "" && act != "" {
				allowed, err := authorizer.Enforce(ctx, sub, obj, act, dom)
				if err != nil {
					// 处理授权错误
					var authzErr *authz.AuthzError
					if errors.As(err, &authzErr) {
						switch authzErr.Code {
						case authz.ErrCodePermissionDenied:
							return nil, ErrPermissionDenied
						default:
							return nil, errors.New(ErrForbidden, "FORBIDDEN", authzErr.Error())
						}
					}
					return nil, ErrPermissionDenied
				}

				if !allowed {
					return nil, ErrPermissionDenied
				}

				// 将授权结果注入上下文
				ctx = authz.ContextWithAuthzResult(ctx, true)
			}

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// CombinedAuthMiddleware 创建组合身份验证和身份鉴权中间件
func CombinedAuthMiddleware(authenticator authn.Authenticator, authorizer authz.Authorizer) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 执行身份验证
			claims, err := authenticator.Authenticate(ctx)
			if err != nil {
				// 处理身份验证错误
				var authErr *authn.AuthError
				if errors.As(err, &authErr) {
					switch authErr.Code {
					case authn.ErrCodeMissingToken:
						return nil, ErrMissingToken
					case authn.ErrCodeExpiredToken:
						return nil, ErrExpiredToken
					case authn.ErrCodeInvalidToken, authn.ErrCodeInvalidSignature, authn.ErrCodeInvalidClaims:
						return nil, ErrInvalidToken
					default:
						return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", authErr.Error())
					}
				}
				return nil, ErrInvalidToken
			}

			// 将认证声明注入上下文
			ctx = authn.ContextWithAuthClaims(ctx, claims)

			// 从请求中提取授权信息
			var sub authz.Subject
			var obj authz.Object
			var act authz.Action
			var dom authz.Domain

			// 从上下文中提取授权信息
			sub, obj, act, dom, ok := authz.ExtractAuthzInfo(ctx)
			if !ok {
				// 如果上下文中没有授权信息，尝试从请求中提取
				if tr, ok := transport.FromServerContext(ctx); ok {
					// 从请求路径和方法中提取授权信息
					path := tr.Operation()
					method := ""
					switch tr.Kind() {
					case transport.KindHTTP:
						method = tr.RequestHeader().Get("X-HTTP-Method")
					case transport.KindGRPC:
						// 从gRPC方法中提取操作
						parts := strings.Split(path, "/")
						if len(parts) > 0 {
							method = parts[len(parts)-1]
						}
					}

					// 设置授权信息
					sub = authz.Subject(claims.GetSubject())
					obj = authz.Object(path)
					act = authz.Action(method)
					dom = authz.Domain(claims.GetIssuer())
				}
			}

			// 执行授权检查
			if sub != "" && obj != "" && act != "" {
				allowed, err := authorizer.Enforce(ctx, sub, obj, act, dom)
				if err != nil {
					// 处理授权错误
					var authzErr *authz.AuthzError
					if errors.As(err, &authzErr) {
						switch authzErr.Code {
						case authz.ErrCodePermissionDenied:
							return nil, ErrPermissionDenied
						default:
							return nil, errors.New(ErrForbidden, "FORBIDDEN", authzErr.Error())
						}
					}
					return nil, ErrPermissionDenied
				}

				if !allowed {
					return nil, ErrPermissionDenied
				}

				// 将授权结果注入上下文
				ctx = authz.ContextWithAuthzResult(ctx, true)
			}

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// SkipAuthPathMiddleware 创建跳过特定路径的身份验证中间件
func SkipAuthPathMiddleware(authenticator authn.Authenticator, skipPaths []string) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 检查是否需要跳过身份验证
			if tr, ok := transport.FromServerContext(ctx); ok {
				path := tr.Operation()
				for _, skipPath := range skipPaths {
					if strings.HasPrefix(path, skipPath) {
						// 跳过身份验证，直接处理请求
						return handler(ctx, req)
					}
				}
			}

			// 执行身份验证
			claims, err := authenticator.Authenticate(ctx)
			if err != nil {
				// 处理身份验证错误
				var authErr *authn.AuthError
				if errors.As(err, &authErr) {
					switch authErr.Code {
					case authn.ErrCodeMissingToken:
						return nil, ErrMissingToken
					case authn.ErrCodeExpiredToken:
						return nil, ErrExpiredToken
					case authn.ErrCodeInvalidToken, authn.ErrCodeInvalidSignature, authn.ErrCodeInvalidClaims:
						return nil, ErrInvalidToken
					default:
						return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", authErr.Error())
					}
				}
				return nil, ErrInvalidToken
			}

			// 将认证声明注入上下文
			ctx = authn.ContextWithAuthClaims(ctx, claims)

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// SkipAuthRoleMiddleware 创建基于角色跳过身份鉴权的中间件
func SkipAuthRoleMiddleware(authorizer authz.Authorizer, skipRoles []string) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 从上下文中提取授权信息
			sub, obj, act, dom, ok := authz.ExtractAuthzInfo(ctx)
			if !ok {
				// 如果上下文中没有授权信息，尝试从请求中提取
				if tr, ok := transport.FromServerContext(ctx); ok {
					// 从请求路径和方法中提取授权信息
					path := tr.Operation()
					method := ""
					switch tr.Kind() {
					case transport.KindHTTP:
						method = tr.RequestHeader().Get("X-HTTP-Method")
					case transport.KindGRPC:
						// 从gRPC方法中提取操作
						parts := strings.Split(path, "/")
						if len(parts) > 0 {
							method = parts[len(parts)-1]
						}
					}

					// 从认证声明中提取主体
					claims, ok := authn.AuthClaimsFromContext(ctx)
					if !ok || claims == nil {
						return nil, ErrInvalidToken
					}

					// 设置授权信息
					sub = authz.Subject(claims.GetSubject())
					obj = authz.Object(path)
					act = authz.Action(method)
					dom = authz.Domain(claims.GetIssuer())
				}
			}

			// 检查用户角色
			if sub != "" && dom != "" {
				roles, err := authorizer.GetRolesForUser(ctx, sub, dom)
				if err == nil {
					for _, role := range roles {
						for _, skipRole := range skipRoles {
							if string(role) == skipRole {
								// 跳过身份鉴权，直接处理请求
								return handler(ctx, req)
							}
						}
					}
				}
			}

			// 执行授权检查
			if sub != "" && obj != "" && act != "" {
				allowed, err := authorizer.Enforce(ctx, sub, obj, act, dom)
				if err != nil {
					// 处理授权错误
					var authzErr *authz.AuthzError
					if errors.As(err, &authzErr) {
						switch authzErr.Code {
						case authz.ErrCodePermissionDenied:
							return nil, ErrPermissionDenied
						default:
							return nil, errors.New(ErrForbidden, "FORBIDDEN", authzErr.Error())
						}
					}
					return nil, ErrPermissionDenied
				}

				if !allowed {
					return nil, ErrPermissionDenied
				}

				// 将授权结果注入上下文
				ctx = authz.ContextWithAuthzResult(ctx, true)
			}

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}
