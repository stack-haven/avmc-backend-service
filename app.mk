# =============================================================================
# app.mk —— 后端服务的「服务级通用命令层」（L1）
# =============================================================================
#
# 设计定位（见 docs/architecture/4-10-治理-命令契约.md）：
#
#   L0  backend-service/Makefile        仓库级治理：check / verify-* / CI 唯一入口
#   L1  backend-service/app.mk          ← 本文件：服务级通用命令，被子服务 include
#   L2  app/<svc>/Makefile              仅本服务特有（端口、专用 cmd、覆盖项）
#
# 使用方式：在服务目录内执行，例如
#
#   cd backend-service/app/platform/service
#   make run
#   make gen
#
# 为什么必须在服务目录内执行：本文件通过 PWD 推导服务身份（SERVICE_PATH），
# 从而让同一套命令自动作用于"当前服务"。这是 kratos 大仓的标准做法。
#
# 变更纪律：
#   - 能复用的命令必须上收到本文件，禁止在各服务 Makefile 中复制
#     （历史问题：newent 复制 4 份、migrate 复制 3 份、mock/policy/kill 复制 2 份）
#   - 只有"确实与具体服务绑定"的才留在 L2，例如端口、专用 cmd 目录、三方工具链
#   - 本文件内禁止 @latest，工具版本由根 Makefile 的 setup-tools 统一锁定
#
# 2026-11-11 大修（A 案 D6-8）：
#   1. docker 目标指向不存在的 ../../../.docker/Dockerfile → 改为 backend-service/Dockerfile
#   2. build 的 -X main.Service.Version（符号错、变量未定义）→ 改为 main.Version
#   3. init 全 @latest → 委托根 setup-tools（版本锁定）
#   4. 删除与 ent 完全重复的 ent02
#   5. wire 的 -mod=mod 在本地 go.work 下必失败 → 统一 GOWORK=off
#   6. openapi 产物硬编码到 platform assets → 改为按服务模板生成
#   7. run 依赖 api（每次启动都重建，掩盖生成漂移）→ 去掉依赖
#   8. VERSION 在无 tag 仓库下为空（git describe 失败）→ 加 --always
#   9. 上收 newent / migrate / mock / policy / kill / race / config / ts /
#      diff-check / generate-check 等 10 个被复制多份的命令
# =============================================================================

# ── 服务身份（由 PWD 推导，禁止手工指定）────────────────────────────────────
# app/platform/service → platform/service ；app/evie/tool → evie/tool
APP_RELATIVE_PATH ?= $(shell a=`basename "$$PWD"`; cd .. && b=`basename "$$PWD"`; echo "$$b/$$a")
BACKEND_ROOT      := $(abspath ../../..)
PROTO_DIR         := $(BACKEND_ROOT)/proto
SERVICE_PATH      := app/$(APP_RELATIVE_PATH)

# ── 版本 ────────────────────────────────────────────────────────────────────
# 注意：本仓库当前 0 个 git tag，`git describe --tags` 会失败；
# 必须带 --always 才能回退到 commit hash，否则二进制 version 为空。
VERSION ?= $(shell git -C $(BACKEND_ROOT) describe --tags --always 2>/dev/null || echo dev)

# ── 可被 L2 覆盖的变量 ──────────────────────────────────────────────────────
# 注意：变量值后不要写行内注释 —— make 会把注释前的空格并入变量值，
# 导致 `-conf ./configs    ` 这类带尾随空格的参数（历史上的隐性坑）。
CONFIG_DIR        ?= ./configs
PORTS             ?=
KILL_PATTERN      ?= cmd/server
RACE_PKGS         ?= ./internal/biz ./internal/data ./internal/server ./internal/service
DOCKER_IMAGE      ?= ark/$(subst /,-,$(APP_RELATIVE_PATH)):$(VERSION)
OPENAPI_TEMPLATE  ?= $(APP_RELATIVE_PATH)/v1/buf.openapi.gen.yaml

# ── 探测（决定某些命令是否可用）─────────────────────────────────────────────
HAS_CMD_MIGRATE := $(wildcard ./cmd/migrate)
HAS_CMD_MOCK    := $(wildcard ./cmd/mock)
HAS_CMD_POLICY  := $(wildcard ./cmd/policy)
HAS_CONF_TMPL   := $(wildcard $(BACKEND_ROOT)/app/$(APP_RELATIVE_PATH)/buf.gen.yaml)
HAS_TS_TMPL     := $(wildcard $(BACKEND_ROOT)/app/$(APP_RELATIVE_PATH)/buf.typescript.gen.yaml)
HAS_OPENAPI     := $(wildcard $(PROTO_DIR)/$(OPENAPI_TEMPLATE))

# =============================================================================
# 环境
# =============================================================================

.PHONY: init
# 安装生成器工具链（委托根 Makefile，版本已锁定，禁止 @latest）
init:
	@$(MAKE) -C $(BACKEND_ROOT) setup-tools

.PHONY: dep
# 下载 Go 依赖
dep:
	@go mod download

.PHONY: vendor
# 生成 vendor 目录
vendor:
	@go mod vendor

# =============================================================================
# 代码生成
# =============================================================================

.PHONY: gen
# 全量生成：proto → api，ent → gen，conf，wire，openapi
gen: api ent config wire openapi
	@echo "✅ gen done ($(APP_RELATIVE_PATH))"

.PHONY: api
# 从 proto 生成 Go 代码（gRPC / HTTP / validate / AIP / errors）
api:
	@cd $(PROTO_DIR) && buf generate --template buf.gen.yaml
	@echo "✅ api generated"

.PHONY: ent
# 从 ent/schema 生成 ent 代码
ent:
	@if [ -d ./internal/data/ent ]; then \
		go generate ./internal/data/ent; \
		echo "✅ ent generated"; \
	else \
		echo "ℹ️  本服务无 internal/data/ent，跳过"; \
	fi

.PHONY: wire
# 重新生成 cmd/server/wire_gen.go
#
# 优先使用 `make setup-tools` 安装的 wire 二进制，而不是 `go run`：
#   - `go run github.com/google/wire/cmd/wire` 在 workspace 模式下会被 -mod 限制拒绝
#   - 加 GOWORK=off 后又要求 wire 的全部工具依赖出现在 go.sum 中
#     （实测 go.sum 缺 github.com/google/subcommands → 失败）
#   - 用已安装的二进制不参与模块解析，两种模式都稳定
wire:
	@if command -v wire >/dev/null 2>&1; then \
		wire ./cmd/server; \
	else \
		echo "⚠️  未找到 wire 二进制，回退 go run（建议先 make setup-tools）"; \
		GOWORK=off go run -mod=mod github.com/google/wire/cmd/wire ./cmd/server; \
	fi
	@echo "✅ wire generated"

# 若服务自带 config 实现（如 evie/tool 用 protoc 直出），可在 include 前设置
# SKIP_APP_MK_CONFIG := 1 来跳过本定义，避免 make 的 overriding 警告。
ifndef SKIP_APP_MK_CONFIG
.PHONY: config
# 生成 internal/conf/*.pb.go（模板由 <service>/buf.gen.yaml 提供）
config:
	@if [ ! -d "$(BACKEND_ROOT)/app/$(APP_RELATIVE_PATH)" ] || [ -z "$(HAS_CONF_TMPL)" ]; then \
		echo "ℹ️  本服务未提供 buf.gen.yaml，跳过 config"; \
	else \
		cd $(PROTO_DIR) && \
		buf generate --template ../app/$(APP_RELATIVE_PATH)/buf.gen.yaml && \
		cd - >/dev/null && \
		rm -rf internal/conf && \
		mv proto/common/conf internal && \
		rm -rf proto && \
		echo "✅ config generated"; \
	fi
endif

.PHONY: ts
# 生成 TypeScript 客户端（模板由 <service>/buf.typescript.gen.yaml 提供）
ts:
	@if [ -z "$(HAS_TS_TMPL)" ]; then \
		echo "ℹ️  本服务未提供 buf.typescript.gen.yaml，跳过 ts"; \
	else \
		cd $(PROTO_DIR) && \
		buf generate --template ../app/$(APP_RELATIVE_PATH)/buf.typescript.gen.yaml; \
	fi

.PHONY: openapi
# 生成 OpenAPI v3 文档到 cmd/server/assets/
# 只使用本服务自己的模板 <proto>/<service>/v1/buf.openapi.gen.yaml。
# 为什么不做「回退到全局模板」：全局模板 proto/buf.openapi.gen.yaml 的 out
# 硬编码为 platform 的 assets 目录，从其它服务执行会把 platform 的文档覆盖掉
# （历史缺陷，见 4-10 §五）。
openapi:
	@if [ -n "$(HAS_OPENAPI)" ]; then \
		cd $(PROTO_DIR) && \
		buf generate --template $(OPENAPI_TEMPLATE) && \
		echo "✅ openapi generated"; \
	else \
		echo "ℹ️  本服务无 $(OPENAPI_TEMPLATE)，跳过 openapi"; \
	fi

.PHONY: doc
# 兼容别名：旧服务 Makefile 里该目标名为 doc（且路径写错为 platform/admin，长期失效）
doc: openapi
	@true

.PHONY: newent
# 交互式新建 ent schema（历史上有 4 份副本，已上收）
newent:
	@cd internal/data && \
	printf "ent name: " && \
	read name; \
	printf "creating ent schema: %s\n" "$$name" && \
	go run -mod=mod entgo.io/ent/cmd/ent new "$$name"

# =============================================================================
# 构建 / 运行
# =============================================================================

.PHONY: build
# 构建当前服务到 ./bin/
build:
	@mkdir -p ./bin
	@go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...
	@echo "✅ built $(APP_RELATIVE_PATH) → ./bin/ (version=$(VERSION))"

.PHONY: app
# 全流水线：生成 + 构建
app: api ent config wire build

.PHONY: run
# 启动当前服务（不自动重新生成；需要新鲜代码请先 make gen）
run:
	@echo "▶ $(APP_RELATIVE_PATH) 启动中（conf=$(CONFIG_DIR)）"
	@go run ./cmd/server -conf $(CONFIG_DIR)

.PHONY: clean
# 清理构建产物
clean:
	@go clean
	@rm -f ./bin/* ./coverage.out
	@echo "✅ cleaned"

# =============================================================================
# 数据库 / 初始化（按存在性自动跳过）
# =============================================================================

.PHONY: migrate
# 执行 ent schema 迁移（需要 ./cmd/migrate）
migrate:
	@if [ -z "$(HAS_CMD_MIGRATE)" ]; then \
		echo "❌ 本服务无 cmd/migrate"; exit 1; \
	fi
	@go run ./cmd/migrate -conf $(CONFIG_DIR)

.PHONY: mock
# 写入 mock / seed 数据（需要 ./cmd/mock）
mock:
	@if [ -z "$(HAS_CMD_MOCK)" ]; then \
		echo "❌ 本服务无 cmd/mock"; exit 1; \
	fi
	@go run -tags mock ./cmd/mock -conf $(CONFIG_DIR)

.PHONY: policy
# 初始化 Casbin 策略（需要 ./cmd/policy）
policy:
	@if [ -z "$(HAS_CMD_POLICY)" ]; then \
		echo "❌ 本服务无 cmd/policy"; exit 1; \
	fi
	@go run ./cmd/policy -conf $(CONFIG_DIR)

# =============================================================================
# 容器
# =============================================================================

.PHONY: docker
# 构建服务镜像（与 backend-service/Dockerfile 的 SERVICE_PACKAGE 约定一致）
docker:
	@docker build \
		-f $(BACKEND_ROOT)/Dockerfile \
		--build-arg SERVICE_PACKAGE=./$(SERVICE_PATH)/cmd/server \
		--build-arg VERSION=$(VERSION) \
		-t $(DOCKER_IMAGE) \
		$(BACKEND_ROOT)
	@echo "✅ image: $(DOCKER_IMAGE)"

# =============================================================================
# 验证
# =============================================================================

.PHONY: test
# 运行本服务单测
test:
	@go test -count=1 -timeout 120s ./...

.PHONY: cover
# 本服务覆盖率
cover:
	@go test -count=1 -timeout 120s ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1

.PHONY: vet
# 静态检查
vet:
	@go vet ./...

.PHONY: lint
# golangci-lint（带超时，与根 Makefile 一致）
lint:
	@golangci-lint run --timeout 5m ./...

.PHONY: race
# 竞态检测（包列表可由 L2 覆盖）
race:
	@go test -race -timeout 240s $(RACE_PKGS)

.PHONY: diff-check
# 校验生成物无漂移（服务级）
diff-check:
	@cd $(BACKEND_ROOT) && git diff --check
	@cd $(BACKEND_ROOT) && git diff --exit-code -- \
		go.mod go.sum api \
		$(SERVICE_PATH)/cmd/server/assets/openapi.yaml \
		$(SERVICE_PATH)/internal/conf \
		$(SERVICE_PATH)/internal/data/ent/gen
	@cd $(BACKEND_ROOT) && git diff --cached --exit-code -- \
		go.mod go.sum api \
		$(SERVICE_PATH)/cmd/server/assets/openapi.yaml \
		$(SERVICE_PATH)/internal/conf \
		$(SERVICE_PATH)/internal/data/ent/gen
	@test -z "$$(cd $(BACKEND_ROOT) && git ls-files --others --exclude-standard -- \
		api $(SERVICE_PATH)/cmd/server/assets/openapi.yaml \
		$(SERVICE_PATH)/internal/conf $(SERVICE_PATH)/internal/data/ent/gen)" || \
		(echo "❌ 存在未跟踪的生成物："; \
		 cd $(BACKEND_ROOT) && git ls-files --others --exclude-standard -- \
			api $(SERVICE_PATH)/cmd/server/assets/openapi.yaml \
			$(SERVICE_PATH)/internal/conf $(SERVICE_PATH)/internal/data/ent/gen; \
		 exit 1)
	@echo "✅ diff-check passed"

ifndef SKIP_APP_MK_GENERATE_CHECK
.PHONY: generate-check
# 重新生成并校验生成物已提交
generate-check: api ent config wire openapi
	@cd $(BACKEND_ROOT) && go mod tidy
	@$(MAKE) diff-check
endif

.PHONY: kill
# 杀掉本服务进程并释放端口（端口与进程模式由 L2 指定）
kill:
	@pkill -f "$(KILL_PATTERN)" 2>/dev/null || true; \
	if [ -n "$(PORTS)" ]; then \
		for port in $(PORTS); do \
			lsof -ti :$$port 2>/dev/null | xargs -r kill -9 2>/dev/null || true; \
		done; \
		echo "✅ killed '$(KILL_PATTERN)', released ports: $(PORTS)"; \
	else \
		echo "✅ killed '$(KILL_PATTERN)'（未指定 PORTS）"; \
	fi

.PHONY: help
# 显示可用命令
help:
	@echo ''
	@echo "  $(APP_RELATIVE_PATH)  [L1 app.mk]"
	@echo ''
	@echo '  Usage: make [target]'
	@echo ''
	@echo '  Targets:'
	@awk '/^[a-zA-Z\-_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "    \033[36m%-20s\033[0m %s\n", helpCommand, helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
