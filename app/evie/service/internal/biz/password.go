package biz

import (
	"fmt"
)

// ValidatePassword validates password strength.
// TODO: implement proper password policy validation
func ValidatePassword(password string) error {
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}
	return nil
}
