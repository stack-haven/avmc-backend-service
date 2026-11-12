package data

import (
	"context"
	"database/sql"
	"strconv"
	"testing"

	"backend-service/app/ai/service/internal/data/ent/gen"
	"backend-service/app/ai/service/internal/data/ent/gen/migrate"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/entgo/viewer"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/glebarez/sqlite"
)

// testSecurityUser 是测试用的最小 SecurityUser 实现，用于把租户/用户身份
// 通过 authn.ContextWithAuthUser 注入 ctx（与生产中间件行为一致）。
type testSecurityUser struct {
	userID   uint32
	tenantID uint32
}

func (u testSecurityUser) Name() string                           { return "test" }
func (u testSecurityUser) ParseFromContext(context.Context) error { return nil }
func (u testSecurityUser) GetSubject() string                     { return strconv.FormatUint(uint64(u.userID), 10) }
func (u testSecurityUser) GetObject() string                      { return "" }
func (u testSecurityUser) GetAction() string                      { return "" }
func (u testSecurityUser) GetTenant() string {
	return strconv.FormatUint(uint64(u.tenantID), 10)
}

// systemContext 返回绕过租户隐私过滤的上下文（迁移、种子、跨租户运维场景）。
func systemContext() context.Context {
	return viewer.NewSystemContext(context.Background())
}

// tenantContext 返回绑定指定租户的上下文，同时注入 Ent viewer 与认证用户，
// 覆盖生产链路中"中间件注入租户"的真实形态。
func tenantContext(tenantID uint32) context.Context {
	ctx := viewer.NewTenantContext(context.Background(), tenantID)
	return authn.ContextWithAuthUser(ctx, testSecurityUser{userID: 1, tenantID: tenantID})
}

// newTestClientWithDB 创建进程内 SQLite 的 Ent 客户端并返回底层 *sql.DB。
// 返回 *sql.DB 是为了在需要绕过 Ent 校验器直接改写数据时使用
// （例如模拟 DDL 加列时数据库以 0 填充的历史行）。
func newTestClientWithDB(t *testing.T) (*gen.Client, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := gen.NewClient(gen.Driver(drv))
	tables, err := schema.CopyTables(migrate.Tables)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Create(context.Background(), client.Schema, tables); err != nil {
		t.Fatal(err)
	}
	return client, db
}
