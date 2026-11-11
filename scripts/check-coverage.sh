#!/usr/bin/env bash
# check-coverage.sh
# 统计 backend-service 关键分组的覆盖率并与门禁 / 基线对比。
#
# 用法：
#   ./scripts/check-coverage.sh                 # 报告 + 基线回归检查（fail-closed）
#   FAIL_ON_GAP=1 ./scripts/check-coverage.sh   # 额外启用绝对阈值阻断
#   ./scripts/check-coverage.sh --init-baseline # 重新生成基线（棘轮）
#
# 阈值（docs/architecture/4-5 §二）：
#   biz=70%  data=60%  service=50%  pkg=45%
#
# ── 2026-11-11 重写（A 案 Phase 5）────────────────────────────────────────
# 旧实现有两个致命缺陷，导致覆盖率数字不可信：
#   1. 用 `go test -cover ./x/... | grep ... | tail -1` 取值——
#      取到的是**最后一个子包**的覆盖率，不是聚合值。
#      实测 platform/data 被报成 0.0%，真实聚合是 1.4%。
#   2. 分母包含 `*/ent/gen/*` 与 `api/` 等生成代码（数千条语句、0 覆盖），
#      把业务代码的真实覆盖率稀释到接近 0。
#   3. 取不到覆盖率时 status=skip 且不置失败 → 门禁在"什么都没测"时是绿的。
#
# 现实现：一次 `go test -coverprofile` 全量跑，用 profile 聚合，
# 显式排除生成代码；分组无任何语句 → 判定失败（fail-closed）。
# ─────────────────────────────────────────────────────────────────────────

set -euo pipefail

cd "$(dirname "$0")/.."

OUT_DIR="${OUT_DIR:-./bin/coverage}"
BASELINE_FILE="scripts/coverage-baseline.txt"
mkdir -p "$OUT_DIR"

PROFILE="$OUT_DIR/coverage.out"
SUMMARY="$OUT_DIR/coverage-summary.txt"

# 绝对阈值（4-5 §二）；仅在 FAIL_ON_GAP=1 时阻断
THRESHOLD_BIZ=70
THRESHOLD_DATA=60
THRESHOLD_SERVICE=50
THRESHOLD_PKG=45

# 基线允许的回退容差（百分点）
TOLERANCE=1

echo "==> Running tests with coverage ..."
if ! go test -count=1 -timeout 300s -coverprofile="$PROFILE" ./... >"$OUT_DIR/test.log" 2>&1; then
  echo "❌ 测试失败，覆盖率不可信。输出摘要："
  tail -20 "$OUT_DIR/test.log"
  exit 1
fi

if [ ! -s "$PROFILE" ]; then
  echo "❌ 未生成覆盖率 profile（$PROFILE）—— 视为门禁失败，不允许静默跳过"
  exit 1
fi

INIT_BASELINE=0
[ "${1:-}" = "--init-baseline" ] && INIT_BASELINE=1

# 分组聚合（排除生成代码）
python3 - "$PROFILE" "$SUMMARY" "$THRESHOLD_BIZ" "$THRESHOLD_DATA" \
         "$THRESHOLD_SERVICE" "$THRESHOLD_PKG" "$TOLERANCE" \
         "$BASELINE_FILE" "$INIT_BASELINE" "${FAIL_ON_GAP:-0}" <<'PY'
import os
import re
import sys

(profile, summary, thr_biz, thr_data, thr_service, thr_pkg, tolerance,
 baseline_file, init_baseline, fail_on_gap) = sys.argv[1:]
tolerance = float(tolerance)

# 分组定义：顺序即报告顺序
GROUPS = [
    ("app/platform/service/internal/biz",     "biz",     int(thr_biz)),
    ("app/platform/service/internal/data",    "data",    int(thr_data)),
    ("app/platform/service/internal/service", "service", int(thr_service)),
    ("app/ai/service/internal/biz",           "biz",     int(thr_biz)),
    ("app/ai/service/internal/data",          "data",    int(thr_data)),
    ("app/evie/service/internal/biz",         "biz",     int(thr_biz)),
    ("app/evie/service/internal/data",        "data",    int(thr_data)),
    ("pkg/auth",                              "pkg",     int(thr_pkg)),
    ("pkg/aip/listing",                       "pkg",     int(thr_pkg)),
    ("pkg/health",                            "pkg",     int(thr_pkg)),
]

# 生成代码前缀：计入分母只会稀释真实覆盖率
GENERATED = ("/ent/gen/", "/api/", ".pb.go", "_grpc.pb.go", "_http.pb.go")

statements = {g[0]: 0 for g in GROUPS}
covered = {g[0]: 0 for g in GROUPS}
skipped_files = 0

for line in open(profile, encoding="utf-8"):
    line = line.strip()
    if not line or line.startswith("mode:"):
        continue
    # 格式：file:startLine.startCol,endLine.endCol numStmt count
    m = re.match(r"^(.*?):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$", line)
    if not m:
        continue
    path, numstmt, count = m.group(1), int(m.group(6)), int(m.group(7))
    # go 的 coverprofile 路径带模块前缀（backend-service/），统一剥掉再分组匹配
    if path.startswith("backend-service/"):
        path = path[len("backend-service/"):]
    if any(gen in path for gen in GENERATED) or path.startswith("api/"):
        skipped_files += 1
        continue
    for prefix, _kind, _thr in GROUPS:
        if path.startswith(prefix + "/"):
            statements[prefix] += numstmt
            if count > 0:
                covered[prefix] += numstmt
            break

def pct(prefix):
    s = statements[prefix]
    return None if s == 0 else round(100.0 * covered[prefix] / s, 1)

# 基线
baseline = {}
if os.path.exists(baseline_file):
    for line in open(baseline_file, encoding="utf-8"):
        line = line.split("#")[0].strip()
        if not line:
            continue
        parts = line.split()
        if len(parts) >= 2:
            try:
                baseline[parts[0]] = float(parts[1])
            except ValueError:
                pass

rows = []
hard_fail = []
below_threshold = []
regression = []
unmeasured = []

for prefix, kind, thr in GROUPS:
    p = pct(prefix)
    if p is None:
        unmeasured.append(prefix)
        rows.append((prefix, "n/a", "NO-STATEMENTS"))
        continue
    status = "ok"
    if p < thr:
        status = f"< {thr}%"
        below_threshold.append(f"{prefix} ({p}% < {thr}%)")
    if prefix in baseline and p < baseline[prefix] - tolerance:
        status = f"REGRESSED (-{baseline[prefix] - p:.1f})"
        regression.append(f"{prefix} ({p}% < baseline {baseline[prefix]}%)")
    rows.append((prefix, f"{p}", status))

# 输出
lines = []
lines.append("Ark Tech Platform backend-service coverage summary")
lines.append(f"Thresholds(biz/data/service/pkg): {thr_biz}/{thr_data}/{thr_service}/{thr_pkg}%  tolerance: {tolerance}pp")
lines.append(f"Mode: {'GATE' if fail_on_gap == '1' else 'RATCHET+REPORT'}")
lines.append(f"Excluded generated statements blocks: {skipped_files}")
lines.append("")
lines.append(f"{'GROUP':<44}{'COV%':>8}{'STATUS':>18}")
lines.append("-" * 70)
for prefix, cov, status in rows:
    lines.append(f"{prefix:<44}{cov:>8}{status:>18}")

if init_baseline == "1":
    with open(baseline_file, "w", encoding="utf-8") as f:
        f.write("# 覆盖率基线（棘轮）：只允许上升，下降超过容差即失败\n")
        f.write("# 生成：./scripts/check-coverage.sh --init-baseline\n")
        f.write("# 生成代码（ent/gen、api、*.pb.go）不计入分母\n")
        f.write(f"# 更新时间：{__import__('datetime').date.today()}\n")
        for prefix, cov, _ in rows:
            if cov != "n/a":
                f.write(f"{prefix} {cov}\n")

print("\n".join(lines))
with open(summary, "w", encoding="utf-8") as f:
    f.write("\n".join(lines) + "\n")

if unmeasured:
    print()
    print("❌ 以下分组没有任何语句被测量（路径失效或测试未运行）：")
    for u in unmeasured:
        print(f"   - {u}")

if regression:
    print()
    print("❌ 覆盖率相对基线回退：")
    for r in regression:
        print(f"   - {r}")

if below_threshold:
    print()
    print(f"⚠️  低于绝对阈值（{len(below_threshold)} 个分组）：")
    for b in below_threshold:
        print(f"   - {b}")

if init_baseline == "1":
    print()
    print(f"✅ 基线已写入 {baseline_file}")
    raise SystemExit(0)

if unmeasured or regression:
    raise SystemExit(1)

if below_threshold and fail_on_gap == "1":
    print()
    print("❌ FAIL_ON_GAP=1：低于绝对阈值，退出 1")
    raise SystemExit(1)

if below_threshold:
    print()
    print("ℹ️  绝对阈值未达标但设为报告模式；基线棘轮已生效。设 FAIL_ON_GAP=1 可阻断。")
PY
