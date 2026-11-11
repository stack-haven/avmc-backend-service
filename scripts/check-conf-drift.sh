#!/usr/bin/env bash
# check-conf-drift.sh
# 校验各服务 `internal/conf/*.pb.go` 与仓库级 `api/common/conf/*.pb.go` 完全一致。
#
# ── 为什么需要这道门禁（Stage 0-a）───────────────────────────────────────────
# `proto/common/conf/*.proto` 是配置 schema 的唯一事实源，它有两套生成出口：
#
#   A. 根 proto/buf.gen.yaml        inputs: ./common      → api/common/conf/
#   B. 服务级 app/<svc>/buf.gen.yaml inputs: ./common/conf  → 服务 internal/conf/
#
# 出口 A 由 `make generate-check`（CI: backend-ci.yml）重新生成 + diff 校验；
# 出口 B 只在**服务级** `make generate-check` 里被 diff-check 覆盖，而 CI 从不按服务调用。
#
# 结果：改一个 conf proto → 本地/CI 跑根 `make proto` → api 更新、CI 绿 →
# 4 个服务的 internal/conf 全部静默陈旧，配置读不到新字段，无任何红灯。
# 这是本项目"静默通过"的第 10 例。
#
# ── 为什么用「与 api 逐字节比对」而不是「重新生成再 diff」──────────────────
# 1. 生成物是确定性的：5 个出口独立生成过同一份 conf，12 个文件 md5 全等。
# 2. 不依赖 buf / 网络（服务级模板用的是 `remote: buf.build/...` 远程插件），
#    因此本门禁在离线、无 buf 的环境同样可信。
# 3. 出口 A 的"是否与 proto 同步"已由根 `make generate-check` 保证 ——
#    本脚本只需证明 B 与 A 相等，即可传递得到 B 与 proto 同步。
#
# ⚠️ 依赖执行顺序：CI 中本步骤必须排在 `make generate-check` **之后**，
#    否则拿到的可能是陈旧的 api 基准（详见 backend-ci.yml 的步骤顺序）。
#
# 用法：./scripts/check-conf-drift.sh   （backend-service 根目录）
set -euo pipefail

cd "$(dirname "$0")/.."

# 仓库级生成出口（基准）。若未来把 conf 迁到 pkg/conf，只需改这两个变量。
CANONICAL_DIR="${CONF_CANONICAL_DIR:-api/common/conf}"

FAIL=0
SCANNED=0
SKIPPED=0
MISMATCH=0

if [ ! -d "${CANONICAL_DIR}" ]; then
    echo "❌ 基准目录 ${CANONICAL_DIR}/ 不存在 —— 无法判定 conf 漂移（fail-closed）" >&2
    exit 1
fi

shopt -s nullglob
canonical=( "${CANONICAL_DIR}"/*.pb.go )
shopt -u nullglob
if [ "${#canonical[@]}" -eq 0 ]; then
    echo "❌ 基准目录 ${CANONICAL_DIR}/ 下没有 *.pb.go —— 检查不可信（fail-closed）" >&2
    exit 1
fi

canonical_names=()
for f in "${canonical[@]}"; do canonical_names+=("$(basename "$f")"); done

echo "==> Check: internal/conf must stay in sync with ${CANONICAL_DIR}"
echo "   基准 ${#canonical[@]} 个 *.pb.go"
echo

# 固定深度 glob：避免 find 递归 app/evie/tool(~1GB) 与 app/funasr(2.8GB)
for dir in app/*/service/internal/conf; do
    [ -d "$dir" ] || continue

    shopt -s nullglob
    local_files=( "${dir}"/*.pb.go )
    shopt -u nullglob

    if [ "${#local_files[@]}" -eq 0 ]; then
        # 类型别名层（如 edai 的 conf/doc.go），不持有生成物 → 不参与比对
        echo "   ⏭  ${dir}：无 *.pb.go（别名层），跳过"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    SCANNED=$((SCANNED + 1))
    dir_bad=0

    for name in "${canonical_names[@]}"; do
        if [ ! -f "${dir}/${name}" ]; then
            echo "   ❌ ${dir}/${name} 缺失（基准中存在）"
            dir_bad=1
            continue
        fi
        if ! cmp -s "${CANONICAL_DIR}/${name}" "${dir}/${name}"; then
            echo "   ❌ ${dir}/${name} 与基准不一致"
            dir_bad=1
        fi
    done

    # 反向：本地多出来的 pb.go（proto 新增文件只生成到一边）
    for f in "${local_files[@]}"; do
        name="$(basename "$f")"
        hit=0
        for cn in "${canonical_names[@]}"; do
            [ "$cn" = "$name" ] && { hit=1; break; }
        done
        if [ "$hit" -eq 0 ]; then
            echo "   ❌ ${dir}/${name} 在基准中不存在"
            dir_bad=1
        fi
    done

    if [ "$dir_bad" -eq 0 ]; then
        echo "   ok  ${dir}"
    else
        MISMATCH=$((MISMATCH + 1))
        FAIL=1
    fi
done

echo
echo "── 汇总 ─────────────────────────────────────────────"
echo "   比对目录 ${SCANNED}（另跳过别名层 ${SKIPPED}）"
echo "   漂移目录 ${MISMATCH}"

if [ "$FAIL" -eq 1 ]; then
    echo
    echo "❌ conf 漂移：服务的 internal/conf 与 ${CANONICAL_DIR} 不一致。"
    echo "   修复方式（在对应服务目录内）："
    echo "     make config      # 重新生成并安装 internal/conf"
    echo "   若确认是有意分化（例如服务级模板单独升降 protoc-gen-go），"
    echo "   必须先在本脚本中显式登记例外，禁止直接改基准。"
    exit 1
fi

echo
echo "✅ conf drift check passed."