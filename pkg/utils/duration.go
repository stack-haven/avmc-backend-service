package utils

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

// Duration 将 protobuf Duration 转为 time.Duration。
// d 为 nil 或非正值时返回 fallback，避免各服务重复实现。
func Duration(d *durationpb.Duration, fallback time.Duration) time.Duration {
	if d == nil {
		return fallback
	}
	v := d.AsDuration()
	if v <= 0 {
		return fallback
	}
	return v
}
