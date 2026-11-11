# =============================================================================
# 本文件由 scripts/gen-service-targets.py 自动生成，禁止手工修改。
# 数据源：backend-service/services.yaml
# 重新生成：make gen-servicetargets
# 校验漂移：make verify-servicetargets
# =============================================================================
#
# 这些是 L0（根 Makefile）的**委托命令**：只做 $(MAKE) -C <service> <target>，
# 真正的实现在 L1 backend-service/app.mk 与服务级 Makefile 中。
# 目的：让"在根目录就能操作任意服务"，同时不产生第二份实现。

.PHONY: services
# 列出所有后端服务及其端口
services:
	@printf "%-12s %-32s %-6s %-6s %-6s %-10s %s\n" \
		"SERVICE" "PATH" "HTTP" "GRPC" "SSE" "STATUS" "DB"
	@printf "%-12s %-32s %-6s %-6s %-6s %-10s %s\n" \
		"platform" "app/platform/service" "8000" "9000" "-" "active" "platform_system"
	@printf "%-12s %-32s %-6s %-6s %-6s %-10s %s\n" \
		"ai" "app/ai/service" "8001" "9001" "7001" "active" "platform_ai"
	@printf "%-12s %-32s %-6s %-6s %-6s %-10s %s\n" \
		"evie" "app/evie/service" "8100" "9100" "-" "active" "platform_evie"
	@printf "%-12s %-32s %-6s %-6s %-6s %-10s %s\n" \
		"edai" "app/edai/service" "8200" "9200" "-" "active" "sqlite"
	@printf "%-12s %-32s %-6s %-6s %-6s %-10s %s\n" \
		"version" "app/version/service" "8300" "9300" "-" "frozen" "platform_version"
	@printf "%-12s %-32s %-6s %-6s %-6s %-10s %s\n" \
		"evie-tool" "app/evie/tool" "8110" "9110" "-" "active" "none"

# ── serve-<service>：启动服务（前台） ──
.PHONY: serve-platform
# 启动服务（前台）：platform
serve-platform:
	@$(MAKE) --no-print-directory -C app/platform/service run

.PHONY: serve-ai
# 启动服务（前台）：ai
serve-ai:
	@$(MAKE) --no-print-directory -C app/ai/service run

.PHONY: serve-evie
# 启动服务（前台）：evie
serve-evie:
	@$(MAKE) --no-print-directory -C app/evie/service run

.PHONY: serve-edai
# 启动服务（前台）：edai
serve-edai:
	@$(MAKE) --no-print-directory -C app/edai/service run

.PHONY: serve-version
# 启动服务（前台）：version
serve-version:
	@$(MAKE) --no-print-directory -C app/version/service run

.PHONY: serve-evie-tool
# 启动服务（前台）：evie-tool
serve-evie-tool:
	@$(MAKE) --no-print-directory -C app/evie/tool serve

# ── gen-<service>：全量生成该服务的代码 ──
.PHONY: gen-platform
# 全量生成该服务的代码：platform
gen-platform:
	@$(MAKE) --no-print-directory -C app/platform/service gen

.PHONY: gen-ai
# 全量生成该服务的代码：ai
gen-ai:
	@$(MAKE) --no-print-directory -C app/ai/service gen

.PHONY: gen-evie
# 全量生成该服务的代码：evie
gen-evie:
	@$(MAKE) --no-print-directory -C app/evie/service gen

.PHONY: gen-edai
# 全量生成该服务的代码：edai
gen-edai:
	@$(MAKE) --no-print-directory -C app/edai/service gen

.PHONY: gen-version
# 全量生成该服务的代码：version
gen-version:
	@$(MAKE) --no-print-directory -C app/version/service gen

.PHONY: gen-evie-tool
# 全量生成该服务的代码：evie-tool
gen-evie-tool:
	@$(MAKE) --no-print-directory -C app/evie/tool gen

# ── build-<service>：构建该服务二进制 ──
.PHONY: build-platform
# 构建该服务二进制：platform
build-platform:
	@$(MAKE) --no-print-directory -C app/platform/service build

.PHONY: build-ai
# 构建该服务二进制：ai
build-ai:
	@$(MAKE) --no-print-directory -C app/ai/service build

.PHONY: build-evie
# 构建该服务二进制：evie
build-evie:
	@$(MAKE) --no-print-directory -C app/evie/service build

.PHONY: build-edai
# 构建该服务二进制：edai
build-edai:
	@$(MAKE) --no-print-directory -C app/edai/service build

.PHONY: build-version
# 构建该服务二进制：version
build-version:
	@$(MAKE) --no-print-directory -C app/version/service build

.PHONY: build-evie-tool
# 构建该服务二进制：evie-tool
build-evie-tool:
	@$(MAKE) --no-print-directory -C app/evie/tool build

# ── test-<service>：运行该服务单元测试 ──
.PHONY: test-platform
# 运行该服务单元测试：platform
test-platform:
	@$(MAKE) --no-print-directory -C app/platform/service test

.PHONY: test-ai
# 运行该服务单元测试：ai
test-ai:
	@$(MAKE) --no-print-directory -C app/ai/service test

.PHONY: test-evie
# 运行该服务单元测试：evie
test-evie:
	@$(MAKE) --no-print-directory -C app/evie/service test

.PHONY: test-edai
# 运行该服务单元测试：edai
test-edai:
	@$(MAKE) --no-print-directory -C app/edai/service test

.PHONY: test-version
# 运行该服务单元测试：version
test-version:
	@$(MAKE) --no-print-directory -C app/version/service test

.PHONY: test-evie-tool
# 运行该服务单元测试：evie-tool
test-evie-tool:
	@$(MAKE) --no-print-directory -C app/evie/tool test

# ── stop-<service>：停止该服务并释放端口 ──
.PHONY: stop-platform
# 停止该服务并释放端口：platform
stop-platform:
	@$(MAKE) --no-print-directory -C app/platform/service kill

.PHONY: stop-ai
# 停止该服务并释放端口：ai
stop-ai:
	@$(MAKE) --no-print-directory -C app/ai/service kill

.PHONY: stop-evie
# 停止该服务并释放端口：evie
stop-evie:
	@$(MAKE) --no-print-directory -C app/evie/service kill

.PHONY: stop-edai
# 停止该服务并释放端口：edai
stop-edai:
	@$(MAKE) --no-print-directory -C app/edai/service kill

.PHONY: stop-version
# 停止该服务并释放端口：version
stop-version:
	@$(MAKE) --no-print-directory -C app/version/service kill

.PHONY: stop-evie-tool
# 停止该服务并释放端口：evie-tool
stop-evie-tool:
	@$(MAKE) --no-print-directory -C app/evie/tool kill

# ── migrate-<service>：执行数据库迁移（仅 cmds 含 migrate 的服务）──
.PHONY: migrate-platform
# 执行数据库迁移：platform
migrate-platform:
	@$(MAKE) --no-print-directory -C app/platform/service migrate

.PHONY: migrate-ai
# 执行数据库迁移：ai
migrate-ai:
	@$(MAKE) --no-print-directory -C app/ai/service migrate

.PHONY: migrate-evie
# 执行数据库迁移：evie
migrate-evie:
	@$(MAKE) --no-print-directory -C app/evie/service migrate

.PHONY: migrate-edai
# 执行数据库迁移：edai
migrate-edai:
	@$(MAKE) --no-print-directory -C app/edai/service migrate

# ── seed-<service>：写入 mock / seed 数据（仅 cmds 含 mock 的服务）──
.PHONY: seed-platform
# 写入 mock / seed 数据：platform
seed-platform:
	@$(MAKE) --no-print-directory -C app/platform/service mock

.PHONY: seed-evie
# 写入 mock / seed 数据：evie
seed-evie:
	@$(MAKE) --no-print-directory -C app/evie/service mock

# ── policy-<service>：初始化 Casbin 策略（仅 cmds 含 policy 的服务）──
.PHONY: policy-platform
# 初始化 Casbin 策略：platform
policy-platform:
	@$(MAKE) --no-print-directory -C app/platform/service policy

.PHONY: policy-evie
# 初始化 Casbin 策略：evie
policy-evie:
	@$(MAKE) --no-print-directory -C app/evie/service policy

# ── 运行时辅助 ──
.PHONY: kill-all
# 停止全部服务并释放各自端口
kill-all:
	@$(MAKE) --no-print-directory stop-platform >/dev/null 2>&1 || true
	@$(MAKE) --no-print-directory stop-ai >/dev/null 2>&1 || true
	@$(MAKE) --no-print-directory stop-evie >/dev/null 2>&1 || true
	@$(MAKE) --no-print-directory stop-edai >/dev/null 2>&1 || true
	@$(MAKE) --no-print-directory stop-version >/dev/null 2>&1 || true
	@$(MAKE) --no-print-directory stop-evie-tool >/dev/null 2>&1 || true
	@echo "✅ 已尝试停止全部服务"
