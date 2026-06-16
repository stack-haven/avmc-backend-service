package biz

import "errors"

// 预定义的业务层错误
var (
	ErrPasswordHashFailed = errors.New("password hash failed")
)
