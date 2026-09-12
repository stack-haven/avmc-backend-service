#!/usr/bin/env bash
# check-import-boundaries.sh
# 检查 backend-service 的 import 边界是否符合「service → biz → data」分层
# 与 `.agents/RULES.md §依赖方向硬规则` 对齐。
#
# 用法：./scripts/check-import-boundaries.sh  （在 backend-service 根目录下执行）

set -euo pipefail

cd "$(dirname "$0")/.."

FAIL=0

# 1) biz 包不得 import data / ent / ent/gen
echo "==> Check: internal/biz must not import data or ent ..."
if grep -rEn 'app/[a-z0-9_-]+/internal/(data|conf)|/ent(/gen)?["/]' \
     --include='*.go' \
     app/*/internal/biz 2>/dev/null; then
  echo "❌ biz 包发现反向依赖 data / conf / ent"
  FAIL=1
else
  echo "   ok"
fi

# 2) service 包不得 import data / ent
echo "==> Check: internal/service must not import data or ent ..."
if grep -rEn 'app/[a-z0-9_-]+/internal/(data|conf)|/ent(/gen)?["/]' \
     --include='*.go' \
     app/*/internal/service 2>/dev/null; then
  echo "❌ service 包发现反向依赖 data / conf / ent"
  FAIL=1
else
  echo "   ok"
fi

# 3) 跨产品服务不得直接 import 其它产品的 internal
echo "==> Check: product service must not import other product internal ..."
SVC_DIRS=()
for d in app/*/service; do
  [ -d "$d" ] || continue
  SVC_DIRS+=("$d")
done
for src in "${SVC_DIRS[@]}"; do
  src_name="$(basename "$(dirname "$src")")"
  for dst in "${SVC_DIRS[@]}"; do
    dst_name="$(basename "$(dirname "$dst")")"
    [ "$src_name" = "$dst_name" ] && continue
    if grep -rEn "app/${dst_name}/service/internal" \
         --include='*.go' \
         "$src" 2>/dev/null; then
      echo "❌ $src_name 直接 import $dst_name/internal"
      FAIL=1
    fi
  done
done
echo "   ok"

# 4) ent/gen 不得手工编辑
echo "==> Check: ent/gen is generated; manual edits should be limited ..."
GENERATED=$(find . -path './.git' -prune -o -path '*/ent/gen/*.go' -print | wc -l | tr -d ' ')
echo "   $GENERATED generated files under */ent/gen/ (informational)"

if [ "$FAIL" -eq 1 ]; then
  echo
  echo "❌ Import boundary check failed."
  exit 1
fi
echo
echo "✅ Import boundary check passed."
