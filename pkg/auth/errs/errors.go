package errs

import (
	"errors"
	"fmt"
)

// ErrorCode 统一错误码类型，认证（authn）与鉴权（authz）共用。
// 各子包可基于此类型定义自己的错误码枚举，但错误结构与判断逻辑统一。
type ErrorCode int

// Error 统一错误结构，认证与鉴权共用。
// authn.AuthError 与 authz.AuthzError 均为本类型的别名。
type Error struct {
	// Code 错误码
	Code ErrorCode
	// Message 错误消息
	Message string
	// Err 原始错误
	Err error
}

// Error 实现 error 接口。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("auth error [code=%d]: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("auth error [code=%d]: %s", e.Code, e.Message)
}

// Unwrap 解包错误，支持 errors.Is / errors.As 链式判断。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewError 创建统一错误。
func New(code ErrorCode, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

// IsError 判断错误是否为统一错误结构。
func Is(err error) bool {
	var e *Error
	return errors.As(err, &e)
}

// GetErrorCode 获取统一错误码，非统一错误返回 (0, false)。
func GetCode(err error) (ErrorCode, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Code, true
	}
	return 0, false
}
