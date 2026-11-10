package biz

import (
	"context"
	"testing"

	"backend-service/pkg/auth/authn"

	kerr "github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ctxWithRole(role string) context.Context {
	return authn.ContextWithAuthClaims(context.Background(), &authn.AuthClaims{"role": role})
}

// TestRequireRole 覆盖后端角色断言。
//
// 这条测试是「权限控制必须在后端执行」这条硬规则的可执行证据：
// 前端 access code 只隐藏按钮，真正的拦截在这里。
func TestRequireRole(t *testing.T) {
	require.NoError(t, RequireAdmin(ctxWithRole(RoleAdmin)))
	require.NoError(t, RequireRole(ctxWithRole(RoleOperator), RoleAdmin, RoleOperator))

	err := RequireAdmin(ctxWithRole(RoleViewer))
	require.Error(t, err)
	assert.Equal(t, 403, authErrorCode(err))

	// operator 不能删数据源 / 驱动任务
	require.Error(t, RequireAdmin(ctxWithRole(RoleOperator)))
}

// TestRequireRole_NoToken 无令牌上下文必须按未登录处理，而不是放行。
func TestRequireRole_NoToken(t *testing.T) {
	err := RequireAdmin(context.Background())
	require.Error(t, err)
	assert.Equal(t, 401, authErrorCode(err), "缺 claim 时不能默认放行")
}

// TestRoleAccessCodesMatchAssertions 权限码表与后端断言必须一致。
//
// 前端按 access code 隐藏入口，后端按角色拦截；两边若各说各话，
// 就会出现「按钮看得见但接口 403」或反过来「按钮藏了但接口随便调」。
func TestRoleAccessCodesMatchAssertions(t *testing.T) {
	// admin：全部
	require.NoError(t, RequireRole(ctxWithRole(RoleAdmin), RoleAdmin, RoleOperator))
	require.NoError(t, RequireAdmin(ctxWithRole(RoleAdmin)))
	// operator：可写配置，不可 admin-only
	require.NoError(t, RequireRole(ctxWithRole(RoleOperator), RoleAdmin, RoleOperator))
	require.Error(t, RequireAdmin(ctxWithRole(RoleOperator)))
	// viewer：两者都不行
	require.Error(t, RequireRole(ctxWithRole(RoleViewer), RoleAdmin, RoleOperator))
	require.Error(t, RequireAdmin(ctxWithRole(RoleViewer)))

	// viewer 的 access code 必须只含只读项
	for _, code := range roleAccessCodes[RoleViewer] {
		assert.NotContains(t, code, "write", "viewer 不应拿到写权限码")
		assert.NotContains(t, code, "delete", "viewer 不应拿到删除权限码")
		assert.NotContains(t, code, "manage", "viewer 不应拿到任务管理权限码")
	}
}

// authErrorCode 取 kratos 错误的 HTTP 语义码。
//
// 断言码值而不是只断言「有错」：403（角色不够）与 401（没登录）在前端是
// 两套处理——401 会触发刷新/跳登录，403 只提示无权限。弄混会让 viewer
// 被踢回登录页。
func authErrorCode(err error) int { return int(kerr.FromError(err).Code) }
