#!/usr/bin/env bash
#
# HTTP 路径规范合规检查（AIP-136 对齐）
#
# 依据: docs/architecture/3-4-跨领域-HTTP-API设计规范.md
#
# 检查规则:
#   1. 禁止动作前置反模式:  /{resource}/{verb}/{id}  （如 /users/status-update/{id}）
#   2. 禁止斜杠动作:        /{resource}/{id}/{verb}  （如 /files/{id}/replace）
#   3. 禁止服务名重复:      /{service}/v1/{service}/ （如 /ai/v1/ai/）
#   4. 自定义动作必须用冒号: /{resource}/{id}:{verb} 或 /{resource}:{verb}
#
# 用法:
#   ./scripts/check-http-path-convention.sh          # 检查所有 proto
#   ./scripts/check-http-path-convention.sh --fix    # 仅报告（不支持自动修复）
#
# 退出码: 0 = 通过, 1 = 发现违规

set -euo pipefail

PROTO_DIR="${PROTO_DIR:-$(cd "$(dirname "$0")/../proto" && pwd)}"
FAILED=0

# 需要检查的自定义动作词（这些词作为路径最后一段时，应为 :verb 而非 /verb）
CUSTOM_VERBS=(
  "replace" "confirm" "complete" "abort" "cancel" "retry" "consume" "release"
  "publish" "rollback" "send" "read" "download" "content" "stats" "test"
  "set-default" "reset-password" "status-update" "delete-impact"
  "transfer-and-delete" "re-recognize" "recognize" "correct" "stream"
)

echo "==> HTTP 路径规范检查 (AIP-136)"
echo "    目录: $PROTO_DIR"

check_paths() {
  local paths
  # 提取所有 google.api.http 注解中的路径（get:/post:/put:/delete:/patch: 后面的引号内容）
  paths=$(grep -rhoE '"(/admin/v1|/evie/v1|/ai/v1|/version/v1)[^"]*"' "$PROTO_DIR" \
    --include='*.proto' 2>/dev/null | tr -d '"' | sort -u || true)

  # 规则 1: 动作前置反模式 /{resource}/{verb}/{id}
  # 例: /users/status-update/{id} → 应为 /users/{id}:status-update
  for verb in "${CUSTOM_VERBS[@]}"; do
    local matches
    matches=$(echo "$paths" | grep -E "/${verb}/\{[a-z_]+\}$" || true)
    if [ -n "$matches" ]; then
      while IFS= read -r p; do
        echo "  ❌ 动作前置反模式: $p （应为 .../{id}:${verb}）"
        FAILED=1
      done <<< "$matches"
    fi
  done

  # 规则 2: 斜杠动作 /{resource}/{id}/{verb}
  # 例: /files/{id}/replace → 应为 /files/{id}:replace
  for verb in "${CUSTOM_VERBS[@]}"; do
    local matches
    matches=$(echo "$paths" | grep -E "/\{[a-z_]+\}/${verb}$" || true)
    if [ -n "$matches" ]; then
      while IFS= read -r p; do
        echo "  ❌ 斜杠动作: $p （应为 ...:{verb}）"
        FAILED=1
      done <<< "$matches"
    fi
  done

  # 规则 3: 服务名重复 /{service}/v1/{service}/
  # 例: /ai/v1/ai/chats → 应为 /ai/v1/chats
  local dup
  dup=$(echo "$paths" | grep -oE '/(admin|evie|ai|version)/v1/\1/' || true)
  if [ -n "$dup" ]; then
    echo "  ❌ 服务名重复: $dup （应去掉重复段）"
    FAILED=1
  fi
}

check_paths

if [ "$FAILED" -eq 0 ]; then
  echo "✅ 所有 HTTP 路径符合 AIP-136 规范"
  exit 0
else
  echo ""
  echo "❌ 发现 HTTP 路径规范违规，请参考:"
  echo "   docs/architecture/3-4-跨领域-HTTP-API设计规范.md"
  exit 1
fi
