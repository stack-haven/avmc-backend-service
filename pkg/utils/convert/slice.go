package convert

import (
	"fmt"
	"strconv"
	"strings"
)

// StringToBool 将字符串转换为布尔值。
// s: 待转换的字符串，支持 "true", "false", "1", "0"。
// 返回值: 转换后的布尔值，若转换失败返回 false。
func StringToBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "1":
		return true
	case "false", "0":
		return false
	default:
		return false
	}
}

// SliceContains 检查切片中是否包含指定元素。
// arr: 待检查的切片，元素类型为 T。
// target: 要查找的目标元素。
// 返回值: 如果包含返回 true，否则返回 false。
func SliceContains[T comparable](arr []T, target T) bool {
	for _, v := range arr {
		if v == target {
			return true
		}
	}
	return false
}

// SliceToString 将一个 interface 类型的切片转换为逗号分隔的字符串。
// array: 待转换的 interface 类型切片。
// 返回值: 转换后的逗号分隔字符串。
func SliceToString(array []interface{}) string {
	return strings.Replace(strings.Trim(fmt.Sprint(array), "[]"), " ", ",", -1)
}

type Number interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64
}

type Map[T Number | string, D bool | byte | string | struct{}] map[T]D

// SliceUnique 去除切片中的重复元素。
// arr: 待处理的切片，元素类型为 Number 或 string。
// 返回值: 去重后的切片。
func SliceUnique[T Number | string, D bool](arr []T) []T {
	if len(arr) <= 1 {
		return arr
	}
	temp, result := make(Map[T, D]), make([]T, 0)
	for _, v := range arr {
		_, ok := temp[v]
		if !ok {
			result = append(result, v)
			temp[v] = true
		}
	}
	return result
}

// SliceStringToUint 将字符串切片转换为 uint 类型切片，忽略转换失败的元素。
// array: 待转换的字符串切片。
// 返回值: 转换后的 uint 类型切片。
func SliceStringToUint(array []string) []uint {
	arrUint := make([]uint, 0, len(array))
	for _, v := range array {
		id, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		arrUint = append(arrUint, uint(id))
	}
	return arrUint
}

// SliceToAny 将切片 source 中的每个元素通过 transform 函数转换为 D 类型，并返回一个新的 D 类型切片。
// source: 待转换的切片。
// transform: 用于转换元素的函数。
// 返回值: 转换后的新切片。
func SliceToAny[T any, D any](source []T, transform func(T) D) []D {
	l := make([]D, 0, len(source))
	for _, v := range source {
		l = append(l, transform(v))
	}
	return l
}
