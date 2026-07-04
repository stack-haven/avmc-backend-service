package data

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"backend-service/app/platform/admin/internal/data/ent/gen"

	"github.com/go-kratos/kratos/v2/log"
)

// BaseRepo 仓库基类 — 持有公共依赖，减少每个子 repo 重复的 Data/Log 声明
type BaseRepo struct {
	Data *Data
	Log  *log.Helper
}

func isSelectForUpdateUnsupported(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOR UPDATE/SHARE not supported")
}

// NewBaseRepo 创建 BaseRepo
func NewBaseRepo(data *Data, logger log.Logger) BaseRepo {
	return BaseRepo{
		Data: data,
		Log:  log.NewHelper(logger),
	}
}

// ConvertSlice 泛型切片转换 — 消除每个 repo 的 for 循环
func ConvertSlice[E, D any](src []*E, fn func(*E) *D) []*D {
	if src == nil {
		return nil
	}
	dst := make([]*D, len(src))
	for i, v := range src {
		dst[i] = fn(v)
	}
	return dst
}

// NowPtr 返回当前时间指针（软删除用）
func NowPtr() *time.Time {
	t := time.Now()
	return &t
}

func rollback(tx *gen.Tx, logger *log.Helper) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logger.Errorf("rollback transaction failed: %v", err)
	}
}
