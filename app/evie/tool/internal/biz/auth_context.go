// Package biz · auth_context.go
// AuthContext 抽象 + ctx 注入/提取。
//
// 设计动机（M9.5 修复）：
//   - 原 data 包独占 ctxKeyAuthInfo / WithAuthInfo / AuthInfoFromContext。
//   - VocabSyncer 在 biz 层需要从 ctx 取 AuthInfo（用于 qua 调用），但不能 import data（避免反向依赖）。
//   - 解决：把 ctx key 提到 biz 包；data 包适配。
//
// 依赖方向（保持单向）：
//
//	biz.AuthContext（interface + ctx key）
//	  ↑ 实现
//	data.AuthInfo（struct implements biz.AuthContext）
//
// 调用方（service/biz）通过 biz.AuthFrom / biz.WithAuth 统一访问。
// data 包保留同名 AuthInfoFromContext / WithAuthInfo 作为 alias，内部转发到 biz（保留 API 兼容）。
package biz

import "context"

// AuthContext 业务调用需要的最小认证信息接口。
//
// 实现：data.AuthInfo（*AuthInfo 实现了这两个 getter）。
// 设计为 interface 而非 *data.AuthInfo 是为了 biz 不依赖 data 的具体类型。
type AuthContext interface {
	GetAccessToken() string
	GetTenantID() string
}

type ctxKeyAuthInfo struct{}

// CtxKeyAuthInfo 公开类型别名，供外部包（data 中间件、service 测试）使用。
type CtxKeyAuthInfo = ctxKeyAuthInfo

// WithAuth 把 AuthContext 注入 ctx。
//
// 任何实现了 AuthContext 接口的对象都可注入；biz 层不依赖 data 具体类型。
func WithAuth(ctx context.Context, info AuthContext) context.Context {
	if info == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyAuthInfo{}, info)
}

// AuthFrom 从 ctx 取 AuthContext。
//
// 返回 (nil, false) 表示 ctx 中没有 AuthContext（未经过中间件或被 cancel）。
func AuthFrom(ctx context.Context) (AuthContext, bool) {
	v, ok := ctx.Value(ctxKeyAuthInfo{}).(AuthContext)
	return v, ok
}

// CopyAuthContext 从 src ctx 提取 AuthContext 并注入 dst ctx。
//
// 用法：biz 层需要从请求 ctx 复制 auth 到独立 timeout ctx（不被请求 cancel 打断）。
func CopyAuthContext(dst, src context.Context) context.Context {
	if auth, ok := AuthFrom(src); ok {
		return WithAuth(dst, auth)
	}
	return dst
}
