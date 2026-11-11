# app/ai/service —— L2（仅声明 AI 服务特有配置）
#
# 通用命令由 L1 `backend-service/app.mk` 提供。
# 本次改动顺带修复：原 `doc` 指向不存在的 `platform/ai/v1/buf.openapi.gen.yaml`，
# 现由 app.mk 的 `openapi` 按 `ai/service/v1/buf.openapi.gen.yaml` 生成。

include ../../../app.mk

# kill 需要释放的端口（ai: HTTP 7001 / 内部 8001 / gRPC 9001）
PORTS := 7001 8001 9001
