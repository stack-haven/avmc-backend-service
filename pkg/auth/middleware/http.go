package middleware

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"

	"backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/authz"
)

// HTTPAuthExtractor 定义HTTP请求中提取认证信息的函数类型
type HTTPAuthExtractor func(ctx context.Context, req *http.Request) (string, error)

// DefaultHTTPAuthExtractor 默认的HTTP认证信息提取器
// 从Authorization头或cookie中提取令牌
func DefaultHTTPAuthExtractor(ctx context.Context, req *http.Request) (string, error) {
	// 从Authorization头中提取
	auth := req.Header.Get("Authorization")
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			scheme := strings.ToLower(parts[0])
			if scheme == "bearer" || scheme == "token" {
				return parts[1], nil
			}
		}
	}

	// 从Cookie中提取
	cookie, err := req.Cookie("token")
	if err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	// 从查询参数中提取
	token := req.URL.Query().Get("token")
	if token != "" {
		return token, nil
	}

	return "", authn.NewAuthError(authn.ErrCodeMissingToken, "token not found in request", nil)
}

// HTTPAuthzInfoExtractor 定义HTTP请求中提取授权信息的函数类型
type HTTPAuthzInfoExtractor func(ctx context.Context, req *http.Request) (authz.Subject, authz.Object, authz.Action, authz.Domain, error)

// DefaultHTTPAuthzInfoExtractor 默认的HTTP授权信息提取器
// 从请求路径、方法和认证声明中提取授权信息
func DefaultHTTPAuthzInfoExtractor(ctx context.Context, req *http.Request) (authz.Subject, authz.Object, authz.Action, authz.Domain, error) {
	// 从认证声明中提取主体和域
	claims, ok := authn.AuthClaimsFromContext(ctx)
	if !ok || claims == nil {
		return "", "", "", "", authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid or missing auth claims", nil)
	}

	// 提取主体和域
	sub := authz.Subject(claims.GetSubject())
	dom := authz.Domain(claims.GetIssuer())

	// 从请求中提取对象和操作
	obj := authz.Object(req.URL.Path)
	act := authz.Action(req.Method)

	return sub, obj, act, dom, nil
}

// HTTPAuthnMiddleware 创建HTTP身份验证中间件
func HTTPAuthnMiddleware(authenticator authn.Authenticator, extractor HTTPAuthExtractor) middleware.Middleware {
	if extractor == nil {
		extractor = DefaultHTTPAuthExtractor
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if tr.Kind() == transport.KindHTTP {
					httpCtx, ok := ctx.(http.Context)
					if ok {
						httpReq := httpCtx.Request()
						// 提取令牌
						token, err := extractor(ctx, httpReq)
						if err != nil {
							// 处理提取错误
							var authErr *authn.AuthError
							if errors.As(err, &authErr) {
								switch authErr.Code {
								case authn.ErrCodeMissingToken:
									return nil, ErrMissingToken
								default:
									return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", authErr.Error())
								}
							}
							return nil, ErrMissingToken
						}

						// 验证令牌
						claims, err := authenticator.ValidateToken(ctx, token)
						if err != nil {
							// 处理验证错误
							var authErr *authn.AuthError
							if errors.As(err, &authErr) {
								switch authErr.Code {
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
					}
				}
			}

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// HTTPAuthzMiddleware 创建HTTP身份鉴权中间件
func HTTPAuthzMiddleware(authorizer authz.Authorizer, extractor HTTPAuthzInfoExtractor) middleware.Middleware {
	if extractor == nil {
		extractor = DefaultHTTPAuthzInfoExtractor
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if tr.Kind() == transport.KindHTTP {

					httpCtx, ok := ctx.(http.Context)
					if ok {
						httpReq := httpCtx.Request()
						// 提取授权信息
						sub, obj, act, dom, err := extractor(ctx, httpReq)
						if err != nil {
							// 处理提取错误
							var authErr *authn.AuthError
							if errors.As(err, &authErr) {
								switch authErr.Code {
								case authn.ErrCodeInvalidToken:
									return nil, ErrInvalidToken
								default:
									return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", authErr.Error())
								}
							}
							return nil, ErrInvalidToken
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
					}
				}
			}

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// HTTPCombinedAuthMiddleware 创建HTTP组合身份验证和身份鉴权中间件
func HTTPCombinedAuthMiddleware(
	authenticator authn.Authenticator,
	authorizer authz.Authorizer,
	authExtractor HTTPAuthExtractor,
	authzExtractor HTTPAuthzInfoExtractor,
) middleware.Middleware {
	if authExtractor == nil {
		authExtractor = DefaultHTTPAuthExtractor
	}
	if authzExtractor == nil {
		authzExtractor = DefaultHTTPAuthzInfoExtractor
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if tr.Kind() == transport.KindHTTP {
					httpCtx, ok := ctx.(http.Context)
					if ok {
						httpReq := httpCtx.Request()
						// 提取令牌
						token, err := authExtractor(ctx, httpReq)
						if err != nil {
							// 处理提取错误
							var authErr *authn.AuthError
							if errors.As(err, &authErr) {
								switch authErr.Code {
								case authn.ErrCodeMissingToken:
									return nil, ErrMissingToken
								default:
									return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", authErr.Error())
								}
							}
							return nil, ErrMissingToken
						}

						// 验证令牌
						claims, err := authenticator.ValidateToken(ctx, token)
						if err != nil {
							// 处理验证错误
							var authErr *authn.AuthError
							if errors.As(err, &authErr) {
								switch authErr.Code {
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

						// 提取授权信息
						sub, obj, act, dom, err := authzExtractor(ctx, httpReq)
						if err != nil {
							// 处理提取错误
							var authErr *authn.AuthError
							if errors.As(err, &authErr) {
								switch authErr.Code {
								case authn.ErrCodeInvalidToken:
									return nil, ErrInvalidToken
								default:
									return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", authErr.Error())
								}
							}
							return nil, ErrInvalidToken
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
					}
				}
			}

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// HTTPSkipAuthPathMiddleware 创建跳过特定路径的HTTP身份验证中间件
func HTTPSkipAuthPathMiddleware(
	authenticator authn.Authenticator,
	skipPaths []string,
	extractor HTTPAuthExtractor,
) middleware.Middleware {
	if extractor == nil {
		extractor = DefaultHTTPAuthExtractor
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if tr.Kind() == transport.KindHTTP {
					httpCtx, ok := ctx.(http.Context)
					if ok {
						httpReq := httpCtx.Request()
						// 检查是否需要跳过身份验证
						for _, skipPath := range skipPaths {
							if strings.HasPrefix(httpReq.URL.Path, skipPath) {
								// 跳过身份验证，直接处理请求
								return handler(ctx, req)
							}
						}

						// 提取令牌
						token, err := extractor(ctx, httpReq)
						if err != nil {
							// 处理提取错误
							var authErr *authn.AuthError
							if errors.As(err, &authErr) {
								switch authErr.Code {
								case authn.ErrCodeMissingToken:
									return nil, ErrMissingToken
								default:
									return nil, errors.New(ErrUnauthorized, "UNAUTHORIZED", authErr.Error())
								}
							}
							return nil, ErrMissingToken
						}

						// 验证令牌
						claims, err := authenticator.ValidateToken(ctx, token)
						if err != nil {
							// 处理验证错误
							var authErr *authn.AuthError
							if errors.As(err, &authErr) {
								switch authErr.Code {
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
					}
				}
			}

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}
