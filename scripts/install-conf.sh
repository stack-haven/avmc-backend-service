#!/usr/bin/env bash
# install-conf.sh
# 把 buf 生成的 proto/common/conf 产物安装到当前服务的 internal/conf。
#
# 背景（见 app.mk:135 `config` 目标）：
#   服务级 buf.gen.yaml 只声明 `inputs: ./common/conf`，
#   而 `paths=source_relative` 会生成 `<svc>/proto/common/conf/*.pb.go`。
#   为了让 import 路径是 `internal/conf`（而不是 `internal/common/conf`），
#   历史上用 `mv proto/common/conf internal` 一次性搬走 —— 顺带 `rm -rf internal/conf`。
#
# 为什么这个脚本取代了裸 `rm -rf` + `mv`：
#   `rm -rf internal/conf` 会静默删除该目录下**一切非生成文件**。
#   实际存在这类文件：app/edai/service/internal/conf/{doc.go,provider.go}
#   （类型别名层，`provider.go` 还是 wire ProviderSet）。
#   一旦给 edai 补上 buf.gen.yaml 并跑 `make config`，这些手写文件会消失，
#   而且因为 mv 会成功，命令仍然打印 ✅ —— 典型的静默破坏。
#
# 现在的语义：
#   1. 只删 `internal/conf/*.pb.go`（生成物），不碰任何手写文件；
#   2. internal/conf 删空则整体搬入（保持历史行为）；
#   3. internal/conf 仍有手写文件则就地覆盖生成物（合并）；
#   4. 无论哪条路径，最后都清掉临时 `<svc>/proto` 目录。
#
# 用法：在服务目录内执行（cwd 决定安装目标）
#   cd app/platform/service && ../../../scripts/install-conf.sh
#
# 退出码：0 成功；非 0 失败（生成物缺失 / 搬运失败），由调用方 make 目标透传。

set -euo pipefail

GENERATED_DIR="proto/common/conf"
TARGET_DIR="internal/conf"

if [ ! -d "${GENERATED_DIR}" ]; then
    echo "❌ 未找到生成物目录 ${GENERATED_DIR}/（buf generate 未产出？）" >&2
    echo "   当前目录：$(pwd)" >&2
    exit 1
fi

shopt -s nullglob
generated=( "${GENERATED_DIR}"/*.pb.go )
shopt -u nullglob
if [ "${#generated[@]}" -eq 0 ]; then
    echo "❌ ${GENERATED_DIR}/ 下没有 *.pb.go，拒绝安装" >&2
    exit 1
fi

# 1) 只清理旧生成物，保留手写文件
if [ -d "${TARGET_DIR}" ]; then
    find "${TARGET_DIR}" -maxdepth 1 -name '*.pb.go' -delete
fi

# 2) 目录已空 → 整体搬入；仍有手写文件 → 合并覆盖
if [ -d "${TARGET_DIR}" ] && [ -n "$(find "${TARGET_DIR}" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
    echo "ℹ️  ${TARGET_DIR}/ 含手写文件，改为就地覆盖生成物（手写文件保留）"
    cp -R "${GENERATED_DIR}/." "${TARGET_DIR}/"
    rm -rf proto
else
    [ -d "${TARGET_DIR}" ] && rmdir "${TARGET_DIR}"
    mv "${GENERATED_DIR}" "${TARGET_DIR}"
    rm -rf proto
fi

echo "✅ conf installed → ${TARGET_DIR}（${#generated[@]} 个 pb.go）"