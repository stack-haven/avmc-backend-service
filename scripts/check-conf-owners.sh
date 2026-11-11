#!/usr/bin/env bash
# check-conf-owners.sh
# 校验 `common/conf/*.proto` 在整仓只被**一个** Go 包注册（Stage 0-c）。
#
# ── 为什么需要这道门禁 ─────────────────────────────────────────────────────
# Go 的 protobuf 运行时维护一张**全局**注册表，键是 proto 文件路径。
# 同一份 `common/conf/base.proto` 若被两个 Go 包各自 init 注册，先注册者的
# 类型解析会被后注册者覆盖，protobuf-go 直接 panic：
#
#   panic: proto: file "common/conf/base.proto" is already registered
#
# 2026-11-11 之前本仓库有 5 个 Go 包注册同一批 conf proto（api/common/conf 加
# 4 个服务的 internal/conf 副本，实测 md5 全等）。现已收敛为唯一出口：
#
#   proto/common/conf/*.proto  --(make conf)-->  pkg/conf/*.pb.go
#
# 收敛后风险从「潜伏」变为「结构性不可能」，但它随时可能被重新引入 ——
# 最典型的路径是「某个服务为了自定义配置，又加了一份 conf.proto 副本」。
# 本门禁的作用就是让这种回退立即变红。
#
# baseline 保留棘轮机制：若确有新的注册点需要临时接受，必须显式登记
# 并写明原因与去向（与 check-import-boundaries.sh 同机制）。
#
# 兼容性：刻意不使用 bash 4 才有的 mapfile / declare -A
#   —— 本仓库开发者本机是 macOS 自带 bash 3.2，用了会在本地直接报错，
#   门禁只在 CI 能跑等于半个静默通过。
#
# 用法：./scripts/check-conf-owners.sh   （backend-service 根目录）
set -euo pipefail

cd "$(dirname "$0")/.."

BASELINE_FILE="scripts/conf-owner-baseline.txt"
# 唯一允许的注册点（唯一生成出口，见 pkg/conf/buf.gen.yaml）。
CANONICAL_OWNER="pkg/conf"
# 判定依据：生成物头部 `// source: <proto 路径>` 以此前缀开头即算 conf 注册点
CONF_SOURCE_PREFIX="common/conf/"

FILES_TMP="$(mktemp)"
OWNERS_TMP="$(mktemp)"
trap 'rm -f "${FILES_TMP}" "${OWNERS_TMP}"' EXIT

echo "==> Check: proto/${CONF_SOURCE_PREFIX}* must be registered by exactly one Go package"
echo "   允许的注册点：${CANONICAL_OWNER}"
echo

# 用 git ls-files 而非 find：避开 app/evie/tool(~1GB)、app/funasr(2.8GB) 与 node_modules
{
    git ls-files '*.pb.go'
    git ls-files --others --exclude-standard '*.pb.go'
} | sort -u >"${FILES_TMP}"

PB_TOTAL="$(grep -c . "${FILES_TMP}" || true)"
if [ "${PB_TOTAL:-0}" -eq 0 ]; then
    echo "❌ 未发现任何 *.pb.go —— 扫描路径失效，检查不可信（fail-closed）" >&2
    exit 1
fi

while IFS= read -r f; do
    [ -n "$f" ] || continue
    # 只读头部：proto 文件路径固定出现在前 10 行内
    src="$(head -10 "$f" 2>/dev/null | grep -m1 '^// source: ' | sed 's|^// source: ||')" || true
    case "${src:-}" in
        "${CONF_SOURCE_PREFIX}"*) dirname "$f" >>"${OWNERS_TMP}" ;;
    esac
done <"${FILES_TMP}"

sort -u "${OWNERS_TMP}" -o "${OWNERS_TMP}"
OWNER_TOTAL="$(grep -c . "${OWNERS_TMP}" || true)"
if [ "${OWNER_TOTAL:-0}" -eq 0 ]; then
    echo "❌ 未发现任何 ${CONF_SOURCE_PREFIX} 生成物 —— 检查不可信（fail-closed）" >&2
    exit 1
fi

# baseline：只对比目录列，`#` 注释与空行忽略
BASELINE_DIRS=""
if [ -f "$BASELINE_FILE" ]; then
    # `|| true` 必需：基线为空时 grep 退出码为 1，在 set -euo pipefail 下
    # 会让整个脚本静默退出（基线清零后立即踩到过这个坑）。
    BASELINE_DIRS="$(grep -vE '^[[:space:]]*(#|$)' "$BASELINE_FILE" | awk '{print $1}' || true)"
fi

echo "   发现 ${OWNER_TOTAL} 个注册点，扫描 ${PB_TOTAL} 个 pb.go"
echo

KNOWN=0
NEW_COUNT=0
NEW_LIST=""
while IFS= read -r dir; do
    [ -n "$dir" ] || continue
    if [ "$dir" = "$CANONICAL_OWNER" ]; then
        echo "   ✅ ${dir}（规范注册点）"
        continue
    fi
    if [ -n "$BASELINE_DIRS" ] && printf '%s\n' "$BASELINE_DIRS" | grep -Fxq "$dir"; then
        KNOWN=$((KNOWN + 1))
        echo "   ⏸  known: ${dir}"
    else
        NEW_COUNT=$((NEW_COUNT + 1))
        NEW_LIST="${NEW_LIST}${dir}"$'\n'
        echo "   ❌ NEW:   ${dir}"
    fi
done <"${OWNERS_TMP}"

echo
echo "── 汇总 ─────────────────────────────────────────────"
echo "   非规范注册点 $((OWNER_TOTAL - 1))（其中 baseline 存量 ${KNOWN}）"
echo "   新增注册点 ${NEW_COUNT}"

if [ "${NEW_COUNT}" -gt 0 ]; then
    echo
    echo "❌ 出现新的 conf 注册点："
    printf '%s' "${NEW_LIST}" | sed 's/^/   /'
    echo
    echo "   同一份 conf proto 被多个 Go 包注册会导致 protobuf 全局注册表冲突，"
    echo "   任一同时链接两边的二进制都会在 init 阶段 panic。"
    echo "   正确做法：删除该目录下的 conf.proto / buf.gen.yaml，直接 import ${CANONICAL_OWNER}。"
    echo "   若确需服务专属配置字段，应扩展 proto/common/conf 而不是复制一份。"
    exit 1
fi

echo
echo "✅ conf owner check passed（存量 ${KNOWN} 个注册点待收敛，见 ${BASELINE_FILE}）"