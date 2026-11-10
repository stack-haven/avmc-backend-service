package biz

import (
	"context"

	pb "backend-service/api/edai/service/v1"

	"backend-service/pkg/auth/authn"
)

// EDAI V1.0 的角色集合。
//
// 刻意只留三档（与 auth.go 的 roleAccessCodes 一一对应，不能各说各话）：
//   - viewer   只读
//   - operator 可维护数据源配置，但不能删、不能驱动任务
//   - admin    全部
//
// Local-first 单机部署不存在复杂组织授权，但「读 / 改 / 破坏性」必须分开，
// 否则前端的按钮隐藏就是唯一的权限屏障。
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// RequireRole 断言当前登录令牌的角色在 roles 之内。
//
// 为什么必须显式断言（而不是只依赖认证中间件）：
// 项目硬规则要求「权限控制必须在后端执行，不能只依赖 UI 隐藏」。
// EDAI 按 ADR-013 不接 Casbin，前端靠 `/auth/codes` 返回的 access code 隐藏按钮；
// 如果后端只验令牌有效性，任何拿到 viewer 令牌的人都能删数据源、发起或取消扫描。
//
// 断言放在 biz 而不是 HTTP 中间件：扫描/数据源将来也会被 MCP 或内部调用触达，
// 校验点必须覆盖所有入口，而不是只挡 REST。
func RequireRole(ctx context.Context, roles ...string) error {
	claims, ok := authn.AuthClaimsFromContext(ctx)
	if !ok {
		return pb.ErrorNotLoggedIn("未登录或令牌已失效")
	}
	role, _ := (*claims)["role"].(string)
	for _, want := range roles {
		if role == want {
			return nil
		}
	}
	return pb.ErrorAccessForbidden("该操作需要管理员角色")
}

// RequireAdmin 是 RequireRole(ctx, RoleAdmin) 的语义别名。
func RequireAdmin(ctx context.Context) error { return RequireRole(ctx, RoleAdmin) }
