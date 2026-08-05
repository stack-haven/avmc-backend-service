package convert

import (
	"strconv"
	"strings"
)

// StringToUint 将字符串转换为 uint 类型。若转换失败，返回 0。
// id: 待转换的字符串。
// 返回值: 转换后的 uint 类型值，失败则返回 0。
func StringToUint(id string) uint {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return 0
	}
	if idInt < 0 {
		return 0
	}
	return uint(idInt)
}

// DefaultString returns value if it is non-empty (after trimming spaces), otherwise returns fallback.
func DefaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
