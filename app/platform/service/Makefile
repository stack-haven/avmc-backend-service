# app/platform/service —— L2（仅声明平台服务特有配置）
#
# 通用命令（run / gen / api / ent / wire / config / openapi / migrate / mock /
# policy / test / lint / race / docker / diff-check / generate-check ...）
# 全部由 L1 `backend-service/app.mk` 提供，见 docs/architecture/4-10-治理-命令契约.md。
#
# 本文件只允许出现「与平台服务绑定」的内容。

include ../../../app.mk

# kill 需要释放的端口（platform: HTTP 8000 / gRPC 9000）
PORTS := 8000 9000
