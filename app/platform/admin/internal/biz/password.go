package biz

import (
	"unicode"

	"github.com/go-kratos/kratos/v2/errors"
)

const (
	minPasswordLength = 6
	maxPasswordLength = 72
)

func ValidatePassword(password string) error {
	length := len([]rune(password))
	if length < minPasswordLength || length > maxPasswordLength {
		return errors.BadRequest("USER_PASSWORD_TOO_WEAK", "密码长度必须在 6 到 72 个字符之间")
	}
	var upper, lower, digit, symbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			symbol = true
		}
	}
	if !upper || !lower || !digit || !symbol {
		return errors.BadRequest("USER_PASSWORD_TOO_WEAK", "密码必须包含大写字母、小写字母、数字和特殊字符")
	}
	return nil
}
