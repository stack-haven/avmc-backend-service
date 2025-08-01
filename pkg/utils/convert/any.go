package convert

import "reflect"

// ToAny 将切片 source 中的每个元素通过 transform 函数转换为 D 类型，并返回一个新的 D 类型切片。
// source: 待转换的切片。
// transform: 用于转换元素的函数。
// 返回值: 转换后的新切片。
func ToAny[T any, D any](source []T, transform func(T) D) []D {
	l := make([]D, 0, len(source))
	for _, v := range source {
		l = append(l, transform(v))
	}
	return l
}

// ToPointer 将任意值转换为对应的指针引用。
// value: 任意类型的值。
// 返回值: 指向该值的指针。
// 示例:
//
//	str := "hello"
//	ptr := ToPointer(str) // *string
func ToPointer[T any](value T) *T {
	return &value
}

// EmptyToNil 判断值是否为空，为空则返回 nil，否则返回指针引用。
// 支持的类型：string、slice、map、指针、接口。
// value: 待检查的值。
// 返回值: 空值返回 nil，非空返回指针。
// 示例:
//
//	str := ""
//	ptr := EmptyToNil(str) // nil
//	str2 := "hello"
//	ptr2 := EmptyToNil(str2) // *string
func EmptyToNil[T any](value T) *T {
	if isEmpty(value) {
		return nil
	}
	return &value
}

// isEmpty 判断值是否为空。
func isEmpty[T any](value T) bool {
	v := any(value)
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == ""
	case []byte:
		return len(val) == 0
	case []rune:
		return len(val) == 0
	case []int:
		return len(val) == 0
	case []string:
		return len(val) == 0
	case []any:
		return len(val) == 0
	case map[string]string:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	case map[any]any:
		return len(val) == 0
	default:
		// 使用反射处理其他类型
		return reflect.ValueOf(v).IsZero()
	}
}
