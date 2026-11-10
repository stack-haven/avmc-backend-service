package data

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	pbenum "backend-service/api/common/enum"
	"backend-service/app/edai/service/internal/data/ent/gen"
	"backend-service/pkg/utils/crypto"

	"github.com/go-kratos/kratos/v2/log"
)

// seedAdminUser 首次启动时创建默认管理员（仅当用户表为空）。
//
// 口令来源优先级：EDAI_ADMIN_PASSWORD > 随机生成并打印一次。
// **绝不使用硬编码默认密码**——EDAI 会在客户机器上以 loopback 运行，
// 固定口令等于把本机数据放在公开入口后面。
func seedAdminUser(ctx context.Context, client *gen.Client, l *log.Helper) error {
	count, err := client.User.Query().Count(entSystemContext(ctx))
	if err != nil {
		return fmt.Errorf("counting users: %w", err)
	}
	if count > 0 {
		return nil
	}

	password := strings.TrimSpace(os.Getenv("EDAI_ADMIN_PASSWORD"))
	generated := false
	if password == "" {
		raw := make([]byte, 18)
		if _, err := rand.Read(raw); err != nil {
			return fmt.Errorf("generating admin password: %w", err)
		}
		password = base64.RawURLEncoding.EncodeToString(raw)
		generated = true
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing admin password: %w", err)
	}

	_, err = client.User.Create().
		SetUsername("admin").
		SetDisplayName("管理员").
		SetRole("admin").
		SetStatus(int32(pbenum.Status_STATUS_ENABLED)).
		SetPasswordHash(hash).
		SetTenantID(1).
		Save(entSystemContext(ctx))
	if err != nil {
		return fmt.Errorf("creating admin user: %w", err)
	}

	if generated {
		l.Warnf("已创建默认管理员 admin，初始口令（仅显示一次，请立即修改）：%s", password)
	} else {
		l.Infof("已创建默认管理员 admin（口令来自 EDAI_ADMIN_PASSWORD）")
	}
	return nil
}
