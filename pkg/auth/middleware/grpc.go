package middleware

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc/metadata"

	"backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/authz"
)

// GRPCAuthExtractor 定义gRPC请求中提取认证信息的函数类型
type GRPCAuthExtractor func(ctx context.Context) (string, error)

// DefaultGRPCAuthExtractor 默认的gRPC认证信息提取器
// 从metadata中提取令牌
func DefaultGRPCAuthExtractor(ctx context.Context) (string, error) {
	// 从metadata中提取
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", authn.NewAuthError(authn.ErrCodeMissingToken, "no metadata in context", nil)
	}

	// 尝试从authorization头中提取
	values := md.Get("authorization")
	if len(values) > 0 {
		auth := values[0]
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			scheme := strings.ToLower(parts[0])
			if scheme == "bearer" || scheme == "token" {
				return parts[1], nil
			}
		}
	}

	// 尝试从token头中提取
	values = md.Get("token")
	if len(values) > 0 && values[0] != "" {
		return values[0], nil
	}

	// 尝试从x-token头中提取
	values = md.Get("x-token")
	if len(values) > 0 && values[0] != "" {
		return values[0], nil
	}

	return "", authn.NewAuthError(authn.ErrCodeMissingToken, "token not found in metadata", nil)
}

// GRPCAuthzInfoExtractor 定义gRPC请求中提取授权信息的函数类型
type GRPCAuthzInfoExtractor func(ctx context.Context, fullMethod string) (authz.Subject, authz.Object, authz.Action, authz.Domain, error)

// DefaultGRPCAuthzInfoExtractor 默认的gRPC授权信息提取器
// 从请求方法和认证声明中提取授权信息
func DefaultGRPCAuthzInfoExtractor(ctx context.Context, fullMethod string) (authz.Subject, authz.Object, authz.Action, authz.Domain, error) {
	// 从认证声明中提取主体和域
	claims, ok := authn.AuthClaimsFromContext(ctx)
	if !ok || claims == nil {
		return "", "", "", "", authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid or missing auth claims", nil)
	}

	// 提取主体和域
	sub := authz.Subject(claims.GetSubject())
	dom := authz.Domain(claims.GetIssuer())

	// 从请求方法中提取对象和操作
	obj := authz.Object(fullMethod)

	// 从方法名中提取操作
	parts := strings.Split(fullMethod, "/")
	act := authz.Action("")
	if len(parts) > 0 {
		act = authz.Action(parts[len(parts)-1])
	}

	return sub, obj, act, dom, nil
}

// GRPCAuthnMiddleware 创建gRPC身份验证中间件
func GRPCAuthnMiddleware(authenticator authn.Authenticator, extractor GRPCAuthExtractor) middleware.Middleware {
	if extractor == nil {
		extractor = DefaultGRPCAuthExtractor
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if tr.Kind() == transport.KindGRPC {
					// 提取令牌
					token, err := extractor(ctx)
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

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// GRPCAuthzMiddleware 创建gRPC身份鉴权中间件
func GRPCAuthzMiddleware(authorizer authz.Authorizer, extractor GRPCAuthzInfoExtractor) middleware.Middleware {
	if extractor == nil {
		extractor = DefaultGRPCAuthzInfoExtractor
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if tr.Kind() == transport.KindGRPC {
					// 获取完整方法名
					fullMethod := tr.Operation()

					// 提取授权信息
					sub, obj, act, dom, err := extractor(ctx, fullMethod)
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

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// GRPCCombinedAuthMiddleware 创建gRPC组合身份验证和身份鉴权中间件
func GRPCCombinedAuthMiddleware(
	authenticator authn.Authenticator,
	authorizer authz.Authorizer,
	authExtractor GRPCAuthExtractor,
	authzExtractor GRPCAuthzInfoExtractor,
) middleware.Middleware {
	if authExtractor == nil {
		authExtractor = DefaultGRPCAuthExtractor
	}
	if authzExtractor == nil {
		authzExtractor = DefaultGRPCAuthzInfoExtractor
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if tr.Kind() == transport.KindGRPC {
					// 提取令牌
					token, err := authExtractor(ctx)
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

					// 获取完整方法名
					fullMethod := tr.Operation()

					// 提取授权信息
					sub, obj, act, dom, err := authzExtractor(ctx, fullMethod)
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

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}

// GRPCSkipAuthMethodMiddleware 创建跳过特定方法的gRPC身份验证中间件
func GRPCSkipAuthMethodMiddleware(
	authenticator authn.Authenticator,
	skipMethods []string,
	extractor GRPCAuthExtractor,
) middleware.Middleware {
	if extractor == nil {
		extractor = DefaultGRPCAuthExtractor
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if tr.Kind() == transport.KindGRPC {
					// 获取完整方法名
					fullMethod := tr.Operation()

					// 检查是否需要跳过身份验证
					for _, skipMethod := range skipMethods {
						if strings.HasSuffix(fullMethod, skipMethod) || fullMethod == skipMethod {
							// 跳过身份验证，直接处理请求
							return handler(ctx, req)
						}
					}

					// 提取令牌
					token, err := extractor(ctx)
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

			// 继续处理请求
			return handler(ctx, req)
		}
	}
}
