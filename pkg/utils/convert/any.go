package convert

import "reflect"

// ToAny 将切片 source 中的每个元素通过 transform 转换为 D 类型。委托 SliceToAny 实现。
func ToAny[T any, D any](source []T, transform func(T) D) []D {
	return SliceToAny(source, transform)
}

// ToValue 将指针类型转换为对应的非指针值，如果为 nil 则返回零值。
// ptr: 任意类型的指针。
// 返回值: 指针指向的值，若指针为 nil 则返回该类型的零值。
// 示例:
//
//	str := "hello"
//	ptr := &str
//	val := ToValue(ptr) // string: "hello"
func ToValue[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
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
