VERSION=$(shell git describe --tags --always)

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest

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

.PHONY: contract-check
# run strict protobuf and generated-code contract checks
contract-check: proto-lint generate-check

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
# lint only new changes relative to HEAD (for incremental adoption)
lint-new:
	@golangci-lint run --new --timeout 5m ./...

.PHONY: security-check
# run dedicated security scan
security-check:
	@gosec -quiet -fmt=colored-line-number ./...

.PHONY: check
# run the required local quality gate
check: fmt-check lint
	go vet ./...
	go test -timeout 90s ./...
	git diff --check

.PHONY: http-convention-check
# verify HTTP paths follow AIP-136 (custom methods use ':', not '/')
http-convention-check:
	@./scripts/check-http-path-convention.sh

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
