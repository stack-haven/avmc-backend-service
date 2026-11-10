package biz

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/wire"
)

// ProviderSet 业务层依赖注入集合。
var ProviderSet = wire.NewSet(
	NewAuthUsecase,
	NewSourceUsecase,
	NewFileUsecase,
	NewScanUsecase,
	ProvideScanLimits,
	ProvideAccessExpiration,
)

// ProvideAccessExpiration 提供访问令牌有效期。
//
// 由 main 侧按配置覆盖；默认 7 天与 pkg/auth 的默认值一致。
func ProvideAccessExpiration() time.Duration {
	return 7 * 24 * time.Hour
}

// 扫描资源上限的环境变量名。
//
// 放在 biz 层读环境变量而不是走 conf 包：biz 不允许 import conf（分层硬规则），
// 而这两个值只影响扫描的并发/限速，属于运行时调参，不需要进配置文件。
const (
	envScanWorkers        = "EDAI_SCAN_WORKERS"
	envScanFilesPerSecond = "EDAI_SCAN_FILES_PER_SECOND"
)

// ProvideScanLimits 从环境变量读取扫描并发与限速。
//
// 缺省值见 DefaultScanWorkers / DefaultScanFilesPerSecond：
// 4 个 worker + 500 文件/秒，既能明显快于串行，又不会把远端 SMB 打满。
// 设 EDAI_SCAN_FILES_PER_SECOND=0 表示不限速（仅建议本机 SSD 使用）。
func ProvideScanLimits() ScanLimits {
	limits := ScanLimits{
		Workers:        DefaultScanWorkers,
		FilesPerSecond: DefaultScanFilesPerSecond,
	}
	if v := strings.TrimSpace(os.Getenv(envScanWorkers)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limits.Workers = n
		}
	}
	if v := strings.TrimSpace(os.Getenv(envScanFilesPerSecond)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			// 显式 0 = 不限速（scanWorker 用 <0 表示不限速，这里做一次映射）。
			if n == 0 {
				limits.FilesPerSecond = -1
			} else {
				limits.FilesPerSecond = n
			}
		}
	}
	return limits
}

// Transaction 事务抽象，实现放在 data 层。
type Transaction interface {
	InTx(context.Context, func(ctx context.Context) error) error
}
