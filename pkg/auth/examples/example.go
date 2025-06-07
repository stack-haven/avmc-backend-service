package examples

import (
	"context"
	netHttp "net/http"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	"backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/authn/jwt"
	"backend-service/pkg/auth/authz"
	"backend-service/pkg/auth/authz/casbin"
	authModdleware "backend-service/pkg/auth/middleware"
)

// 示例：创建JWT认证器
func createJWTAuthenticator() authn.Authenticator {
	// 配置JWT认证选项
	options := []authn.Option{
		authn.WithIssuer("example-service"),
		authn.WithAudience("example-clients"),
		authn.WithTokenExpiration(24 * time.Hour),
		authn.WithSigningMethod("HS256"),
		authn.WithSigningKey([]byte("your-secret-key")),
	}
	// 创建JWT认证提供者
	provider := jwt.NewProvider()
	// 创建JWT认证器
	authenticator, _ := provider.NewAuthenticator(context.Background(), options...)

	return authenticator
}

// 示例：创建Casbin授权器
func createCasbinAuthorizer() authz.Authorizer {
	mod := `
	[request_definition]
	r = sub, obj, act, dom
	
	[policy_definition]
	p = sub, obj, act, dom
	
	[role_definition]
	g = _, _, _
	
	[policy_effect]
	e = some(where (p.eft == allow))
	
	[matchers]
	m = g(r.sub, p.sub, r.dom) && r.obj == p.obj && r.act == p.act && r.dom == p.dom
	`
	// 配置Casbin授权选项
	options := []authz.Option{
		authz.WithEngineType(authz.EngineCasbin),
		authz.WithModelFormat(authz.ModelFormatFile),
		authz.WithModelText(mod),
		authz.WithAdapterType(authz.AdapterMySQL),
		authz.WithAdapterDSN("root:123456@tcp(localhost:3306)/avmc"),
	}

	// 创建Casbin授权提供者
	provider := casbin.NewProvider()
	// 创建Casbin授权器
	authorizer, _ := provider.NewAuthorizer(context.Background(), options...)

	// 添加策略
	ctx := context.Background()
	authorizer.AddPolicy(ctx, authz.Policy{Subject: "user1", Object: "/api/resource", Action: "GET", Domain: "example-service", Effect: "allow"})
	authorizer.AddRoleForUser(ctx, "user2", "admin", "example-service")
	authorizer.AddPolicy(ctx, authz.Policy{Subject: "admin", Object: "/api/admin", Action: "*", Domain: "example-service", Effect: "allow"})

	return authorizer
}

// 示例：配置HTTP服务器中间件
func configureHTTPServer() *http.Server {
	// 创建认证器和授权器
	authenticator := createJWTAuthenticator()
	authorizer := createCasbinAuthorizer()

	// 创建HTTP服务器
	srv := http.NewServer(
		http.Address(":8000"),
		http.Middleware(
			// 使用HTTP身份验证中间件
			authModdleware.HTTPAuthnMiddleware(authenticator, nil),
			// 使用HTTP身份鉴权中间件
			authModdleware.HTTPAuthzMiddleware(authorizer, nil),
		),
	)

	// 注册路由
	srv.HandleFunc("/api/public", func(w netHttp.ResponseWriter, r *netHttp.Request) {
		w.Write([]byte("Public API"))
	})

	// 注册需要认证的路由
	srv.HandleFunc("/api/resource", func(w netHttp.ResponseWriter, r *netHttp.Request) {
		ctx := r.Context()
		// 从上下文中获取认证声明
		claims, ok := authn.AuthClaimsFromContext(ctx)
		if !ok || claims == nil {
			w.WriteHeader(401)
			w.Write([]byte("Unauthorized"))
		}
		w.WriteHeader(200)
		w.Write([]byte("Protected Resource"))
	})

	// 注册需要管理员权限的路由

	srv.HandleFunc("/api/admin", func(w netHttp.ResponseWriter, r *netHttp.Request) {
		ctx := r.Context()
		// 从上下文中获取授权结果
		allowed, ok := authz.AuthzResultFromContext(ctx)
		if !ok || !allowed {
			w.WriteHeader(403)
			w.Write([]byte("Forbidden"))
		}

		w.WriteHeader(200)
		w.Write([]byte("Admin Resource"))
	})

	return srv
}

// 示例：配置gRPC服务器中间件
func configureGRPCServer() *grpc.Server {
	// 创建认证器和授权器
	authenticator := createJWTAuthenticator()
	authorizer := createCasbinAuthorizer()

	// 创建gRPC服务器
	srv := grpc.NewServer(
		grpc.Address(":9000"),
		grpc.Middleware(
			// 使用gRPC身份验证中间件
			authModdleware.GRPCAuthnMiddleware(authenticator, nil),
			// 使用gRPC身份鉴权中间件
			authModdleware.GRPCAuthzMiddleware(authorizer, nil),
		),
	)

	return srv
}

// 示例：使用跳过特定路径的中间件
func configureHTTPServerWithSkipPaths() *http.Server {
	// 创建认证器和授权器
	authenticator := createJWTAuthenticator()
	authorizer := createCasbinAuthorizer()

	// 定义跳过身份验证的路径
	skipPaths := []string{
		"/api/public",
		"/api/health",
		"/api/metrics",
	}

	// 创建HTTP服务器
	srv := http.NewServer(
		http.Address(":8000"),
		http.Middleware(
			// 使用跳过特定路径的HTTP身份验证中间件
			authModdleware.HTTPSkipAuthPathMiddleware(authenticator, skipPaths, nil),
			// 使用HTTP身份鉴权中间件
			authModdleware.HTTPAuthzMiddleware(authorizer, nil),
		),
	)

	return srv
}

// 示例：使用组合中间件
func configureHTTPServerWithCombinedMiddleware() *http.Server {
	// 创建认证器和授权器
	authenticator := createJWTAuthenticator()
	authorizer := createCasbinAuthorizer()

	// 创建HTTP服务器
	srv := http.NewServer(
		http.Address(":8000"),
		http.Middleware(
			// 使用组合身份验证和身份鉴权中间件
			authModdleware.HTTPCombinedAuthMiddleware(authenticator, authorizer, nil, nil),
		),
	)

	return srv
}

// 示例：使用自定义提取器
func configureHTTPServerWithCustomExtractors() *http.Server {
	// 创建认证器和授权器
	authenticator := createJWTAuthenticator()
	authorizer := createCasbinAuthorizer()

	// 自定义HTTP认证信息提取器
	customAuthExtractor := func(ctx context.Context, req *http.Request) (string, error) {
		// 从自定义头中提取令牌
		token := req.Header.Get("X-Custom-Token")
		if token != "" {
			return token, nil
		}

		return "", authn.NewAuthError(authn.ErrCodeMissingToken, "token not found in custom header", nil)
	}

	// 自定义HTTP授权信息提取器
	customAuthzExtractor := func(ctx context.Context, req *http.Request) (authz.Subject, authz.Object, authz.Action, authz.Domain, error) {
		// 从认证声明中提取主体和域
		claims, ok := authn.AuthClaimsFromContext(ctx)
		if !ok || claims == nil {
			return "", "", "", "", authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid or missing auth claims", nil)
		}

		// 提取主体和域
		sub := authz.Subject(claims.GetSubject())
		dom := authz.Domain(claims.GetIssuer())

		// 从自定义头中提取对象和操作
		obj := authz.Object(req.Header.Get("X-Resource"))
		act := authz.Action(req.Header.Get("X-Action"))

		return sub, obj, act, dom, nil
	}

	// 创建HTTP服务器
	srv := http.NewServer(
		http.Address(":8000"),
		http.Middleware(
			// 使用自定义提取器的HTTP身份验证中间件
			authModdleware.HTTPAuthnMiddleware(authenticator, customAuthExtractor),
			// 使用自定义提取器的HTTP身份鉴权中间件
			authModdleware.HTTPAuthzMiddleware(authorizer, customAuthzExtractor),
		),
	)

	return srv
}

// 示例：使用Wire进行依赖注入
// 注意：这里只是示例代码，实际使用时需要配合Wire工具生成代码

// ProvideAuthenticator 提供认证器实例
func ProvideAuthenticator() authn.Authenticator {
	return createJWTAuthenticator()
}

// ProvideAuthorizer 提供授权器实例
func ProvideAuthorizer() authz.Authorizer {
	return createCasbinAuthorizer()
}

// ProvideHTTPMiddleware 提供HTTP中间件
func ProvideHTTPMiddleware(authenticator authn.Authenticator, authorizer authz.Authorizer) []middleware.Middleware {
	return []middleware.Middleware{
		authModdleware.HTTPAuthnMiddleware(authenticator, nil),
		authModdleware.HTTPAuthzMiddleware(authorizer, nil),
	}
}

// ProvideGRPCMiddleware 提供gRPC中间件
func ProvideGRPCMiddleware(authenticator authn.Authenticator, authorizer authz.Authorizer) []middleware.Middleware {
	return []middleware.Middleware{
		authModdleware.GRPCAuthnMiddleware(authenticator, nil),
		authModdleware.GRPCAuthzMiddleware(authorizer, nil),
	}
}
