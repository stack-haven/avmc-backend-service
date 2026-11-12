VERSION=$(shell git describe --tags --always)

# L0 服务委托命令（serve-<svc> / gen-<svc> / migrate-<svc> ...），
# 由 services.yaml 生成，禁止手改。见 docs/architecture/4-10-治理-命令契约.md
-include mk/services.mk

# ---------------------------------------------------------------------------
# 工具链版本锁定（禁止 @latest）
# 与 .github/workflows/backend-ci.yml 中的 BUF_VERSION / GOLANGCI_VERSION 保持一致。
# TODO(D6-8): 迁移到 tools/tools.go + go.mod 锁定，见 docs/architecture/4-10-治理-命令契约.md
# ---------------------------------------------------------------------------
KRATOS_VERSION          ?= v2.9.2
PROTOC_GEN_GO_VERSION   ?= v1.36.11
PROTOC_GEN_GO_GRPC_VERSION ?= v1.6.0
AIP_VERSION             ?= v0.80.0
WIRE_VERSION            ?= v0.7.0
ENT_VERSION             ?= v0.14.5

.PHONY: setup-tools
# install every code generator at a pinned version (required before `make proto`)
setup-tools:
	@echo "==> installing pinned codegen toolchain"
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@$(KRATOS_VERSION)
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@$(KRATOS_VERSION)
	go install go.einride.tech/aip/cmd/protoc-gen-go-aip@$(AIP_VERSION)
	go install github.com/google/wire/cmd/wire@$(WIRE_VERSION)
	go install entgo.io/ent/cmd/ent@$(ENT_VERSION)
	@echo "==> done. 需要单独安装（非 Go 模块）：buf / golangci-lint / gosec"

.PHONY: init
# init env (kept for backwards compatibility; prefer `make setup-tools`)
init: setup-tools
	go install github.com/go-kratos/kratos/cmd/kratos/v2@$(KRATOS_VERSION)
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest

.PHONY: gen-servicetargets
# 从 services.yaml 重新生成 mk/services.mk
gen-servicetargets:
	@python3 scripts/gen-service-targets.py

.PHONY: verify-servicetargets
# 校验 mk/services.mk 与 services.yaml 一致（CI 门禁）
verify-servicetargets:
	@python3 scripts/gen-service-targets.py --check

.PHONY: verify-make-contract
# 校验 make 命令契约本身（CI 门禁）
#
# 防的是「删了目标、忘了删引用」这类**命令直接不可用**的缺陷：
#   2026-11-13 `app.mk` 删掉 `config` 目标后，`gen`/`app` 的前置仍写着它，
#   导致 5 个服务的 `make gen` 与根目录 `make gen-<svc>` 全部
#   `No rule to make target 'config'`（exit 2）。
# make 只在真正构建时报错，所以这条同步关系必须由门禁强制。
#
# 校验三类：前置闭包（含变量前置展开）/ $(MAKE) -C 委托一致性 / .PHONY 对称性。
# 纯静态解析，只读、无副作用。见 scripts/check-make-contract.py 头部说明。
verify-make-contract:
	@python3 scripts/check-make-contract.py

.PHONY: setup-hooks
# point git at the versioned .githooks directory (run once after clone)
setup-hooks:
	@git config core.hooksPath .githooks
	@chmod +x .githooks/pre-commit .githooks/commit-msg 2>/dev/null || true
	@echo "core.hooksPath = $$(git config --get core.hooksPath)"

# generate protobuf api go code using buf
.PHONY: proto
proto:
	@cd proto && \
	buf generate --template buf.gen.yaml

.PHONY: conf
# generate the shared config types (proto/common/conf → pkg/conf)
#
# 为什么单独一个目标而不并入 `make proto`：
#   api/ 是对外 API 契约（受 buf breaking 保护），pkg/conf 是服务端内部共享类型，
#   两者的生成根目录、插件集与漂移校验口径都不同。混在一起会让「契约变更」与
#   「配置类型变更」互相顶撞。conf 已在 proto/buf.gen.yaml 的 exclude_paths 排除。
conf:
	@cd proto && \
	buf generate --template ../pkg/conf/buf.gen.yaml

.PHONY: proto-lint
# lint protobuf contracts
proto-lint:
	@cd proto && buf lint

.PHONY: diff-check
# verify the working tree has no unstaged generated drift
diff-check:
	@git diff --check
	@git diff --exit-code -- \
		go.mod \
		go.sum \
		api \
		pkg/conf
	@git diff --cached --exit-code -- \
		go.mod \
		go.sum \
		api \
		pkg/conf
	@test -z "$$(git ls-files --others --exclude-standard -- \
		api pkg/conf)" || \
		(echo "Untracked generated files found:"; \
		 git ls-files --others --exclude-standard -- \
			api pkg/conf; \
		 exit 1)

.PHONY: generate-check
# regenerate global protobuf API code and verify generated outputs are committed
generate-check:
	$(MAKE) proto
	$(MAKE) conf
	go mod tidy
	$(MAKE) diff-check

# buf breaking 的基线 ref。CI 中由 workflow 注入：
#   PR   → origin/<base_ref>
#   push → github.event.before（上一个提交）
# 本地默认 origin/coding/ai。注意：本地 origin 落后时会产生"已接受的历史删除"误报。
BUF_BREAKING_BASE ?= origin/coding/ai

.PHONY: breaking
# 校验 proto 是否存在破坏性变更（buf breaking，类别 FILE）
breaking:
	@base="$(BUF_BREAKING_BASE)"; \
	if [ -z "$$base" ] || [ "$$base" = "0000000000000000000000000000000000000000" ]; then \
		echo "ℹ️  新分支无基线提交，跳过 buf breaking"; \
		exit 0; \
	fi; \
	if ! git rev-parse --verify --quiet "$$base" >/dev/null; then \
		echo "❌ 基线 ref '$$base' 不存在；请设置 BUF_BREAKING_BASE=<ref>"; \
		exit 1; \
	fi; \
	tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp"' EXIT; \
	git archive "$$base" proto | tar -x -C "$$tmp"; \
	echo "==> buf breaking against $$base"; \
	cd proto && buf breaking --against "$$tmp/proto"

.PHONY: contract-check
# run strict protobuf and generated-code contract checks
contract-check: proto-lint breaking generate-check

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...

.PHONY: fmt-check
# verify Go source formatting
fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*' -not -path './api/*' -not -path '*/ent/gen/*'))" || \
		(echo "Go files must be formatted with gofmt:"; gofmt -l $$(find . -name '*.go' -not -path './vendor/*' -not -path './api/*' -not -path '*/ent/gen/*'); exit 1)

.PHONY: lint
# run golangci-lint (P0 rules only in Phase 1, expand in Phase 2/3)
lint:
	@golangci-lint run --timeout 5m ./...

.PHONY: lint-fix
# auto-fix lint issues where supported
lint-fix:
	@golangci-lint run --fix --timeout 5m ./...

.PHONY: lint-new
# lint only issues introduced by the current diff.
# 必须用 --new-from-rev 而不能用 --new：
#   `--new` 只检查工作区相对 HEAD 的变更；在 CI 的干净 checkout 上 diff 为空 →
#   等于零检查（历史缺陷，见 docs/architecture/4-6）。
# 基准分支可用 LINT_BASE 覆盖，默认 origin/coding/ai。
LINT_BASE ?= origin/coding/ai
lint-new:
	@if git rev-parse --verify --quiet "$(LINT_BASE)" >/dev/null; then \
		golangci-lint run --new-from-rev="$(LINT_BASE)" --timeout 5m ./...; \
	else \
		echo "⚠️  base ref '$(LINT_BASE)' 不存在，回退为全量 lint"; \
		golangci-lint run --timeout 5m ./...; \
	fi

.PHONY: security-check
# run dedicated security scan
security-check:
	@gosec -quiet -fmt=colored-line-number ./...

.PHONY: check
# run the required local quality gate
# 2026-09 调整：lint → lint-new（仅本次 diff）。原因：仓库预存 130+ P0
# 漂移是代码质量债，不应随治理 commit 一起过，详见 .golangci.yml 头部
# 与 docs/architecture/4-6 变更记录。`make lint` 仍可手动跑作为报告。
check: fmt-check lint-new
	go vet ./...
	go test -timeout 90s ./...
	$(MAKE) operation-coverage-check
	$(MAKE) verify-servicetargets
	$(MAKE) verify-make-contract
	$(MAKE) data-tx-check
	git diff --check

.PHONY: http-convention-check
# verify HTTP paths follow AIP-136 (custom methods use ':', not '/')
http-convention-check:
	@./scripts/check-http-path-convention.sh

.PHONY: operation-coverage-check
# verify every Casbin-protected operation is reachable (has a menu button or is whitelisted)
# 防止「有路由但无按钮 -> 恒 403」缺陷复发，见 scripts/check-operation-coverage.sh
operation-coverage-check:
	@./scripts/check-operation-coverage.sh

.PHONY: data-tx-check
# verify repositories never bypass DB(ctx) (transaction-aware client)
# 见 scripts/check-data-tx-discipline.sh 与 2026-11-12 P0-4a
data-tx-check:
	@./scripts/check-data-tx-discipline.sh

.PHONY: coverage
# generate a coverage report and compare with the gates in docs/architecture/4-5 §二
# does NOT block PRs by default; integration into CI quality gate is decided per release
coverage:
	@./scripts/check-coverage.sh

.PHONY: import-boundary-check
# verify the service → biz → data dependency direction (see .agents/RULES.md §依赖方向硬规则)
import-boundary-check:
	@./scripts/check-import-boundaries.sh

.PHONY: conf-owner-check
# verify proto/common/conf/* is registered by exactly ONE Go package (protobuf 全局注册表唯一性)
# 唯一允许的注册点是 pkg/conf，基线为空 → 任何新增注册点立即变红
# 见 docs/architecture/4-9 遗留 #11
conf-owner-check:
	@./scripts/check-conf-owners.sh

.PHONY: conf-check
# conf 单一事实源门禁（CI 唯一入口）
conf-check: conf-owner-check

.PHONY: race
# run race detection for global backend packages
race:
	go test -race -timeout 240s \
		./pkg/aip/listing \
		./pkg/auth/... \
		./pkg/health/... \
		./pkg/middleware/safelogging/...

.PHONY: generate
# 重新生成 Ent 代码（各服务 schema → internal/data/ent/gen）并整理依赖
#
# ⚠️ 不要改回 `go generate ./...`：该写法会连带执行 wire_gen.go 中 wire 自带的
#    `go run -mod=mod github.com/google/wire/cmd/wire`，而本仓存在 go.work
#    （workspace 模式），该模式禁止 -mod=mod，整条命令会失败。
#    wire 生成请用各服务的 `make wire`（优先用 setup-tools 安装的二进制，见 app.mk）。
generate:
	@for d in app/*/service/internal/data/ent; do \
		echo "==> go generate ./$$d"; \
		go generate ./$$d || exit 1; \
	done
	go mod tidy

.PHONY: all
# 全量生成：proto → api、conf → pkg/conf、各服务 ent → gen
#
# 原实现是 `all: make proto;`（只生成 api，却叫 all，见 README「Generate all
# files」）。改为真实的前置列表：既符合 README 语义，也让 `make -n all`
# 能一次性曝光任一条生成链路的悬空前置。
all: proto conf generate

# show help
.PHONY: help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
