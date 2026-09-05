# ark-lexnorm Makefile
#
# Validation commands per .agents/RULES.md §4.2 / §6.1 / §7.1
# and .agents/REVIEW.md (B series + T series).
#
# Targets:
#   make help         Show this help
#   make build        go build
#   make test         go test (default)
#   make test-race    go test -race (REQUIRED per REVIEW.md C01)
#   make test-cover   go test with coverage report
#   make test-fuzz    go test -fuzz=... (per-package, opt-in)
#   make test-bench   go test -bench=... (per-package, opt-in)
#   make vet          go vet (REQUIRED per RULES.md §6.1)
#   make fmt          gofmt -l (no output = OK)
#   make deps         verify zero third-party dependencies
#   make check        full pre-commit check (fmt + vet + test + race)
#   make ci           full CI pipeline (check + deps)

GO          ?= go
GOFMT       ?= gofmt
PKG         ?= ./...
COVER_FILE  ?= cover.out
TIMEOUT     ?= 60s

.PHONY: help
help: ## Show this help message.
	@awk 'BEGIN {FS = ":.*##"; printf "Targets:\n"} \
	/^[a-zA-Z_-]+:.*?##/ {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ---- Build ----

.PHONY: build
build: ## go build all packages
	$(GO) build $(PKG)

.PHONY: tidy
tidy: ## go mod tidy (verify go.mod / go.sum are clean)
	$(GO) mod tidy
	git diff --exit-code go.mod go.sum

# ---- Test ----

.PHONY: test
test: ## go test (default)
	$(GO) test -timeout=$(TIMEOUT) $(PKG)

.PHONY: test-race
test-race: ## go test -race (REQUIRED per REVIEW.md C01)
	$(GO) test -race -timeout=$(TIMEOUT) $(PKG)

.PHONY: test-cover
test-cover: ## go test with coverage
	$(GO) test -coverprofile=$(COVER_FILE) -timeout=$(TIMEOUT) $(PKG)
	@$(GO) tool cover -func=$(COVER_FILE) | tail -1
	@rm -f $(COVER_FILE)

.PHONY: test-fuzz
test-fuzz: ## run Fuzz tests (FuzzXxx, opt-in)
	@echo "Running Fuzz tests (each 10s)..."
	@$(GO) test -run=^$$ -fuzz=. -fuzztime=10s $(PKG)

.PHONY: test-bench
test-bench: ## run Benchmark tests (opt-in)
	$(GO) test -bench=. -benchmem -benchtime=1s -run=^$$ $(PKG)

# ---- Static analysis ----

.PHONY: vet
vet: ## go vet (REQUIRED per RULES.md §6.1)
	$(GO) vet $(PKG)

.PHONY: fmt
fmt: ## gofmt -l (no output = OK)
	@out=$$($(GO)FMT -l .); \
	if [ -n "$$out" ]; then \
		echo "Files need formatting:"; \
		echo "$$out"; \
		exit 1; \
	else \
		echo "OK: all files formatted"; \
	fi

.PHONY: fmt-fix
fmt-fix: ## gofmt -w (auto-fix formatting)
	$(GO)FMT -w .

# ---- Dependency check ----

.PHONY: deps
deps: ## verify zero third-party dependencies (RULES.md §4.2)
	@echo "Verifying zero third-party dependencies..."
	@bad=$$($(GO) list -deps $(PKG) 2>/dev/null \
		| grep -E "^(github\.com/[^/]+|golang\.org|gopkg\.in)" \
		| grep -v "^github.com/stack-haven/lexnorm" || true); \
	if [ -n "$$bad" ]; then \
		echo "FAIL: third-party dependencies detected:"; \
		echo "$$bad"; \
		exit 1; \
	else \
		echo "OK: only stdlib + github.com/stack-haven/lexnorm"; \
	fi

# ---- Aggregated ----

.PHONY: check
check: fmt vet test test-race ## all pre-commit checks (REVIEW.md R01)

.PHONY: ci
ci: check deps ## full CI pipeline (check + zero-deps check)

.PHONY: clean
clean: ## remove build artifacts
	rm -f $(COVER_FILE)

.DEFAULT_GOAL := help
