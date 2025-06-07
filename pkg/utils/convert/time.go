package convert

// Time 包提供时间与字符串相互转换的工具函数。

import (
	"time"
)

// TimeValueToString 将时间指针转换为指定格式的字符串指针。
// t: 待转换的时间指针。
// format: 时间格式字符串。
// 返回值: 转换后的字符串指针，若 t 为 nil 则返回 nil。
func TimeValueToString(t *time.Time, format string) *string {
	if t != nil {
		s := t.Format(format)
		return &s
	}
	return nil
}

// StringValueToTime 将字符串指针转换为时间指针。
// t: 待转换的字符串指针。
// layout: 时间格式字符串。
// 返回值: 转换后的时间指针，若转换失败或 t 为 nil 则返回 nil。
func StringValueToTime(t *string, layout string) *time.Time {
	if t != nil {
		p, err := time.Parse(layout, *t)
		if err != nil {
			return nil
			// 可考虑使用更规范的日志记录方式，这里暂时注释掉日志输出
			// log.Println(err)
			return nil
		}
		return &p
	}
	return nil
}
