# app/evie/tool —— L2（evie 轻量语音识别增强工具）
#
# 通用命令（run / gen / api / wire / config / openapi / test / lint / docker ...）
# 由 L1 `backend-service/app.mk` 提供。
#
# 本文件只保留「本服务独有」的部分：
#   - 端口与进程模式（8110 / 9110）
#   - serve / demo / build-serve（本服务的配置加载方式与 app.mk 的 run 不同）
#   - cli / bench / health（专用工具与运维命令）
#   - config（本服务用 protoc 直出，与其它服务的 buf 方案不同）
#   - docker-build / push / release / test（cnb 制品库发布链）
#
# 2026-11-11（A 案 D6-8）清理：
#   - 删除 `wire` 覆写：app.mk 已统一使用 GOWORK=off，覆写不再必要
#   - 删除 `doc` 覆写：本服务无 openapi 模板，app.mk 会自动跳过
#   - 删除 `kill` 覆写：改用 app.mk 的 kill + PORTS/KILL_PATTERN
#   - `race` 从覆写 target 改为声明 RACE_PKGS 变量

# 本服务自带 config（protoc 直出）与 generate-check（只校验 wire），
# 在 include 前声明跳过 app.mk 的同名定义，避免 make overriding 警告。
SKIP_APP_MK_CONFIG := 1
SKIP_APP_MK_GENERATE_CHECK := 1

include ../../../app.mk

# ── 本服务特有配置 ─────────────────────────────────────────────────────────
PORTS        := 8110 9110
KILL_PATTERN := evie/tool
RACE_PKGS    := ./internal/biz ./internal/data ./internal/server ./internal/service ./pkg/credential ./pkg/source

# ============================================================================
# 配置生成（本服务用 protoc 直出 internal/conf/conf.pb.go，不入库）
# ============================================================================

.PHONY: config
config:
	@protoc -I . --go_out=. --go_opt=paths=source_relative \
		./internal/conf/conf.proto
	@gofmt -w internal/conf/conf.pb.go
	@echo "✅ internal/conf/conf.pb.go regenerated"

# ============================================================================
# 运行
# ============================================================================

.PHONY: serve
# 直接运行服务（生产配置 ./configs/config.yaml）。
# 自动 source .env.local（gitignored；放凭据用）。
#
# 为什么本服务用 `serve` 而不是 app.mk 的 `run`：
#   app.mk 的 `run` 以 `-conf ./configs`（目录）加载，本服务的 runtimeconfig
#   会把 configs/tenants.json 当配置 merge，触发 panic。故本服务固定传文件路径。
#
# 用法：
#   make serve                                  # 默认 config.yaml
#   make serve CONF=./configs/config.demo.yaml # 自定义配置
serve:
	@if [ -f .env.local ]; then \
		echo "▶ 已 source .env.local"; \
		set -a; . ./.env.local; set +a; \
	fi
	@CONF_FILE=$${CONF:-./configs/config.yaml}; \
	echo "▶ 使用配置: $$CONF_FILE"; \
	exec go run ./cmd/server -conf $$CONF_FILE

.PHONY: demo
# Demo 模式（零外部依赖：StaticProvider + FileSource）
demo:
	@EVIE_TOOL_DEMO=1 go run ./cmd/server -conf ./configs/config.demo.yaml

.PHONY: build-serve
# 编译后运行（比 make serve 快，避免每次 go run 重编译）
build-serve:
	@mkdir -p bin
	@go build -o bin/evie-tool ./cmd/server
	@echo "▶ 运行 bin/evie-tool"
	@./bin/evie-tool -conf ./configs/config.yaml

# ============================================================================
# 运维 / 工具
# ============================================================================

.PHONY: health
health:
	@curl -sS http://127.0.0.1:8110/health/live | jq . 2>/dev/null || echo "(live OK if 200)"
	@curl -sS http://127.0.0.1:8110/health/ready | jq . 2>/dev/null || echo "(ready OK if 200)"

.PHONY: cli
# 构建运维 CLI（cmd/evie-toolctl）
cli:
	@mkdir -p bin
	@go build -o bin/evie-toolctl ./cmd/evie-toolctl
	@echo "✅ bin/evie-toolctl built"

.PHONY: bench
# 基准测试（含内存统计）
bench:
	@go test -bench=. -benchmem -run=^$$ ./...

.PHONY: generate-check
# CI 检查：wire_gen.go 与最新 wire 输出一致
generate-check:
	@$(MAKE) --no-print-directory wire
	@cd ../../.. && git diff --exit-code -- \
		app/evie/tool/cmd/server/wire_gen.go || \
		(echo "❌ wire_gen.go 已过时，请运行 make wire 并提交"; exit 1)
	@echo "✅ generate-check passed"

# ============================================================================
# 容器镜像（Docker + cnb 制品库）
# ============================================================================
#
# 用法：
#   make docker-build                            # 构建镜像（local，3 个 tag）
#   make docker-push                             # 推送 latest + version + sha 到 cnb
#   make docker-release                          # 一键 build + push
#   make docker-test                             # 本地容器起服务做健康检查
#
# 镜像：stack-haven/evie-tool:<tag> / docker.cnb.cool/beiduoke/iaas/evie-tool:<tag>

.PHONY: docker-build
docker-build:
	@echo "▶ 构建 evie/tool 容器镜像"
	@VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	SHA=$$(git rev-parse --short HEAD); \
	echo "  VERSION: $$VERSION"; \
	echo "  COMMIT:  $$SHA"; \
	docker build -f Dockerfile.evie-tool \
		--build-arg VERSION=$$VERSION \
		-t stack-haven/evie-tool:latest \
		-t stack-haven/evie-tool:$$VERSION \
		-t stack-haven/evie-tool:$$SHA \
		$(CURDIR)/../../..
	@echo "✅ 构建完成"
	@docker images | grep "stack-haven/evie-tool" | head -5

.PHONY: docker-push
docker-push:
	@VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	SHA=$$(git rev-parse --short HEAD); \
	IMAGE_BASE=docker.cnb.cool/beiduoke/iaas/evie-tool; \
	echo "▶ 打远端 tag"; \
	docker tag stack-haven/evie-tool:latest $$IMAGE_BASE:latest; \
	docker tag stack-haven/evie-tool:$$VERSION $$IMAGE_BASE:$$VERSION; \
	docker tag stack-haven/evie-tool:$$SHA $$IMAGE_BASE:$$SHA; \
	echo "▶ 推送 latest"; docker push $$IMAGE_BASE:latest; \
	echo "▶ 推送 $$VERSION"; docker push $$IMAGE_BASE:$$VERSION; \
	echo "▶ 推送 $$SHA"; docker push $$IMAGE_BASE:$$SHA; \
	echo "✅ 远端镜像已更新"

.PHONY: docker-release
docker-release: docker-build docker-push
	@echo "✅ 发布完成"

.PHONY: docker-test
# 本地容器运行 + 健康检查（需 docker daemon，占用 8110/9110）
docker-test:
	@echo "▶ 启动容器（host network，redis 走 host IP）"
	@docker rm -f evie-tool-test 2>/dev/null || true
	@docker run -d \
		--name evie-tool-test \
		--network host \
		-v $(CURDIR)/configs:/data/conf:ro \
		-v $(CURDIR)/upload:/data/upload:rw \
		stack-haven/evie-tool:latest \
		-conf /data/conf/config.yaml
	@sleep 5
	@curl -fsS http://127.0.0.1:8110/health/ready | python3 -m json.tool || \
		(echo "❌ 健康检查失败"; docker logs evie-tool-test --tail 30; exit 1)
	@echo "✅ 容器内服务运行正常（logs: docker logs evie-tool-test）"
