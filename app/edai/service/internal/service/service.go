package service

import (
	"strconv"

	"github.com/google/wire"
)

// ProviderSet 服务层依赖注入集合。
var ProviderSet = wire.NewSet(
	NewAuthServiceService,
	NewSourceServiceService,
	NewFileServiceService,
	NewScanServiceService,
	NewSystemServiceService,
	NewSystemOptions,
)

// parseOffset 把前端传入的 page_token 解析为整数偏移。
// EDAI 的 Vben 表格使用页码式分页，因此 token 直接用 offset 的十进制串。
func parseOffset(token string) (int, error) {
	v, err := strconv.Atoi(token)
	if err != nil || v < 0 {
		return 0, err
	}
	return v, nil
}

func formatOffset(offset int) string {
	if offset < 0 {
		return "0"
	}
	return strconv.Itoa(offset)
}
