// Package utils 提供通用的工具函数和辅助方法
// 包含数组/切片操作、条件处理、JSON序列化等常用功能
package utils

import (
	"bytes"
	"encoding/json"
	"iter"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// Filter 根据提供的条件函数过滤切片中的元素
// 保留满足条件的元素，移除不满足条件的元素
// 如果输入切片为nil，则返回nil
// 参数:
//
//	slice: 要过滤的切片
//	f: 条件函数，返回true表示保留元素，false表示移除元素
//
// 返回值:
//
//	过滤后的新切片
func Filter[T any](slice []T, f func(T) bool) []T {
	if slice == nil {
		return nil
	}
	for i, value := range slice {
		if !f(value) {
			result := slices.Clone(slice[:i])
			for i++; i < len(slice); i++ {
				value = slice[i]
				if f(value) {
					result = append(result, value)
				}
			}
			return result
		}
	}
	return slice
}

// FilterIndex 根据提供的条件函数过滤切片中的元素
// 与Filter不同，条件函数还接收元素索引和原始切片作为参数
// 如果输入切片为nil，则返回nil
// 参数:
//
//	slice: 要过滤的切片
//	f: 条件函数，接收元素、索引和原始切片，返回true表示保留元素
//
// 返回值:
//
//	过滤后的新切片
func FilterIndex[T any](slice []T, f func(T, int, []T) bool) []T {
	if slice == nil {
		return nil
	}
	for i, value := range slice {
		if !f(value, i, slice) {
			result := slices.Clone(slice[:i])
			for i++; i < len(slice); i++ {
				value = slice[i]
				if f(value, i, slice) {
					result = append(result, value)
				}
			}
			return result
		}
	}
	return slice
}

// Map 将切片中的每个元素通过映射函数转换为新类型的值
// 参数:
//
//	slice: 要映射的切片
//	f: 映射函数，接收T类型的值，返回U类型的值
//
// 返回值:
//
//	包含映射结果的新切片（如果slice为nil则返回nil）
func Map[T, U any](slice []T, f func(T) U) []U {
	if slice == nil {
		return nil
	}
	result := make([]U, len(slice))
	for i, value := range slice {
		result[i] = f(value)
	}
	return result
}

// TryMap 将切片中的每个元素通过映射函数转换为新类型的值
// 与Map不同，映射函数可能返回错误，任何错误都会导致整个操作失败
// 参数:
//
//	slice: 要映射的切片
//	f: 映射函数，接收T类型的值，返回U类型的值和可能的错误
//
// 返回值:
//
//	包含映射结果的新切片（如果slice为空则返回空切片）和可能的错误
func TryMap[T, U any](slice []T, f func(T) (U, error)) ([]U, error) {
	if len(slice) == 0 {
		return []U{}, nil
	}
	result := make([]U, len(slice))
	for i, value := range slice {
		mapped, err := f(value)
		if err != nil {
			return nil, err
		}
		result[i] = mapped
	}
	return result, nil
}

// MapIndex 将切片中的每个元素通过映射函数转换为新类型的值
// 与Map不同，映射函数还接收元素索引作为参数
// 参数:
//
//	slice: 要映射的切片
//	f: 映射函数，接收T类型的值和索引，返回U类型的值
//
// 返回值:
//
//	包含映射结果的新切片
func MapIndex[T, U any](slice []T, f func(T, int) U) []U {
	if slice == nil {
		return nil
	}
	result := make([]U, len(slice))
	for i, value := range slice {
		result[i] = f(value, i)
	}
	return result
}

// MapNonNil 将切片中的每个元素通过映射函数转换为新类型的值
// 只保留非零值的映射结果
// 如果输入切片为nil，则返回nil
// 参数:
//
//	slice: 要映射的切片
//	f: 映射函数，接收T类型的值，返回U类型的值
//
// 返回值:
//
//	包含非零映射结果的新切片
func MapNonNil[T any, U comparable](slice []T, f func(T) U) []U {
	if slice == nil {
		return nil
	}
	var result []U
	for _, value := range slice {
		mapped := f(value)
		if mapped != *new(U) {
			result = append(result, mapped)
		}
	}
	return result
}

// SameMap 将切片中的每个元素通过映射函数转换为相同类型的值
// 如果所有元素的映射结果都与原元素相同，则返回原切片
// 否则创建一个新切片，包含所有元素的映射结果
// 参数:
//
//	slice: 要映射的切片
//	f: 映射函数，接收T类型的值，返回T类型的值
//
// 返回值:
//
//	原切片（如果所有映射结果都相同）或新的映射结果切片
func SameMap[T comparable](slice []T, f func(T) T) []T {
	for i, value := range slice {
		mapped := f(value)
		if mapped != value {
			result := make([]T, len(slice))
			copy(result, slice[:i])
			result[i] = mapped
			for j := i + 1; j < len(slice); j++ {
				result[j] = f(slice[j])
			}
			return result
		}
	}
	return slice
}

// SameMapIndex 将切片中的每个元素通过映射函数转换为相同类型的值
// 与SameMap不同，映射函数还接收元素索引作为参数
// 如果所有元素的映射结果都与原元素相同，则返回原切片
// 否则创建一个新切片，包含所有元素的映射结果
// 参数:
//
//	slice: 要映射的切片
//	f: 映射函数，接收T类型的值和索引，返回T类型的值
//
// 返回值:
//
//	原切片（如果所有映射结果都相同）或新的映射结果切片
func SameMapIndex[T comparable](slice []T, f func(T, int) T) []T {
	for i, value := range slice {
		mapped := f(value, i)
		if mapped != value {
			result := make([]T, len(slice))
			copy(result, slice[:i])
			result[i] = mapped
			for j := i + 1; j < len(slice); j++ {
				result[j] = f(slice[j], j)
			}
			return result
		}
	}
	return slice
}

// Same 检查两个切片是否引用相同的底层数组
// 注意：这不是深度比较，只是检查切片是否指向相同的内存位置
// 两个切片可能长度不同但共享部分底层数组，此函数会返回false
// 参数:
//
//	s1: 第一个切片
//	s2: 第二个切片
//
// 返回值:
//
//	如果两个切片引用相同的底层数组且长度相同，则返回true；否则返回false
func Same[T any](s1 []T, s2 []T) bool {
	if len(s1) == len(s2) {
		return len(s1) == 0 || &s1[0] == &s2[0]
	}
	return false
}

// Some 检查切片中是否有至少一个元素满足条件函数
// 参数:
//
//	slice: 要检查的切片
//	f: 条件函数，返回true表示元素满足条件
//
// 返回值:
//
//	如果有至少一个元素满足条件，则返回true；否则返回false
func Some[T any](slice []T, f func(T) bool) bool {
	for _, value := range slice {
		if f(value) {
			return true
		}
	}
	return false
}

// Every 检查切片中的所有元素是否都满足条件函数
// 参数:
//
//	slice: 要检查的切片
//	f: 条件函数，返回true表示元素满足条件
//
// 返回值:
//
//	如果所有元素都满足条件，则返回true；否则返回false
func Every[T any](slice []T, f func(T) bool) bool {
	for _, value := range slice {
		if !f(value) {
			return false
		}
	}
	return true
}

// Find 在切片中查找第一个满足条件函数的元素
// 参数:
//
//	slice: 要查找的切片
//	f: 条件函数，返回true表示找到匹配元素
//
// 返回值:
//
//	第一个满足条件的元素和true（如果找到），否则返回T类型的零值和false
func Find[T any](slice []T, f func(T) bool) (T, bool) {
	for _, value := range slice {
		if f(value) {
			return value, true
		}
	}
	return *new(T), false
}

// FindLast 在切片中从后向前查找第一个满足条件函数的元素
// 参数:
//
//	slice: 要查找的切片
//	f: 条件函数，返回true表示找到匹配元素
//
// 返回值:
//
//	最后一个满足条件的元素和true（如果找到），否则返回T类型的零值和false
func FindLast[T any](slice []T, f func(T) bool) (T, bool) {
	for i := len(slice) - 1; i >= 0; i-- {
		value := slice[i]
		if f(value) {
			return value, true
		}
	}
	return *new(T), false
}

// FindIndex 在切片中查找第一个满足条件函数的元素的索引
// 参数:
//
//	slice: 要查找的切片
//	f: 条件函数，返回true表示找到匹配元素
//
// 返回值:
//
//	第一个满足条件的元素的索引，如果没有找到则返回-1
func FindIndex[T any](slice []T, f func(T) bool) int {
	for i, value := range slice {
		if f(value) {
			return i
		}
	}
	return -1
}

// FindLastIndex 在切片中从后向前查找第一个满足条件函数的元素的索引
// 参数:
//
//	slice: 要查找的切片
//	f: 条件函数，返回true表示找到匹配元素
//
// 返回值:
//
//	最后一个满足条件的元素的索引，如果没有找到则返回-1
func FindLastIndex[T any](slice []T, f func(T) bool) int {
	for i := len(slice) - 1; i >= 0; i-- {
		value := slice[i]
		if f(value) {
			return i
		}
	}
	return -1
}

// FirstOrNil 获取切片的第一个元素
// 如果切片为空，则返回T类型的零值
// 参数:
//
//	slice: 要获取元素的切片
//
// 返回值:
//
//	切片的第一个元素，如果切片为空则返回T类型的零值
func FirstOrNil[T any](slice []T) T {
	if len(slice) != 0 {
		return slice[0]
	}
	return *new(T)
}

// LastOrNil 获取切片的最后一个元素
// 如果切片为空，则返回T类型的零值
// 参数:
//
//	slice: 要获取元素的切片
//
// 返回值:
//
//	切片的最后一个元素，如果切片为空则返回T类型的零值
func LastOrNil[T any](slice []T) T {
	if len(slice) != 0 {
		return slice[len(slice)-1]
	}
	return *new(T)
}

// ElementOrNil 获取切片中指定索引的元素
// 如果索引越界（负数或超出切片长度），则返回T类型的零值
// 参数:
//
//	slice: 要获取元素的切片
//	index: 元素的索引
//
// 返回值:
//
//	切片中指定索引的元素，如果索引越界则返回T类型的零值
func ElementOrNil[T any](slice []T, index int) T {
	if index >= 0 && index < len(slice) {
		return slice[index]
	}
	return *new(T)
}

// FirstOrNilSeq 从迭代器序列中获取第一个元素
// 如果序列为空或nil，则返回T类型的零值
// 参数:
//
//	seq: 迭代器序列
//
// 返回值:
//
//	序列中的第一个元素，如果序列为空或nil则返回T类型的零值
func FirstOrNilSeq[T any](seq iter.Seq[T]) T {
	if seq != nil {
		for value := range seq {
			return value
		}
	}
	return *new(T)
}

// FirstNonNil 遍历切片，返回第一个非零的映射结果
// 对切片中的每个元素应用映射函数，返回第一个非零的映射结果
// 如果所有映射结果都是零值，则返回U类型的零值
// 参数:
//
//	slice: 要遍历的切片
//	f: 映射函数，接收T类型的值，返回U类型的值
//
// 返回值:
//
//	第一个非零的映射结果，如果所有都是零值则返回U类型的零值
func FirstNonNil[T any, U comparable](slice []T, f func(T) U) U {
	for _, value := range slice {
		mapped := f(value)
		if mapped != *new(U) {
			return mapped
		}
	}
	return *new(U)
}

// Concatenate 连接两个切片
// 参数:
//
//	s1: 第一个切片
//	s2: 第二个切片
//
// 返回值:
//
//	包含s1和s2所有元素的新切片
func Concatenate[T any](s1 []T, s2 []T) []T {
	if len(s2) == 0 {
		return s1
	}
	if len(s1) == 0 {
		return s2
	}
	return slices.Concat(s1, s2)
}

// Splice 在切片中插入或删除元素
// 从指定位置开始删除指定数量的元素，并插入新元素
// 参数:
//
//	s1: 要操作的切片
//	start: 开始操作的索引位置（负数表示从末尾开始计算）
//	deleteCount: 要删除的元素数量
//	items: 要插入的元素
//
// 返回值:
//
//	操作后的新切片
func Splice[T any](s1 []T, start int, deleteCount int, items ...T) []T {
	if start < 0 {
		start = len(s1) + start
	}
	if start < 0 {
		start = 0
	}
	if start > len(s1) {
		start = len(s1)
	}
	if deleteCount < 0 {
		deleteCount = 0
	}
	end := min(start+max(deleteCount, 0), len(s1))
	if start == end && len(items) == 0 {
		return s1
	}
	return slices.Concat(s1[:start], items, s1[end:])
}

// CountWhere 计算切片中满足条件函数的元素数量
// 参数:
//
//	slice: 要计数的切片
//	f: 条件函数，返回true表示元素满足条件
//
// 返回值:
//
//	满足条件的元素数量
func CountWhere[T any](slice []T, f func(T) bool) int {
	count := 0
	for _, value := range slice {
		if f(value) {
			count++
		}
	}
	return count
}

// ReplaceElement 替换切片中指定索引的元素
// 创建一个新切片，其中指定索引的元素被替换为新值
// 如果索引越界，将触发panic
// 参数:
//
//	slice: 要操作的切片
//	i: 要替换的元素的索引
//	t: 新的元素值
//
// 返回值:
//
//	替换后的新切片
func ReplaceElement[T any](slice []T, i int, t T) []T {
	if i < 0 || i >= len(slice) {
		panic("index out of range")
	}
	result := slices.Clone(slice)
	result[i] = t
	return result
}

// InsertSorted 将元素插入到已排序的切片中的正确位置
// 保持切片的排序状态
// 参数:
//
//	slice: 已排序的切片
//	element: 要插入的元素
//	cmp: 比较函数，用于确定元素的插入位置
//
// 返回值:
//
//	插入元素后的新切片
func InsertSorted[T any](slice []T, element T, cmp func(T, T) int) []T {
	i, _ := slices.BinarySearchFunc(slice, element, cmp)
	return slices.Insert(slice, i, element)
}

// AppendIfUnique 向切片中添加元素，但仅当元素不存在于切片中时才添加
// 参数:
//
//	slice: 要操作的切片
//	element: 要添加的元素
//
// 返回值:
//
//	如果元素已存在则返回原切片，否则返回添加元素后的新切片
func AppendIfUnique[T comparable](slice []T, element T) []T {
	if slices.Contains(slice, element) {
		return slice
	}
	return append(slice, element)
}

// Memoize 创建一个记忆化函数
// 该函数只执行一次创建逻辑，之后每次调用都返回缓存的结果
// 该实现是线程安全的
// 参数:
//
//	create: 创建值的函数
//
// 返回值:
//
//	一个函数，首次调用时执行create并缓存结果，后续调用直接返回缓存的结果
func Memoize[T any](create func() T) func() T {
	var (
		value T
		once  sync.Once
	)
	return func() T {
		once.Do(func() {
			value = create()
		})
		return value
	}
}

// IfElse 根据条件返回不同的值
// 如果条件为true，则返回whenTrue；否则返回whenFalse
// 注意：无论条件如何，两个分支都会被求值
// 仅当分支是常量或预先计算好的值时才应使用此函数
// 参数:
//
//	b: 条件表达式
//	whenTrue: 条件为true时返回的值
//	whenFalse: 条件为false时返回的值
//
// 返回值:
//
//	根据条件选择的对应值
func IfElse[T any](b bool, whenTrue T, whenFalse T) T {
	if b {
		return whenTrue
	}
	return whenFalse
}

// OrElse 如果值不是零值则返回该值，否则返回默认值
// 注意：无论value是否为零值，defaultValue都会被求值
// 仅当defaultValue是常量或预先计算好的值时才应使用此函数
// 参数:
//
//	value: 要检查的值
//	defaultValue: 当value为零值时返回的默认值
//
// 返回值:
//
//	value（如果不是零值）或defaultValue（如果value是零值）
func OrElse[T comparable](value T, defaultValue T) T {
	if value != *new(T) {
		return value
	}
	return defaultValue
}

// Coalesce 返回第一个非nil的值
// 如果a不是nil，则返回a；否则返回b
// 注意：这不是短路操作，无论a是否为nil，b都会被求值
// 建议仅为b使用常量或预先计算好的值
// 参数:
//
//	a: 第一个要检查的值
//	b: 当a为nil时返回的默认值
//
// 返回值:
//
//	a（如果不是nil）或b（如果a是nil）
func Coalesce[T *U, U any](a T, b T) T {
	if a == nil {
		return b
	} else {
		return a
	}
}

// Flatten 将二维切片展平为一维切片
// 如果输入为nil，则返回nil
// 参数:
//
//	array: 二维切片
//
// 返回值:
//
//	包含所有元素的一维切片
func Flatten[T any](array [][]T) []T {
	if array == nil {
		return nil
	}
	var result []T
	for _, subArray := range array {
		result = append(result, subArray...)
	}
	return result
}

// Must 处理可能返回错误的操作
// 如果err不为nil，则panic；否则返回v
// 通常用于确保关键操作成功，不容许失败的场景
// 参数:
//
//	v: 操作成功时的值
//	err: 可能的错误
//
// 返回值:
//
//	v（如果err为nil）
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// FirstResult 提取多值返回中的第一个值
// 忽略其他返回值
// 参数:
//
//	t1: 第一个返回值
//	_: 其他要忽略的返回值
//
// 返回值:
//
//	t1
func FirstResult[T1 any](t1 T1, _ ...any) T1 {
	return t1
}

// StringifyJson 将任意值序列化为格式化的JSON字符串
// 参数:
//
//	input: 要序列化的值
//	prefix: 每行的前缀
//	indent: 缩进字符串
//
// 返回值:
//
//	格式化的JSON字符串和可能的错误
func StringifyJson(input any, prefix string, indent string) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, indent)
	if input == nil {
		return "null", nil
	}
	if _, ok := input.([]any); ok && len(input.([]any)) == 0 {
		return "[]", nil
	}
	if err := encoder.Encode(input); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// Identity 恒等函数，返回输入的值本身
// 参数:
//
//	t: 输入值
//
// 返回值:
//
//	输入值t
func Identity[T any](t T) T {
	return t
}

// CheckEachDefined 检查切片中的每个元素是否都不为nil
// 如果发现nil元素，则panic并显示指定的错误消息
// 参数:
//
//	s: 要检查的切片
//	msg: 当发现nil元素时显示的错误消息
//
// 返回值:
//
//	原切片（如果所有元素都不为nil）
func CheckEachDefined[S any](s []*S, msg string) []*S {
	for _, value := range s {
		if value == nil {
			panic(msg)
		}
	}
	return s
}

// StripQuotes 去除字符串两端的引号
// 如果字符串以单引号、双引号或反引号开头和结尾，则去除这些引号
// 否则返回原字符串
// 参数:
//
//	name: 要处理的字符串
//
// 返回值:
//
//	去除两端引号后的字符串
func StripQuotes(name string) string {
	firstChar, _ := utf8.DecodeRuneInString(name)
	lastChar, _ := utf8.DecodeLastRuneInString(name)
	if firstChar == lastChar && (firstChar == '\'' || firstChar == '"' || firstChar == '`') {
		return name[1 : len(name)-1]
	}
	return name
}
