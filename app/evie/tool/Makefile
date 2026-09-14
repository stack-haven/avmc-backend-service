include ../../../app.mk

# 本 Makefile 只声明 evie/tool 特有的 target。
# 通用 target（init / build / test / lint / api / openapi / vet / cover / docker ...）
# 由 ../app.mk 提供。
#
# 必须覆盖的 app.mk target：
#  1. `run` — app.mk 用 `-conf ./configs` 目录加载所有 .yaml/.json，会触发 panic
#     （configs/tenants.json 是 JSON 数组，无法 merge 到 Bootstrap map）。
#     本服务的 run 默认用 ./configs/config.yaml。
#  2. `wire` — app.mk 用 `-mod=mod` 触发 workspace 冲突；本地用 GOWORK=off 绕开。

# ============================================================================
# 代码生成（服务特有）
# ============================================================================

.PHONY: config
# 生成 internal/conf/conf.pb.go（基于 internal/conf/conf.proto）
# 注：conf.pb.go 在 .gitignore 中（由本地 protoc 生成，不入库）
config:
	@protoc -I . --go_out=. --go_opt=paths=source_relative \
		./internal/conf/conf.proto
	@gofmt -w internal/conf/conf.pb.go
	@echo "✓ conf.pb.go regenerated"

.PHONY: wire
# 重新生成 cmd/server/wire_gen.go（已入库，CI 必须保证与 wire 输出一致）
# GOWORK=off 绕开 backend-service 的 go workspace（避免 `-mod=mod` 冲突）
wire:
	@GOWORK=off go run github.com/google/wire/cmd/wire ./cmd/server
	@echo "✓ wire_gen.go regenerated"

# ============================================================================
# 运行（核心）
# ============================================================================

.PHONY: run
# 直接运行服务（生产配置 ./configs/config.yaml）。
# 自动 source .env.local（gitignored；放凭据用）。
#
# 用法：
#   make run                                  # 默认 config.yaml
#   make run CONF=./configs/config.demo.yaml # 自定义配置
#   make run CONF=./configs/config.yaml      # 显式指定生产配置
run:
	@if [ -f .env.local ]; then \
		echo "▶ 已 source .env.local"; \
		set -a; . ./.env.local; set +a; \
	fi
	@CONF_FILE=$${CONF:-./configs/config.yaml}; \
	echo "▶ 使用配置: $$CONF_FILE"; \
	exec go run ./cmd/server -conf $$CONF_FILE

.PHONY: demo
# Demo 模式（零外部依赖：StaticProvider + FileSource）。
# 不连真实 Redis/qua/ASR；适合本地开发与 CI 烟雾测试。
demo:
	EVIE_TOOL_DEMO=1 go run ./cmd/server -conf ./configs/config.demo.yaml

.PHONY: build-run
# 编译后运行（生产配置）— 比 make run 快（避免每次 go run 都重编译）。
build-run:
	@mkdir -p bin
	@go build -o bin/evie-tool ./cmd/server
	@echo "▶ 运行 bin/evie-tool"
	@./bin/evie-tool -conf ./configs/config.yaml

# ============================================================================
# 运维
# ============================================================================

.PHONY: health
# 健康检查（live + ready）
health:
	@curl -sS http://127.0.0.1:8110/health/live | jq . 2>/dev/null || echo "(live OK if 200)"
	@curl -sS http://127.0.0.1:8110/health/ready | jq . 2>/dev/null || echo "(ready OK if 200)"

.PHONY: kill
# 杀掉运行中的 evie/tool 进程并释放端口
kill:
	@pkill -f "evie/tool/cmd/server" || true
	@for port in 8110 9110; do \
		lsof -ti :$$port 2>/dev/null | xargs -r kill -9 2>/dev/null || true; \
	done

# ============================================================================
# 测试 & 工具
# ============================================================================

.PHONY: race
# race detector（CI 必跑）
race:
	go test -race -timeout 240s \
		./internal/biz \
		./internal/data \
		./internal/server \
		./internal/service \
		./pkg/credential \
		./pkg/source

.PHONY: cli
# 构建运维 CLI（cmd/evie-toolctl）
cli:
	@mkdir -p bin
	@go build -o bin/evie-toolctl ./cmd/evie-toolctl
	@echo "✓ bin/evie-toolctl built"

.PHONY: doc
# 生成 OpenAPI v3 文档（Kratos assets embed）
doc:
	@cd ../../../proto && \
	buf generate --template evie/tool/v1/buf.openapi.gen.yaml 2>/dev/null || \
	echo "OpenAPI generation skipped"

.PHONY: bench
# 基准测试（含内存统计）
bench:
	go test -bench=. -benchmem -run=^$ ./...

.PHONY: generate-check
# CI 检查：generated code 与最新 wire/config 输出是否一致
# 注：wire 需要 GOWORK=off（避免 workspace 冲突），子 make 不会继承环境变量，显式传递
generate-check:
	GOWORK=off $(MAKE) wire
	@cd ../../.. && git diff --exit-code -- \
		app/evie/tool/cmd/server/wire_gen.go || \
		(echo "✗ wire_gen.go 已过时，请运行 make wire 并提交"; exit 1)
	@echo "✓ generate-check passed"
