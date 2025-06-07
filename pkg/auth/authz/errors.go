package authz

import (
	"errors"
	"fmt"
)

// 错误码定义
type ErrorCode int

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
	// ErrCodeAddPoliciesFailed 批量更新策略失败
	ErrCodeUpdatePoliciesFailed
	// ErrCodeRemovePoliciesFailed 批量移除策略失败
	ErrCodeRemovePoliciesFailed
	// ErrCodeGetAllSubjectsFailed 获取所有主体失败
	ErrCodeGetAllSubjectsFailed
	// ErrCodeGetAllObjectsFailed 获取所有对象失败
	ErrCodeGetAllObjectsFailed
	// ErrCodeGetAllActionsFailed 获取所有操作失败
	ErrCodeGetAllActionsFailed
	// ErrCodeGetAllDomainsFailed 获取所有域失败
	ErrCodeGetAllDomainsFailed
	// ErrCodeGetAllRolesFailed 获取所有角色失败
	ErrCodeGetAllRolesFailed
	// ErrCodeGetRolesForUserFailed 获取用户角色失败
	ErrCodeGetRolesForUserFailed
	// ErrCodeGetRolesForUserFailed 更新用户角色失败
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
	// ErrCodeInvalidDomain 无效域
	ErrCodeInvalidDomain
	// ErrCodeInvalidPolicy 无效策略
	ErrCodeInvalidPolicy
	// ErrCodeInvalidRole 无效角色
	ErrCodeInvalidRole
	// ErrCodeInvalidUser 无效用户
	ErrCodeInvalidUser
	// ErrCodePermissionDenied 权限被拒绝
	ErrCodePermissionDenied
)

// 预定义错误
var (
	// ErrUnknown 未知错误
	ErrUnknown = errors.New("unknown authorization error")
	// ErrInitializationFailed 初始化失败
	ErrInitializationFailed = errors.New("authorizer initialization failed")
	// ErrProviderNotFound 提供者未找到
	ErrProviderNotFound = errors.New("authorization provider not found")
	// ErrInvalidConfiguration 无效配置
	ErrInvalidConfiguration = errors.New("invalid authorizer configuration")
	// ErrEnforceFailed 授权检查失败
	ErrEnforceFailed = errors.New("enforce check failed")
	// ErrBatchEnforceFailed 批量授权检查失败
	ErrBatchEnforceFailed = errors.New("batch enforce check failed")
	// ErrAddPolicyFailed 添加策略失败
	ErrAddPolicyFailed = errors.New("add policy failed")
	// ErrRemovePolicyFailed 移除策略失败
	ErrRemovePolicyFailed = errors.New("remove policy failed")
	// ErrAddPoliciesFailed 批量添加策略失败
	ErrAddPoliciesFailed = errors.New("add policies failed")
	// ErrRemovePoliciesFailed 批量移除策略失败
	ErrRemovePoliciesFailed = errors.New("remove policies failed")
	// ErrGetAllSubjectsFailed 获取所有主体失败
	ErrGetAllSubjectsFailed = errors.New("get all subjects failed")
	// ErrGetAllObjectsFailed 获取所有对象失败
	ErrGetAllObjectsFailed = errors.New("get all objects failed")
	// ErrGetAllActionsFailed 获取所有操作失败
	ErrGetAllActionsFailed = errors.New("get all actions failed")
	// ErrGetAllDomainsFailed 获取所有域失败
	ErrGetAllDomainsFailed = errors.New("get all domains failed")
	// ErrGetAllRolesFailed 获取所有角色失败
	ErrGetAllRolesFailed = errors.New("get all roles failed")
	// ErrGetRolesForUserFailed 获取用户角色失败
	ErrGetRolesForUserFailed = errors.New("get roles for user failed")
	// ErrGetUsersForRoleFailed 获取角色用户失败
	ErrGetUsersForRoleFailed = errors.New("get users for role failed")
	// ErrHasRoleForUserFailed 检查用户角色失败
	ErrHasRoleForUserFailed = errors.New("has role for user failed")
	// ErrAddRoleForUserFailed 添加用户角色失败
	ErrAddRoleForUserFailed = errors.New("add role for user failed")
	// ErrDeleteRoleForUserFailed 删除用户角色失败
	ErrDeleteRoleForUserFailed = errors.New("delete role for user failed")
	// ErrInvalidSubject 无效主体
	ErrInvalidSubject = errors.New("invalid subject")
	// ErrInvalidObject 无效对象
	ErrInvalidObject = errors.New("invalid object")
	// ErrInvalidAction 无效操作
	ErrInvalidAction = errors.New("invalid action")
	// ErrInvalidDomain 无效域
	ErrInvalidDomain = errors.New("invalid domain")
	// ErrInvalidPolicy 无效策略
	ErrInvalidPolicy = errors.New("invalid policy")
	// ErrInvalidRole 无效角色
	ErrInvalidRole = errors.New("invalid role")
	// ErrInvalidUser 无效用户
	ErrInvalidUser = errors.New("invalid user")
	// ErrPermissionDenied 权限被拒绝
	ErrPermissionDenied = errors.New("permission denied")
)

// AuthzError 授权错误类型
type AuthzError struct {
	// Code 错误码
	Code ErrorCode
	// Message 错误消息
	Message string
	// Err 原始错误
	Err error
}

// Error 实现error接口
func (e *AuthzError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("authorization error [code=%d]: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("authorization error [code=%d]: %s", e.Code, e.Message)
}

// Unwrap 解包错误
func (e *AuthzError) Unwrap() error {
	return e.Err
}

// NewAuthzError 创建新的授权错误
func NewAuthzError(code ErrorCode, message string, err error) *AuthzError {
	return &AuthzError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// IsAuthzError 检查错误是否为授权错误
func IsAuthzError(err error) bool {
	var authzErr *AuthzError
	return errors.As(err, &authzErr)
}

// GetAuthzErrorCode 获取授权错误码
func GetAuthzErrorCode(err error) (ErrorCode, bool) {
	var authzErr *AuthzError
	if errors.As(err, &authzErr) {
		return authzErr.Code, true
	}
	return ErrCodeUnknown, false
}
