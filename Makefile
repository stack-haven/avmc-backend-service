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
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || \
		(echo "Go files must be formatted with gofmt:"; gofmt -l $$(find . -name '*.go' -not -path './vendor/*'); exit 1)

.PHONY: check
# run the required local quality gate
check: fmt-check
	go vet ./...
	go test -timeout 90s ./...
	git diff --check

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
