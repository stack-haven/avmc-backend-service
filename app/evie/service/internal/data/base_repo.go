package data

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/pkg/auth/authn"

	kerrors "github.com/go-kratos/kratos/v2/errors"
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

func rollback(tx *gen.Tx, logger *log.Helper) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logger.Errorf("rollback transaction failed: %v", err)
	}
}

// RequireTenantID extracts the tenant ID from the context. Returns Forbidden if missing.
func (BaseRepo) RequireTenantID(ctx context.Context) (uint32, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	if tenantID == 0 {
		return 0, kerrors.Forbidden("TENANT_CONTEXT_REQUIRED", "缺少有效的数据租户上下文")
	}
	return tenantID, nil
}

// RequireUserID extracts the user ID from the context. Returns Forbidden if missing.
func (BaseRepo) RequireUserID(ctx context.Context) (uint32, error) {
	userID := authn.GetAuthUserID(ctx)
	if userID == 0 {
		return 0, kerrors.Forbidden("USER_CONTEXT_REQUIRED", "缺少有效的数据用户上下文")
	}
	return userID, nil
}

// MapNotFound wraps Ent's IsNotFound with a kratos NotFound error. Returns the original error unchanged if not found.
func (BaseRepo) MapNotFound(err error, code, msg string) error {
	if gen.IsNotFound(err) {
		return kerrors.NotFound(code, msg)
	}
	return err
}

// MapConstraint wraps Ent's IsConstraintError with a kratos Conflict error. Returns the original error unchanged if not a constraint violation.
func (BaseRepo) MapConstraint(err error, code, msg string) error {
	if gen.IsConstraintError(err) {
		return kerrors.Conflict(code, msg)
	}
	return err
}
