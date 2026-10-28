#!/usr/bin/env bash
# operation-coverage-check — 校验「受 Casbin 保护的 operation」都有可达的授权路径。
#
# 背景（2026-10-24 系统性审计）：Casbin 策略由角色的菜单按钮（auth_code = proto
# Operation 常量）生成（见 internal/data/role_policy_sync.go）。若某个受保护
# operation 在 authzpolicy 中声明、却没有对应菜单按钮，则该端点对**所有角色**
# 恒返回 403。历史上因此有 40 个端点（含大量「详情」接口）不可达。
#
# 判定：受保护 operation（authzpolicy/policy.go）
#       减去 白名单（server.go 的 anthn/anthz 白名单，认证鉴权跳过）
#       减去 菜单按钮（cmd/mock/*.go 的 buttonSpec.Operation）
#       减去 显式忽略清单（见下方 IGNORE）
#       必须为空。
#
# 用法：make operation-coverage-check
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - <<'PY'
import glob
import re
import sys

# 显式忽略：已确认「无需菜单按钮」的受保护 operation。
# 每项都必须写明原因，禁止无理由堆积。
IGNORE = {
    # 设备管理：暂无前端页面/消费方（system_devices 表存在，功能未接线）。
    # 若后续加「设备管理」页，请补按钮并从此清单移除。
    "/platform.service.v1.DeviceService/ListMyDevices",
    "/platform.service.v1.DeviceService/ListUserDevices",
    "/platform.service.v1.DeviceService/RegisterDevice",
    "/platform.service.v1.DeviceService/UnregisterDevice",
}


def resolve_symbols():
    """常量符号 -> operation 字符串。"""
    mapping = {}
    for pattern in ("api/**/*_http.pb.go", "api/**/*_grpc.pb.go"):
        for path in glob.glob(pattern, recursive=True):
            text = open(path, encoding="utf-8").read()
            for m in re.finditer(
                r'\b(Operation\w+|\w+Service_\w+_FullMethodName)\s*=\s*"([^"]+)"',
                text,
            ):
                mapping[m.group(1)] = m.group(2)
    return mapping


sym2str = resolve_symbols()

policy = open(
    "app/platform/service/internal/authzpolicy/policy.go", encoding="utf-8"
).read()
protected = {
    sym2str[m.group(1)]
    for m in re.finditer(r"op\(\s*v1\.(Operation\w+)\s*,", policy)
    if m.group(1) in sym2str
}

server = open(
    "app/platform/service/internal/server/server.go", encoding="utf-8"
).read()
whitelisted = set()
for m in re.finditer(
    r"v1\.(Operation\w+|\w+Service_\w+_FullMethodName)\s*:\s*true", server
):
    if m.group(1) in sym2str:
        whitelisted.add(sym2str[m.group(1)])

buttons = set()
for path in glob.glob("app/platform/service/cmd/mock/*.go"):
    text = open(path, encoding="utf-8").read()
    for m in re.finditer(r"Operation:\s*v1\.(Operation\w+)", text):
        if m.group(1) in sym2str:
            buttons.add(sym2str[m.group(1)])

missing = sorted(protected - whitelisted - buttons - IGNORE)
if missing:
    print("❌ 受保护 operation 缺少菜单按钮（将对所有角色恒 403）：")
    for op in missing:
        print(f"   - {op}")
    print()
    print("修复：在 app/platform/service/cmd/mock/seed_platform.go 的 platformButtons")
    print("     中为该 operation 增加按钮；若确属内部/未接线，请在脚本 IGNORE 中")
    print("     登记并说明原因。")
    sys.exit(1)

print(
    f"✅ operation 覆盖完整：受保护 {len(protected)} = "
    f"白名单 {len(whitelisted & protected)} + 按钮 {len(buttons & protected)} + "
    f"忽略 {len(IGNORE & protected)}"
)
PY