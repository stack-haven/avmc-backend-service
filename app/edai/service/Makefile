# app/edai/service —— L2（仅声明 EDAI 服务特有配置）
#
# 通用命令由 L1 `backend-service/app.mk` 提供，见 docs/architecture/4-10-治理-命令契约.md。
#
# 说明：本文件为 2026-11-11 补齐。此前 edai 没有 Makefile，导致
#   - 无法使用服务级通用命令（make run / gen / test ...）
#   - 根目录的 serve-edai / migrate-edai 等委托命令会报 "No rule to make target"
# 该缺口由 scripts/gen-service-targets.py 的「Makefile 存在性校验」防止复发。

include ../../../app.mk

# kill 需要释放的端口（edai: HTTP 8200 / gRPC 9200，仅监听 127.0.0.1）
PORTS := 8200 9200
