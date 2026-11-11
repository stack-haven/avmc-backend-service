package main

import (
	"context"

	"backend-service/app/edai/service/internal/data"
	"backend-service/app/edai/service/internal/data/ent/viewer"
	"backend-service/pkg/conf"

	"github.com/go-kratos/kratos/v2/log"
)

// recoverInterruptedTasks 在服务启动时把遗留的 running / cancel_requested 扫描任务标记失败。
//
// 独立于 wire 依赖图：任务恢复只需要 ent client，构建整棵依赖树没必要，
// 也避免 main 里出现手工 lambda（.agents/RULES.md 禁止）。
func recoverInterruptedTasks(bc *conf.Bootstrap, logger log.Logger) (int32, error) {
	if bc == nil || bc.Data == nil {
		return 0, nil
	}
	client, err := data.NewEntClient(bc.Data, logger)
	if err != nil {
		return 0, err
	}
	defer func() { _ = client.Close() }()

	// 系统上下文：跨租户收口，避免只恢复当前租户的遗留任务。
	ctx := viewer.NewSystemContext(context.Background())
	return data.RecoverInterruptedScanTasks(ctx, client)
}
