#!/usr/bin/env bash
# check-coverage.sh
# 统计 backend-service 关键包覆盖率并与 4-5 §二 门禁对比。仅做报告，不阻断 PR。
# 用法：./scripts/check-coverage.sh  （在 backend-service 根目录下执行）

set -euo pipefail

cd "$(dirname "$0")/.."

OUT_DIR="${OUT_DIR:-./bin/coverage}"
mkdir -p "$OUT_DIR"

PKGS=(
  "./app/platform/service/internal/biz/..."
  "./app/platform/service/internal/data/..."
  "./app/platform/service/internal/service/..."
  "./app/ai/service/internal/biz/..."
  "./app/ai/service/internal/data/..."
  "./app/evie/service/internal/biz/..."
  "./app/evie/service/internal/data/..."
  "./pkg/auth/..."
  "./pkg/aip/listing/..."
  "./pkg/health/..."
)

# 门禁阈值（与 4-5 §二 对齐）
THRESHOLD_BIZ=70
THRESHOLD_DATA=60
THRESHOLD_SERVICE=50
THRESHOLD_PKG=45

# 1) 收集整体覆盖率
PROFILE="$OUT_DIR/coverage.out"
echo "==> Running tests with coverage ..."
go test -cover -timeout 120s -coverprofile="$PROFILE" ./... >/dev/null

# 2) 按包汇总
SUMMARY="$OUT_DIR/coverage-summary.txt"
{
  echo "Ark Tech Platform backend-service coverage summary"
  echo "Generated: $(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  echo "Thresholds: biz=${THRESHOLD_BIZ}% data=${THRESHOLD_DATA}% service=${THRESHOLD_SERVICE}% pkg=${THRESHOLD_PKG}%"
  echo
  printf '%-70s %8s %8s\n' "PACKAGE" "COV%" "STATUS"
  printf '%-70s %8s %8s\n' "-------" "----" "------"
} > "$SUMMARY"

FAIL=0
for pkg in "${PKGS[@]}"; do
  cov="$(go test -cover -timeout 60s -coverprofile="$OUT_DIR/$(echo "$pkg" | tr '/' '_').out" "$pkg" 2>/dev/null | awk -F'[: \t]+' '/coverage:/ {gsub("%","",$NF); print $NF}')"
  if [ -z "${cov:-}" ]; then
    cov="n/a"
    status="skip"
  else
    case "$pkg" in
      */biz/*)        thr=$THRESHOLD_BIZ ;;
      */data/*)       thr=$THRESHOLD_DATA ;;
      */service/*)    thr=$THRESHOLD_SERVICE ;;
      *)              thr=$THRESHOLD_PKG ;;
    esac
    if [ "$(printf '%.0f' "$cov")" -ge "$thr" ]; then status="ok"; else status="< ${thr}%"; FAIL=1; fi
  fi
  printf '%-70s %8s %8s\n' "$pkg" "$cov" "$status" >> "$SUMMARY"
done

cat "$SUMMARY"
echo
echo "Report: $SUMMARY"
if [ "$FAIL" -eq 1 ]; then
  echo "⚠️  Some packages are below the threshold (see 4-5 §二)."
fi
