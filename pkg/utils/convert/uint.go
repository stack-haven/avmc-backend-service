package convert

// Uint 包提供 uint 类型与字符串相互转换的工具函数。

import "strconv"

// UnitToString 将 uint 类型的值转换为字符串。
// id: 待转换的 uint 类型值。
// 返回值: 转换后的字符串。
func UnitToString(id uint) string {
	return strconv.Itoa(int(id))
}

// Unit32ToString 将 uint32 类型的值转换为字符串。
// id: 待转换的 uint32 类型值。
// 返回值: 转换后的字符串。
func Unit32ToString(id uint32) string {
	return strconv.FormatUint(uint64(id), 10)
}

// StringToUnit32 将字符串转换为 uint32 类型的值。若转换失败，返回 0。
// id: 待转换的字符串。
// 返回值: 转换后的 uint32 类型值，失败则返回 0。
func StringToUnit32(id string) uint32 {
	ut64, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0
	}
	if err != nil {
		return 0
	}
	return uint32(ut64)
}

// Unit64ToString 将 uint64 类型的值转换为字符串。
// id: 待转换的 uint64 类型值。
// 返回值: 转换后的字符串。
func Unit64ToString(id uint64) string {
	return strconv.FormatUint(id, 10)
}

// StringToUnit64 将字符串转换为 uint64 类型的值。若转换失败，返回 0。
// id: 待转换的字符串。
// 返回值: 转换后的 uint64 类型值，失败则返回 0。
func StringToUnit64(id string) uint64 {
	ut64, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0
	}
	if err != nil {
		return 0
	}
	return ut64
}
