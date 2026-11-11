# app/version/service —— L2（仅声明历史版本服务特有配置）
#
# 服务状态：🔴 冻结（见 docs/architecture/4-6）
# 通用命令由 L1 `backend-service/app.mk` 提供。
#
# 本次改动：
#   1. 删除死目标 `config1`（`cd../../../proto` 缺空格 + 使用 sudo，从未可执行）
#   2. `doc` 原先指向不存在的 `platform/admin/v1/buf.openapi.gen.yaml`；
#      本服务无 `version/service/v1/buf.openapi.gen.yaml`，app.mk 会跳过并提示
#   3. 端口从 8000/9000 改为 8300/9300，消除与 platform 的冲突（已确认）

include ../../../app.mk

PORTS := 8300 9300
