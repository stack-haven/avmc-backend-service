package convert

// String 包提供字符串与其他类型转换的工具函数。

import (
	"strconv"
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
