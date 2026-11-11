#!/usr/bin/env bash
# check-data-tx-discipline.sh
#
# 校验 data 层事务纪律：仓储不得直接持有 Ent client。
#
# 背景（2026-11-12 P0-4a）：ai/chat_repo.go 的 CountChats/ListChats 直接用
# `r.data.db`，与同文件另外 9 处 `r.data.DB(ctx)` 不一致。当前 ai 的 InTx 无调用者
# 所以尚未出错，但一旦业务启用事务，这两个方法会读到事务外的数据（读不到未提交写入）。
# 这类缺陷 staticcheck 不报、测试也难覆盖，只能靠结构性门禁。
#
# 规则：
#   1. `data/` 目录下（排除 ent/ 生成代码与 _test.go）禁止出现 `.data.db` / `.Data.db`；
#      唯一允许直接持有 client 的文件是 data.go（Data 结构体定义处）。
#   2. 需要访问数据库时一律用 `DB(ctx)`，它会优先返回上下文中的事务 client。
#
# 基线为空：任何一处违规立即变红，不做白名单放行。

set -euo pipefail

cd "$(dirname "$0")/.."

violations=""

# 规则 1：仓储层绕过 DB(ctx) 直接取 client
while IFS= read -r line; do
	[ -z "$line" ] && continue
	file="${line%%:*}"
	# data.go 是 Data 结构体的唯一持有者，允许直接访问 client
	case "$file" in
	*/data.go) continue ;;
	esac
	violations="${violations}${line}
"
done <<EOF
$(grep -rn --include='*.go' -E '\.(data|Data)\.db[^a-zA-Z]' \
	app/*/service/internal/data 2>/dev/null \
	| grep -v '/ent/' \
	| grep -v '_test\.go:' \
	|| true)
EOF

if [ -n "$violations" ]; then
	echo "❌ data 层发现绕过事务的直连 client（应改用 DB(ctx)）："
	printf '%s' "$violations"
	echo
	echo "   DB(ctx) 会在存在事务时返回事务 client；直接用字段会读到事务外的连接。"
	exit 1
fi

echo "✅ data-tx-check passed：仓储层无绕过 DB(ctx) 的直连 client"