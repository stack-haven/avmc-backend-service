package convert

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
