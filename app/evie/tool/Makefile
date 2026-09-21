include ../../../app.mk

# 本 Makefile 只声明 evie/tool 特有的 target。
# 通用 target（init / build / test / lint / api / openapi / vet / cover / docker / help ...）
# 由 ../app.mk 提供。
#
# ============================================================================
# 预期警告（1 个，可安全忽略）
# ============================================================================
# 运行任何 target 都会出现：
#   Makefile:N: warning: overriding commands for target 'wire'
#   ../../../app.mk:M: warning: ignoring old commands for target 'wire'
#
# 原因：app.mk 的 `wire` 用 `go run -mod=mod ...`，在 go workspace 模式下必然失败：
#   "go: -mod may only be set to readonly or vendor when in workspace mode"
# 本地覆写为 `GOWORK=off` 绕开。
# （evie/service 也有同样问题，但未覆写，所以 `make wire` 实际不可用）
#
# GNU make 的 "overriding commands" 警告无法静默。这是必要的设计权衡：
#   - 接受 1 个警告 + `make wire` 可用（本服务选择）
#   - 0 警告 + `make wire` 不可用（evie/service 的隐性 bug）
# ============================================================================
#
# 目标命名映射：
#   - app.mk 的 `wire`：本服务覆写（加 GOWORK=off）
#   - app.mk 的 `run`（用 ./configs 目录加载，会 panic）：→ 本服务用 `serve`
#   - 本服务的 `run` 不存在（避免与 app.mk 冲突）

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
# 重新生成 cmd/server/wire_gen.go。
#
# 为什么覆盖 app.mk 的 wire：
#   app.mk 用 `go run -mod=mod ...`，在 go workspace 模式下必然失败：
#     "go: -mod may only be set to readonly or vendor when in workspace mode"
#   本地覆写为 GOWORK=off 绕开（仅会触发一条 expected warning）。
#
# 警告信息：
#   Makefile:N: warning: overriding commands for target 'wire'
#   ../../../app.mk:M: warning: ignoring old commands for target 'wire'
# 这是 GNU make 的硬限制，无法静默。
wire:
	@GOWORK=off go run github.com/google/wire/cmd/wire ./cmd/server
	@echo "✓ wire_gen.go regenerated"

# ============================================================================
# 运行（核心）
# ============================================================================

.PHONY: serve
# 直接运行服务（生产配置 ./configs/config.yaml）。
# 自动 source .env.local（gitignored；放凭据用）。
#
# 用法：
#   make serve                                  # 默认 config.yaml
#   make serve CONF=./configs/config.demo.yaml # 自定义配置
#   make serve CONF=./configs/config.yaml      # 显式指定生产配置
#
# 为什么叫 `serve` 而不是 `run`：
#   app.mk 的 `run` 用 `-conf ./configs` 目录加载，会把 configs/tenants.json
#   当配置加载触发 panic（JSON 数组无法 merge 到 Bootstrap map）。
#   改名 `serve` 规避冲突，并提供更明确的语义（运行一个服务）。
serve:
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

.PHONY: build-serve
# 编译后运行（生产配置）— 比 make serve 快（避免每次 go run 都重编译）。
build-serve:
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
# pkill 匹配 `evie/tool`（路径），go run 后子进程路径不变
kill:
	@pkill -f "evie/tool" || true
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
# CI 检查：wire_gen.go 与最新 wire 输出一致
# 注：app.mk 的 wire 用 `-mod=mod`，在 workspace 下会失败。
#     本地兜底：先试 app.mk 的 wire，失败时回退到 GOWORK=off。
generate-check:
	$(MAKE) wire
	@cd ../../.. && git diff --exit-code -- \
		app/evie/tool/cmd/server/wire_gen.go || \
		(echo "✗ wire_gen.go 已过时，请运行 make wire 并提交"; exit 1)
	@echo "✓ generate-check passed"

# ============================================================================
# 容器镜像（Docker + cnb 制品库）
# ============================================================================
#
# 使用：
#   make docker-build                            # 构建镜像（local）
#   make docker-push                             # 推送 latest + v1.7.0 到 cnb
#   make docker-release                          # 一键 build + push
#
# 镜像命名：
#   stack-haven/evie-tool:<tag>            (本地)
#   docker.cnb.cool/beiduoke/iaas/evie-tool:<tag>  (cnb 远端)
#
# 自动生成的 tag：
#   latest  ← 默认 tag，始终指向最新版本（每次覆盖）
#   v1.7.0  ← 语义化版本（从 git tag 取，无 tag 则用 dev）
#   <sha>   ← git commit 短 hash（精确版本回溯）
#
# 覆盖：
#   make docker-build VERSION=v1.8.0   # 自定义版本号
#   make docker-push CNB_REGISTRY=other-registry.example.com/team/proj

.PHONY: docker-build
# 构建 evie/tool 容器镜像。
# 自动生成 3 个 tag：latest（默认）/<git-sha>/<version>
# - VERSION: 从 git tag 取最新 tag，无 tag 时为 "dev"
# - 默认 Dockerfile.evie-tool
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
	@echo "✓ 构建完成"
	@docker images | grep "stack-haven/evie-tool" | head -5

.PHONY: docker-push
# 推送镜像到 cnb 制品库。
# 默认 push: latest + 当前 VERSION（语义化）
# 可覆盖：make docker-push VERSION=v1.7.0
docker-push:
	@VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	SHA=$$(git rev-parse --short HEAD); \
	IMAGE_BASE=docker.cnb.cool/beiduoke/iaas/evie-tool; \
	echo "▶ 打远端 tag"; \
	docker tag stack-haven/evie-tool:latest $$IMAGE_BASE:latest; \
	docker tag stack-haven/evie-tool:$$VERSION $$IMAGE_BASE:$$VERSION; \
	docker tag stack-haven/evie-tool:$$SHA $$IMAGE_BASE:$$SHA; \
	docker images | grep "$$IMAGE_BASE" | head -5; \
	echo ""; \
	echo "▶ 推送 latest"; \
	docker push $$IMAGE_BASE:latest; \
	echo ""; \
	echo "▶ 推送 $$VERSION"; \
	docker push $$IMAGE_BASE:$$VERSION; \
	echo ""; \
	echo "▶ 推送 $$SHA"; \
	docker push $$IMAGE_BASE:$$SHA; \
	echo ""; \
	echo "✓ 推送完成"
	@echo ""
	@echo "✓ 远端镜像已更新（latest 自动指向新版本）"
	@echo "  生产环境拉取："
	@echo "    docker pull $$IMAGE_BASE:latest"

.PHONY: docker-release
# 一键 build + push。跳过中间验证步骤。
# 适合本地确认后直接发生产。
docker-release: docker-build docker-push
	@echo "✓ 发布完成"

.PHONY: docker-test
# 本地容器运行 + 接口验证（CI/快速回归用）
# 假设 docker daemon 在跑，会占用本地端口 8110/9110
docker-test:
	@echo "▶ 启动容器（host network 模式，redis 走 host IP）"
	docker rm -f evie-tool-test 2>/dev/null || true
	docker run -d \
		--name evie-tool-test \
		--network host \
		-v $(CURDIR)/configs:/data/conf:ro \
		-v $(CURDIR)/upload:/data/upload:rw \
		stack-haven/evie-tool:latest \
		-conf /data/conf/config.yaml
	@sleep 5
	@echo "▶ 健康检查"
	@curl -fsS http://127.0.0.1:8110/health/ready | python3 -m json.tool || \
		(echo "✗ 健康检查失败"; docker logs evie-tool-test --tail 30; exit 1)
	@echo ""
	@echo "✓ 容器内服务运行正常"
	@echo "  HTTP:    http://127.0.0.1:8110"
	@echo "  gRPC:    127.0.0.1:9110"
	@echo "  logs:    docker logs evie-tool-test --tail 50"
	@echo "  cleanup: docker rm -f evie-tool-test"

