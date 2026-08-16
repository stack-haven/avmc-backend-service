package authz

import (
	"errors"

	"backend-service/pkg/auth/errs"
)

// ErrorCode 错误码类型（别名，统一到 errs.ErrorCode）。
type ErrorCode = errs.ErrorCode

// 错误码定义
const (
	// ErrCodeUnknown 未知错误
	ErrCodeUnknown ErrorCode = iota
	// ErrCodeInitializationFailed 初始化失败
	ErrCodeInitializationFailed
	// ErrCodeProviderNotFound 提供者未找到
	ErrCodeProviderNotFound
	// ErrCodeInvalidConfiguration 无效配置
	ErrCodeInvalidConfiguration
	// ErrCodeEnforceFailed 授权检查失败
	ErrCodeEnforceFailed
	// ErrCodeBatchEnforceFailed 批量授权检查失败
	ErrCodeBatchEnforceFailed
	// ErrCodeAddPolicyFailed 添加策略失败
	ErrCodeAddPolicyFailed
	// ErrCodeRemovePolicyFailed 移除策略失败
	ErrCodeRemovePolicyFailed
	// ErrCodeAddPoliciesFailed 批量添加策略失败
	ErrCodeAddPoliciesFailed
	// ErrCodeUpdatePoliciesFailed 批量更新策略失败
	ErrCodeUpdatePoliciesFailed
	// ErrCodeRemovePoliciesFailed 批量移除策略失败
	ErrCodeRemovePoliciesFailed
	// ErrCodeGetAllSubjectsFailed 获取所有主体失败
	ErrCodeGetAllSubjectsFailed
	// ErrCodeGetAllObjectsFailed 获取所有对象失败
	ErrCodeGetAllObjectsFailed
	// ErrCodeGetAllActionsFailed 获取所有操作失败
	ErrCodeGetAllActionsFailed
	// ErrCodeGetAllTenantsFailed 获取所有租户失败
	ErrCodeGetAllTenantsFailed
	// ErrCodeGetAllRolesFailed 获取所有角色失败
	ErrCodeGetAllRolesFailed
	// ErrCodeGetRolesForUserFailed 获取用户角色失败
	ErrCodeGetRolesForUserFailed
	// ErrCodeUpdateRolesForUserFailed 更新用户角色失败
	ErrCodeUpdateRolesForUserFailed
	// ErrCodeGetUsersForRoleFailed 获取角色用户失败
	ErrCodeGetUsersForRoleFailed
	// ErrCodeHasRoleForUserFailed 检查用户角色失败
	ErrCodeHasRoleForUserFailed
	// ErrCodeAddRoleForUserFailed 添加用户角色失败
	ErrCodeAddRoleForUserFailed
	// ErrCodeDeleteRoleForUserFailed 删除用户角色失败
	ErrCodeDeleteRoleForUserFailed
	// ErrCodeInvalidSubject 无效主体
	ErrCodeInvalidSubject
	// ErrCodeInvalidObject 无效对象
	ErrCodeInvalidObject
	// ErrCodeInvalidAction 无效操作
	ErrCodeInvalidAction
	// ErrCodeInvalidTenant 无效租户
	ErrCodeInvalidTenant
	// ErrCodeInvalidPolicy 无效策略
	ErrCodeInvalidPolicy
	// ErrCodeInvalidRole 无效角色
	ErrCodeInvalidRole
	// ErrCodeInvalidUser 无效用户
	ErrCodeInvalidUser
	// ErrCodePermissionDenied 权限被拒绝
	ErrCodePermissionDenied
)

// 预定义错误（纯字符串错误，供 errors.Is 判断使用）
var (
	ErrUnknown                 = errors.New("unknown authorization error")
	ErrInitializationFailed    = errors.New("authorizer initialization failed")
	ErrProviderNotFound        = errors.New("authorization provider not found")
	ErrInvalidConfiguration    = errors.New("invalid authorizer configuration")
	ErrEnforceFailed           = errors.New("enforce check failed")
	ErrBatchEnforceFailed      = errors.New("batch enforce check failed")
	ErrAddPolicyFailed         = errors.New("add policy failed")
	ErrRemovePolicyFailed      = errors.New("remove policy failed")
	ErrAddPoliciesFailed       = errors.New("add policies failed")
	ErrRemovePoliciesFailed    = errors.New("remove policies failed")
	ErrGetAllSubjectsFailed    = errors.New("get all subjects failed")
	ErrGetAllObjectsFailed     = errors.New("get all objects failed")
	ErrGetAllActionsFailed     = errors.New("get all actions failed")
	ErrGetAllTenantsFailed     = errors.New("get all tenants failed")
	ErrGetAllRolesFailed       = errors.New("get all roles failed")
	ErrGetRolesForUserFailed   = errors.New("get roles for user failed")
	ErrGetUsersForRoleFailed   = errors.New("get users for role failed")
	ErrHasRoleForUserFailed    = errors.New("has role for user failed")
	ErrAddRoleForUserFailed    = errors.New("add role for user failed")
	ErrDeleteRoleForUserFailed = errors.New("delete role for user failed")
	ErrInvalidSubject          = errors.New("invalid subject")
	ErrInvalidObject           = errors.New("invalid object")
	ErrInvalidAction           = errors.New("invalid action")
	ErrInvalidTenant           = errors.New("invalid tenant")
	ErrInvalidPolicy           = errors.New("invalid policy")
	ErrInvalidRole             = errors.New("invalid role")
	ErrInvalidUser             = errors.New("invalid user")
	ErrPermissionDenied        = errors.New("permission denied")
)

// AuthzError 授权错误类型（别名，统一到 errs.Error）。
type AuthzError = errs.Error

// NewAuthzError 创建授权错误（转发到统一错误结构）。
func NewAuthzError(code ErrorCode, message string, err error) *AuthzError {
	return errs.New(code, message, err)
}

// IsAuthzError 检查错误是否为授权错误。
func IsAuthzError(err error) bool {
	return errs.Is(err)
}

// GetAuthzErrorCode 获取授权错误码。
func GetAuthzErrorCode(err error) (ErrorCode, bool) {
	return errs.GetCode(err)
}
