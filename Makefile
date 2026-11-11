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
		api
	@git diff --cached --exit-code -- \
		go.mod \
		go.sum \
		api
	@test -z "$$(git ls-files --others --exclude-standard -- \
		api)" || \
		(echo "Untracked generated files found:"; \
		 git ls-files --others --exclude-standard -- \
			api; \
		 exit 1)

.PHONY: generate-check
# regenerate global protobuf API code and verify generated outputs are committed
generate-check:
	$(MAKE) proto
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

.PHONY: coverage
# generate a coverage report and compare with the gates in docs/architecture/4-5 §二
# does NOT block PRs by default; integration into CI quality gate is decided per release
coverage:
	@./scripts/check-coverage.sh

.PHONY: import-boundary-check
# verify the service → biz → data dependency direction (see .agents/RULES.md §依赖方向硬规则)
import-boundary-check:
	@./scripts/check-import-boundaries.sh

.PHONY: race
# run race detection for global backend packages
race:
	go test -race -timeout 240s \
		./pkg/aip/listing \
		./pkg/auth/... \
		./pkg/health/... \
		./pkg/middleware/safelogging/...

.PHONY: generate
# generate
generate:
	go generate ./...
	go mod tidy

.PHONY: all
# generate all
all:
	make proto;

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
