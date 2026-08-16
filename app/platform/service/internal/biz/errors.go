package biz

import "errors"

// 预定义的业务层错误
var (
	ErrPasswordHashFailed            = errors.New("password hash failed")
	ErrStorageConfigNameRequired     = errors.New("storage config name required")
	ErrStorageConfigProviderRequired = errors.New("storage config provider required")
	ErrStorageConfigJSONRequired     = errors.New("storage config json required")
)
