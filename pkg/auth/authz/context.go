package authz

import (
	"context"
)

// 上下文键类型
type ctxKey string

// 上下文键常量
var (
	// authzSubjectContextKey 授权主体上下文键
	authzSubjectContextKey = ctxKey("authz-subject")
	// authzObjectContextKey 授权对象上下文键
	authzObjectContextKey = ctxKey("authz-object")
	// authzActionContextKey 授权操作上下文键
	authzActionContextKey = ctxKey("authz-action")
	// authzTenantContextKey 授权租户上下文键
	authzTenantContextKey = ctxKey("authz-tenant")
	// authzResultContextKey 授权结果上下文键
	authzResultContextKey = ctxKey("authz-result")
)

// ContextWithSubject 将授权主体注入上下文
// parent: 父上下文
// subject: 授权主体
// 返回: 新的上下文
func ContextWithSubject(parent context.Context, subject Subject) context.Context {
	return context.WithValue(parent, authzSubjectContextKey, subject)
}

// SubjectFromContext 从上下文中提取授权主体
// ctx: 上下文
// 返回: 授权主体和是否存在的标志
func SubjectFromContext(ctx context.Context) (Subject, bool) {
	subject, ok := ctx.Value(authzSubjectContextKey).(Subject)
	return subject, ok
}

// ContextWithObject 将授权对象注入上下文
// parent: 父上下文
// object: 授权对象
// 返回: 新的上下文
func ContextWithObject(parent context.Context, object Object) context.Context {
	return context.WithValue(parent, authzObjectContextKey, object)
}

// ObjectFromContext 从上下文中提取授权对象
// ctx: 上下文
// 返回: 授权对象和是否存在的标志
func ObjectFromContext(ctx context.Context) (Object, bool) {
	object, ok := ctx.Value(authzObjectContextKey).(Object)
	return object, ok
}

// ContextWithAction 将授权操作注入上下文
// parent: 父上下文
// action: 授权操作
// 返回: 新的上下文
func ContextWithAction(parent context.Context, action Action) context.Context {
	return context.WithValue(parent, authzActionContextKey, action)
}

// ActionFromContext 从上下文中提取授权操作
// ctx: 上下文
// 返回: 授权操作和是否存在的标志
func ActionFromContext(ctx context.Context) (Action, bool) {
	action, ok := ctx.Value(authzActionContextKey).(Action)
	return action, ok
}

// ContextWithTenant 将授权租户注入上下文
// parent: 父上下文
// tenant: 授权租户
// 返回: 新的上下文
func ContextWithTenant(parent context.Context, tenant Tenant) context.Context {
	return context.WithValue(parent, authzTenantContextKey, tenant)
}

// TenantFromContext 从上下文中提取授权租户
// ctx: 上下文
// 返回: 授权租户和是否存在的标志
func TenantFromContext(ctx context.Context) (Tenant, bool) {
	tenant, ok := ctx.Value(authzTenantContextKey).(Tenant)
	return tenant, ok
}

// ContextWithAuthzResult 将授权结果注入上下文
// parent: 父上下文
// result: 授权结果
// 返回: 新的上下文
func ContextWithAuthzResult(parent context.Context, result bool) context.Context {
	return context.WithValue(parent, authzResultContextKey, result)
}

// AuthzResultFromContext 从上下文中提取授权结果
// ctx: 上下文
// 返回: 授权结果和是否存在的标志
func AuthzResultFromContext(ctx context.Context) (bool, bool) {
	result, ok := ctx.Value(authzResultContextKey).(bool)
	return result, ok
}

// ExtractAuthzInfo 从上下文中提取授权信息
// ctx: 上下文
// 返回: 主体、对象、操作、租户和是否完整的标志
func ExtractAuthzInfo(ctx context.Context) (Subject, Object, Action, Tenant, bool) {
	subject, subOk := SubjectFromContext(ctx)
	object, objOk := ObjectFromContext(ctx)
	action, actOk := ActionFromContext(ctx)
	tenant, tenantOk := TenantFromContext(ctx)

	// 所有信息都必须存在
	return subject, object, action, tenant, subOk && objOk && actOk && tenantOk
}

// ContextWithAuthzInfo 将授权信息注入上下文
// parent: 父上下文
// subject: 授权主体
// object: 授权对象
// action: 授权操作
// tenant: 授权租户
// 返回: 新的上下文
func ContextWithAuthzInfo(parent context.Context, subject Subject, object Object, action Action, tenant Tenant) context.Context {
	ctx := ContextWithSubject(parent, subject)
	ctx = ContextWithObject(ctx, object)
	ctx = ContextWithAction(ctx, action)
	ctx = ContextWithTenant(ctx, tenant)
	return ctx
}
