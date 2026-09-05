// Package textenhance · errors.go
// Status 枚举 + 错误工厂。Status 的真实定义在 processors/status.go。
package textenhance

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"

	"backend-service/pkg/textenhance/processors"
)

// Status 枚举（re-export from processors）。
const (
	StatusUnknown  = processors.StatusUnknown
	StatusSuccess  = processors.StatusSuccess
	StatusPartial  = processors.StatusPartial
	StatusCanceled = processors.StatusCanceled
	StatusFailed   = processors.StatusFailed
	StatusPanic    = processors.StatusPanic
)

// StatusName 返回状态码可读名。
func StatusName(s int32) string { return processors.StatusName(s) }

// ErrProcessorNotFound Registry 未找到指定 processor。
func ErrProcessorNotFound(name string) *errors.Error {
	return errors.NotFound("TEXTENHANCE_PROCESSOR_NOT_FOUND", fmt.Sprintf("processor %q not registered", name))
}

// ErrProcessorOptionType Option 类型断言失败。
func ErrProcessorOptionType(name string, expected, got interface{}) *errors.Error {
	return errors.BadRequest("TEXTENHANCE_OPTION_TYPE", fmt.Sprintf("processor %q: expected %T, got %T", name, expected, got))
}

// ErrProcessorBuild Processor 构造失败（option 错误 / 内部异常）。
func ErrProcessorBuild(name string, err error) *errors.Error {
	return errors.BadRequest("TEXTENHANCE_PROCESSOR_BUILD", fmt.Sprintf("processor %q: build failed: %v", name, err))
}