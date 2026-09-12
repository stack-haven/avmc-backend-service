#!/usr/bin/env bash
# check-coverage.sh
# 统计 backend-service 关键包覆盖率并与 docs/architecture/4-5 §二 门禁对比。
#
# 用法：
#   ./scripts/check-coverage.sh          # 默认：仅生成报告，不阻断
#   FAIL_ON_GAP=1 ./scripts/check-coverage.sh   # 低于门禁的包会让脚本 exit 1（CI 门禁模式）
#
# 阈值与 4-5 §二 一致：
#   biz=70%  data=60%  service=50%  pkg=45%

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

# 门禁阈值
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
  echo "Mode: $([ "${FAIL_ON_GAP:-0}" = "1" ] && echo 'CI GATE (exit 1 on gap)' || echo 'REPORT ONLY')"
  echo
  printf '%-70s %8s %8s\n' "PACKAGE" "COV%" "STATUS"
  printf '%-70s %8s %8s\n' "-------" "----" "------"
} > "$SUMMARY"

FAIL=0
BELOW_THRESHOLD=()
for pkg in "${PKGS[@]}"; do
  # go test -cover 输出最后一行含 "coverage: NN.N% of statements"
  # 取最后一个匹配项并去掉 % 字符
  cov="$(go test -cover -timeout 60s -coverprofile="$OUT_DIR/$(echo "$pkg" | tr '/' '_').out" "$pkg" 2>/dev/null \
    | grep -Eo 'coverage: [0-9.]+% of statements' \
    | tail -1 \
    | awk '{print $2}' \
    | tr -d '%')"
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
    if [ "$(printf '%.0f' "$cov")" -ge "$thr" ]; then
      status="ok"
    else
      status="< ${thr}%"
      BELOW_THRESHOLD+=("$pkg (${cov}% < ${thr}%)")
      FAIL=1
    fi
  fi
  printf '%-70s %8s %8s\n' "$pkg" "$cov" "$status" >> "$SUMMARY"
done

cat "$SUMMARY"
echo
echo "Report: $SUMMARY"

if [ "$FAIL" -eq 1 ]; then
  echo
  echo "⚠️  Below threshold (${#BELOW_THRESHOLD[@]} package(s)):"
  for p in "${BELOW_THRESHOLD[@]}"; do
    echo "   - $p"
  done
  if [ "${FAIL_ON_GAP:-0}" = "1" ]; then
    echo
    echo "❌ FAIL_ON_GAP=1 set, exiting 1"
    exit 1
  else
    echo
    echo "ℹ️  Report-only mode; set FAIL_ON_GAP=1 to enforce as CI gate."
  fi
fi
