include ../../../app.mk


.PHONY: config
# 生成 evie/tool 的 conf.pb.go（基于 internal/conf/conf.proto）
config:
	@protoc -I . --go_out=. --go_opt=paths=source_relative \
		./internal/conf/conf.proto
	@gofmt -w internal/conf/conf.pb.go

.PHONY: proto
# proto: 调用 app.mk 的 api 目标，从 backend-service/proto 重新生成 API。
proto:
	@cd ../../../ && $(MAKE) -s api

.PHONY: wire
# wire: 从 wire.go 重新生成 cmd/server/wire_gen.go。
wire:
	@cd ../../../ && $(MAKE) -s wire

.PHONY: test
# 全部单元/集成测试
test:
	go test ./...

.PHONY: cover
# 生成覆盖率报告（HTML）
cover:
	go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "coverage.html generated"

.PHONY: health
# 探测本地 evie/tool 健康状态
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

.PHONY: race
# run race detection for evie service packages
race:
	go test -race -timeout 240s \
		./internal/biz \
		./internal/data \
		./internal/server \
		./internal/service

.PHONY: demo
# 零外部依赖 demo 模式：StaticProvider + FileSource，启用 EVIE_TOOL_DEMO=1。
demo:
	EVIE_TOOL_DEMO=1 go run ./cmd/server -conf ./configs/config.demo.yaml

# === 文档目标 ===

.PHONY: doc
# 生成 OpenAPI v3 文档（Kratos assets embed）
doc:
	@cd ../../../proto && \
	buf generate --template evie/tool/v1/buf.openapi.gen.yaml 2>/dev/null || \
	echo "OpenAPI generation skipped (run from proto/evie/tool/v1 after adding buf.gen.yaml)"

# === 质量门禁 ===

.PHONY: bench
# 运行基准（所有 *_test.go 中的 Benchmark*），含内存统计。
bench:
	go test -bench=. -benchmem -run=^$ ./...

.PHONY: lint
# 静态检查：gofmt 检查 + go vet。
lint:
	@gofmt -l internal pkg cmd 2>/dev/null | (read line && echo "gofmt diff: $$line" && exit 1) || true
	@go vet ./...