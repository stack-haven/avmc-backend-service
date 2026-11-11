# app/evie/service —— L2（仅声明 Evie 服务特有配置）
#
# 通用命令由 L1 `backend-service/app.mk` 提供。
# 本次改动顺带修复：原 `doc` 指向的 `evie/service/v1/buf.openapi.gen.yaml` 本来是对的，
# 现统一由 app.mk 的 `openapi` 按同一路径生成，行为不变。

include ../../../app.mk

# kill 需要释放的端口（evie: HTTP 8100 / gRPC 9100）
PORTS := 8100 9100
