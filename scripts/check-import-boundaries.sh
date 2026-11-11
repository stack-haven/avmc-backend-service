#!/usr/bin/env bash
# check-import-boundaries.sh
# 校验 backend-service 的 import 边界是否符合「service → biz → data」分层
# 与 `.agents/RULES.md §依赖方向硬规则` 对齐。
#
# 用法：./scripts/check-import-boundaries.sh   （在 backend-service 根目录下执行）
#
# ── 2026-11-11 修复（A 案 Phase 1）─────────────────────────────────────────
# 旧版有两处致命缺陷，导致本检查**从未真正检查过任何文件**：
#   1. 扫描路径写成 `app/*/internal/biz`、`app/*/internal/service`，
#      而真实路径是 `app/<svc>/service/internal/biz`（多一层 `service/`），
#      glob 匹配 0 个目录 → grep 无输入 → 恒打印 ok。
#   2. 「扫到 0 个目录」不视为失败，于是失效无法被发现。
#
# 因此本版新增两条硬约束：
#   A. 目录发现失败（扫描目录数 = 0）→ 直接判定失败。
#   B. 存量违例走 baseline 棘轮（scripts/import-boundary-baseline.txt），
#      只阻断**新增**违例；baseline 中每条必须写明原因与去向。
#
# 注意：`app/evie/service/internal/service/asr_service.go` 的存量违例
# 已被记入 baseline（A 案非目标：不修改 evie 代码）。
# ──────────────────────────────────────────────────────────────────────────

set -euo pipefail

cd "$(dirname "$0")/.."

BASELINE_FILE="scripts/import-boundary-baseline.txt"
FAIL=0
SCANNED_DIRS=0
SCANNED_FILES=0
VIOLATIONS_TOTAL=0
VIOLATIONS_KNOWN=0
NEW_VIOLATIONS=()

# 反向依赖模式（严格对齐 `.agents/RULES.md` §依赖方向硬规则）：
#   规则原文只禁止 `service → biz → data` 的下行 import 与 ent 耦合，
#   **未**规定禁止 import `internal/conf`。
# 因此：
#   - 失败条件：import internal/data 或 ent / ent/gen
#   - 信息性统计：import internal/conf（若要升级为硬规则，须先改 RULES.md，
#     属 requires-confirmation 变更，不在 A 案范围内擅自扩大门禁）
REVERSE_DEPS='app/[a-z0-9_-]+/(service|tool)/internal/data|/ent(/gen)?["/]'
# conf 依赖同时覆盖两种形态：
#   pkg/conf                                   — 2026-11-12 起的唯一共享配置出口
#   app/<svc>/(service|tool)/internal/conf      — 历史副本；evie/tool 仍保留自有的
#                                                 internal/conf（package kratos.api）
CONF_IMPORT='pkg/conf|app/[a-z0-9_-]+/(service|tool)/internal/conf'

# 统一 grep 选项：只读 Go 源码，跳过运行期大目录。
# 不排除时 platform/service（377MB，其中 bin/ 218MB、upload/ 110MB）
# 与 evie/tool（~1GB）会把本检查拖到 40s+。
GREP_OPTS=(--include='*.go' --exclude-dir=bin --exclude-dir=upload
           --exclude-dir=data --exclude-dir=testdata --exclude-dir=assets
           --exclude-dir=node_modules --binary-files=without-match)

load_baseline() {
  if [ -f "$BASELINE_FILE" ]; then
    grep -vE '^\s*(#|$)' "$BASELINE_FILE" | awk '{print $1}'
  fi
}

is_known() {
  local file="$1"
  printf '%s\n' "$BASELINE_FILES" | grep -Fxq "$file"
}

scan_layer() {
  local layer="$1"   # biz | service
  local allow_reverse="$2"  # 1 = 该层不得反向依赖

  echo "==> Check: internal/${layer} must not import data / conf / ent ..."

  local dirs=()
  # 用固定深度的 glob 发现目录，避免 `find app -type d` 递归遍历
  # app/evie/tool（近 1GB）与 app/funasr（2.8GB 模型）导致分钟级耗时。
  for d in app/*/service/internal/"${layer}" app/*/tool/internal/"${layer}"; do
    [ -d "$d" ] && dirs+=("$d")
  done

  if [ "${#dirs[@]}" -eq 0 ]; then
    echo "❌ 未发现任何 internal/${layer} 目录 —— 路径规则失效，检查不可信"
    FAIL=1
    return
  fi

  local n_files
  n_files=$(find "${dirs[@]}" -name '*.go' 2>/dev/null | wc -l | tr -d ' ')
  echo "   扫描目录 ${#dirs[@]} 个 / Go 文件 ${n_files} 个"
  SCANNED_DIRS=$((SCANNED_DIRS + ${#dirs[@]}))
  SCANNED_FILES=$((SCANNED_FILES + n_files))

  if [ "$allow_reverse" != "1" ]; then
    echo "   ok（该层允许依赖 data）"
    return
  fi

  # 只检查生产代码：`*_test.go` 允许直接依赖 data 实现
  # （测试需要装配具体实现；RULES 的依赖方向规则约束的是生产代码）
  local hits
  hits=$(grep -rEn "$REVERSE_DEPS" "${GREP_OPTS[@]}" --exclude='*_test.go' "${dirs[@]}" 2>/dev/null || true)

  # 信息性：conf 依赖（非硬规则，仅统计）
  #
  # `|| true` 必需：conf 依赖清零后 `grep` 因无匹配返回 1，在 `set -euo pipefail`
  # 下整个脚本会**静默退出**（只打印到一半、无任何错误说明，CI 上表现为裸 Error 1）。
  # 这是 2026-11-12 把 conf 收敛到 pkg/conf 后暴露出来的既有缺陷。
  local conf_count
  conf_count=$(grep -rEl "$CONF_IMPORT" "${GREP_OPTS[@]}" --exclude='*_test.go' "${dirs[@]}" 2>/dev/null | wc -l | tr -d ' ' || true)
  if [ "${conf_count:-0}" -gt 0 ]; then
    echo "   ℹ️  conf 依赖 ${conf_count} 个文件（信息性，非硬规则）"
  fi

  if [ -z "$hits" ]; then
    echo "   ok"
    return
  fi

  local found=0
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    local file="${line%%:*}"
    found=1
    VIOLATIONS_TOTAL=$((VIOLATIONS_TOTAL + 1))
    if is_known "$file"; then
      VIOLATIONS_KNOWN=$((VIOLATIONS_KNOWN + 1))
      echo "   ⏸  known: $line"
    else
      echo "   ❌ NEW:   $line"
      NEW_VIOLATIONS+=("$line")
    fi
  done <<< "$hits"

  if [ "$found" -eq 0 ]; then
    echo "   ok"
  fi

  # 必须显式返回 0：否则函数最后一条语句（[ ] 判定为假）会在 set -e 下
  # 让整个脚本静默提前退出 —— 这是本项目历史上出现过的同类缺陷
  return 0
}

# ──────────────────────────────────────────────────────────────────────────
echo "==> 加载存量违例基线：$BASELINE_FILE"
BASELINE_FILES="$(load_baseline || true)"
echo "   baseline 条目：$(printf '%s\n' "$BASELINE_FILES" | grep -c . || true)"

scan_layer biz 1
echo
scan_layer service 1
echo

# 3) 跨产品服务不得直接 import 其它产品的 internal
echo "==> Check: product service must not import other product internal ..."
SVC_DIRS=()
for d in app/*/service; do
  [ -d "$d" ] || continue
  # 只纳入真正的 Go 服务：必须有 cmd/ 目录。
  # 排除 app/funasr/service（Python 服务，带 2.8GB 模型目录，递归 grep 会拖死 CI）。
  [ -d "$d/cmd" ] || continue
  SVC_DIRS+=("$d")
done
if [ "${#SVC_DIRS[@]}" -eq 0 ]; then
  echo "❌ 未发现任何 app/*/service 目录 —— 路径规则失效"
  FAIL=1
else
  echo "   扫描产品服务 ${#SVC_DIRS[@]} 个"
  cross_ok=1
  for src in "${SVC_DIRS[@]}"; do
    src_name="$(basename "$(dirname "$src")")"
    for dst in "${SVC_DIRS[@]}"; do
      dst_name="$(basename "$(dirname "$dst")")"
      [ "$src_name" = "$dst_name" ] && continue
      if grep -rEn "app/${dst_name}/service/internal" "${GREP_OPTS[@]}" "$src" 2>/dev/null; then
        echo "❌ $src_name 直接 import $dst_name/internal"
        FAIL=1
        cross_ok=0
      fi
    done
  done
  [ "$cross_ok" -eq 1 ] && echo "   ok"
fi
echo

# 4) ent/gen 信息性统计
echo "==> Check: ent/gen is generated; manual edits should be limited ..."
# 同样避免全仓 find（会遍历 funasr 模型与 evie/tool 大目录）
shopt -s nullglob
GENERATED=0
for f in app/*/service/internal/data/ent/gen/*.go app/*/tool/internal/data/ent/gen/*.go; do
  GENERATED=$((GENERATED + 1))
done
shopt -u nullglob
echo "   $GENERATED generated files under */ent/gen/ (informational)"
echo

echo "── 汇总 ─────────────────────────────────────────────"
echo "   扫描目录 ${SCANNED_DIRS} / 文件 ${SCANNED_FILES}"
echo "   违规总数 ${VIOLATIONS_TOTAL}（其中 baseline 存量 ${VIOLATIONS_KNOWN}）"
echo "   新增违规 ${#NEW_VIOLATIONS[@]}"

if [ "${#NEW_VIOLATIONS[@]}" -gt 0 ]; then
  echo
  echo "❌ 新增分层违例："
  printf '   %s\n' "${NEW_VIOLATIONS[@]}"
fi

if [ "$FAIL" -eq 1 ] || [ "${#NEW_VIOLATIONS[@]}" -gt 0 ]; then
  echo
  echo "❌ Import boundary check failed."
  exit 1
fi
echo
echo "✅ Import boundary check passed."
