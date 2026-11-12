package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"backend-service/app/ai/service/internal/data"
	"backend-service/app/ai/service/internal/runtimeconfig"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	flagconf         string
	flagLegacyTenant uint
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
	flag.UintVar(&flagLegacyTenant, "legacy-tenant", 0,
		"把历史对话数据（tenant_id=0）归属到该租户 ID；不传则只统计不修改")
}

func main() {
	if err := run(); err != nil {
		// 命令入口用 log.Fatalf 而非 panic：panic 会打印栈并破坏 forbidigo 规则
		//（biz/data 层禁止 panic），而这里需要的是「非零退出 + 一行原因」。
		log.Fatalf("migrate: %v", err)
	}
}

func run() error {
	flag.Parse()
	logger := log.NewStdLogger(os.Stdout)
	l := log.NewHelper(log.With(logger, "module", "cmd/migrate"))
	ctx := context.Background()

	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	if err := data.RunSchemaMigration(ctx, bc.Data, logger); err != nil {
		return err
	}
	l.Info("schema migration finished")

	// ai 在 2026-11-12 之前没有租户维度。补齐后，早期数据的 tenant_id 会是 0，
	// 被隐私规则永久过滤（数据还在，但对所有租户不可见）。这属于"需要用人工决策
	// 收敛的数据状态"，因此必须显式处理，不能静默放过。
	client := data.NewEntClient(bc.Data, logger)
	defer func() {
		if err := client.Close(); err != nil {
			l.Errorf("closing ent client: %v", err)
		}
	}()

	if flagLegacyTenant > 0 {
		n, err := data.AssignLegacyTenant(ctx, client, uint32(flagLegacyTenant))
		if err != nil {
			return err
		}
		l.Infof("把 %d 行历史对话数据归属到租户 %d", n, flagLegacyTenant)
	}

	unowned, err := data.CountUnownedChats(ctx, client)
	if err != nil {
		return err
	}
	if unowned > 0 {
		return fmt.Errorf("仍有 %d 行对话数据未归属租户（tenant_id=0），它们对所有租户都不可见；"+
			"请用 -legacy-tenant=<租户ID> 明确归属后重跑 migrate", unowned)
	}
	l.Info("对话数据租户归属检查通过")
	return nil
}
