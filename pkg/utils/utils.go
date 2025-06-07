package utils

import (
	"bytes"
	"encoding/json"
	"iter"
	"slices"
	"strings"
	"unicode/utf8"
)

func Filter[T any](slice []T, f func(T) bool) []T {
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

func FilterIndex[T any](slice []T, f func(T, int, []T) bool) []T {
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

func TryMap[T, U any](slice []T, f func(T) (U, error)) ([]U, error) {
	if len(slice) == 0 {
		return nil, nil
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

func MapNonNil[T any, U comparable](slice []T, f func(T) U) []U {
	var result []U
	for _, value := range slice {
		mapped := f(value)
		if mapped != *new(U) {
			result = append(result, mapped)
		}
	}
	return result
}

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

func Same[T any](s1 []T, s2 []T) bool {
	if len(s1) == len(s2) {
		return len(s1) == 0 || &s1[0] == &s2[0]
	}
	return false
}

func Some[T any](slice []T, f func(T) bool) bool {
	for _, value := range slice {
		if f(value) {
			return true
		}
	}
	return false
}

func Every[T any](slice []T, f func(T) bool) bool {
	for _, value := range slice {
		if !f(value) {
			return false
		}
	}
	return true
}

func Find[T any](slice []T, f func(T) bool) T {
	for _, value := range slice {
		if f(value) {
			return value
		}
	}
	return *new(T)
}

func FindLast[T any](slice []T, f func(T) bool) T {
	for i := len(slice) - 1; i >= 0; i-- {
		value := slice[i]
		if f(value) {
			return value
		}
	}
	return *new(T)
}

func FindIndex[T any](slice []T, f func(T) bool) int {
	for i, value := range slice {
		if f(value) {
			return i
		}
	}
	return -1
}

func FindLastIndex[T any](slice []T, f func(T) bool) int {
	for i := len(slice) - 1; i >= 0; i-- {
		value := slice[i]
		if f(value) {
			return i
		}
	}
	return -1
}

func FirstOrNil[T any](slice []T) T {
	if len(slice) != 0 {
		return slice[0]
	}
	return *new(T)
}

func LastOrNil[T any](slice []T) T {
	if len(slice) != 0 {
		return slice[len(slice)-1]
	}
	return *new(T)
}

func ElementOrNil[T any](slice []T, index int) T {
	if index < len(slice) {
		return slice[index]
	}
	return *new(T)
}

func FirstOrNilSeq[T any](seq iter.Seq[T]) T {
	if seq != nil {
		for value := range seq {
			return value
		}
	}
	return *new(T)
}

func FirstNonNil[T any, U comparable](slice []T, f func(T) U) U {
	for _, value := range slice {
		mapped := f(value)
		if mapped != *new(U) {
			return mapped
		}
	}
	return *new(U)
}

func Concatenate[T any](s1 []T, s2 []T) []T {
	if len(s2) == 0 {
		return s1
	}
	if len(s1) == 0 {
		return s2
	}
	return slices.Concat(s1, s2)
}

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

func CountWhere[T any](slice []T, f func(T) bool) int {
	count := 0
	for _, value := range slice {
		if f(value) {
			count++
		}
	}
	return count
}

func ReplaceElement[T any](slice []T, i int, t T) []T {
	result := slices.Clone(slice)
	result[i] = t
	return result
}

func InsertSorted[T any](slice []T, element T, cmp func(T, T) int) []T {
	i, _ := slices.BinarySearchFunc(slice, element, cmp)
	return slices.Insert(slice, i, element)
}

func AppendIfUnique[T comparable](slice []T, element T) []T {
	if slices.Contains(slice, element) {
		return slice
	}
	return append(slice, element)
}

func Memoize[T any](create func() T) func() T {
	var value T
	return func() T {
		if create != nil {
			value = create()
			create = nil
		}
		return value
	}
}

// Returns whenTrue if b is true; otherwise, returns whenFalse. IfElse should only be used when branches are either
// constant or precomputed as both branches will be evaluated regardless as to the value of b.
func IfElse[T any](b bool, whenTrue T, whenFalse T) T {
	if b {
		return whenTrue
	}
	return whenFalse
}

// Returns value if value is not the zero value of T; Otherwise, returns defaultValue. OrElse should only be used when
// defaultValue is constant or precomputed as its argument will be evaluated regardless as to the content of value.
func OrElse[T comparable](value T, defaultValue T) T {
	if value != *new(T) {
		return value
	}
	return defaultValue
}

// Returns `a` if `a` is not `nil`; Otherwise, returns `b`. Coalesce is roughly analogous to `??` in JS, except that it
// non-shortcutting, so it is advised to only use a constant or precomputed value for `b`
func Coalesce[T *U, U any](a T, b T) T {
	if a == nil {
		return b
	} else {
		return a
	}
}

func Flatten[T any](array [][]T) []T {
	var result []T
	for _, subArray := range array {
		result = append(result, subArray...)
	}
	return result
}

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// Extracts the first value of a multi-value return.
func FirstResult[T1 any](t1 T1, _ ...any) T1 {
	return t1
}

func StringifyJson(input any, prefix string, indent string) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, indent)
	if _, ok := input.([]any); ok && len(input.([]any)) == 0 {
		return "[]", nil
	}
	if err := encoder.Encode(input); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func Identity[T any](t T) T {
	return t
}

func CheckEachDefined[S any](s []*S, msg string) []*S {
	for _, value := range s {
		if value == nil {
			panic(msg)
		}
	}
	return s
}

func StripQuotes(name string) string {
	firstChar, _ := utf8.DecodeRuneInString(name)
	lastChar, _ := utf8.DecodeLastRuneInString(name)
	if firstChar == lastChar && (firstChar == '\'' || firstChar == '"' || firstChar == '`') {
		return name[1 : len(name)-1]
	}
	return name
}
