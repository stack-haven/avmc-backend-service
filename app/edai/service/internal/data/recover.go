package data

import (
	"context"
	"time"

	"backend-service/app/edai/service/internal/biz"
	"backend-service/app/edai/service/internal/data/ent/gen"
	"backend-service/app/edai/service/internal/data/ent/gen/scantask"
)

// RecoverInterruptedScanTasks 把遗留的 running / cancel_requested 扫描任务标记失败。
//
// 暴露为包级函数（而非 repo 方法）：启动阶段的恢复发生在依赖图之外，
// 只需要 ent client。业务侧仍走 ScanTaskRepo.RecoverInterrupted。
func RecoverInterruptedScanTasks(ctx context.Context, client *gen.Client) (int32, error) {
	n, err := client.ScanTask.Update().
		Where(scantask.StatusIn(biz.ScanStatusRunning, biz.ScanStatusCancelRequested)).
		SetStatus(biz.ScanStatusFailed).
		SetErrorMessage("服务重启导致扫描中断，请重新发起").
		SetFinishedAt(time.Now()).
		Save(ctx)
	return int32(n), err
}
