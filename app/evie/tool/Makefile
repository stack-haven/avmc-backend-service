include ../../../app.mk


# 本 Makefile 只声明 evie/tool 特有的 target。
# 通用 target（wire / test / cover / lint / api / build ...）由 app.mk 提供。
# 例外：`run` 在本地覆盖 app.mk —— app.mk 的 run 用 `-conf ./configs` 目录，
# 会导致 Kratos 把 configs/tenants.example.json 加载为 config source 而 panic。
# 本地 run 默认 demo 模式（零外部依赖），可通过 CONF= 覆盖。

.PHONY: config
# 生成 evie/tool 的 conf.pb.go（基于 internal/conf/conf.proto）
config:
	@protoc -I . --go_out=. --go_opt=paths=source_relative \
		./internal/conf/conf.proto
	@gofmt -w internal/conf/conf.pb.go

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

.PHONY: cli
# 构建 evie-toolctl 运维 CLI（cmd/evie-toolctl）。
cli:
	@go build -o bin/evie-toolctl ./cmd/evie-toolctl
	@echo "bin/evie-toolctl built"

.PHONY: run
# 覆盖 app.mk 的 run（bug 修复）：传入具体配置文件而非目录，
# 避免 Kratos 加载 configs/tenants.example.json（JSON 数组无法 merge 到 Bootstrap map）。
#
# 用法：
#   make run                       # demo 模式（零外部依赖）
#   make run CONF=./configs/config.yaml  # 生产配置（需 redis/qua）
#   make run CONF=./configs/config.demo.yaml  # demo 配置但不走 EVIE_TOOL_DEMO 分支
run:
	@if [ "$(CONF)" = "" ]; then \
		echo "▶ demo 模式（EVIE_TOOL_DEMO=1，零外部依赖）"; \
		EVIE_TOOL_DEMO=1 go run ./cmd/server -conf ./configs/config.demo.yaml; \
	else \
		echo "▶ 使用配置: $(CONF)"; \
		go run ./cmd/server -conf $(CONF); \
	fi

.PHONY: doc
# 生成 OpenAPI v3 文档（Kratos assets embed）
doc:
	@cd ../../../proto && \
	buf generate --template evie/tool/v1/buf.openapi.gen.yaml 2>/dev/null || \
	echo "OpenAPI generation skipped (run from proto/evie/tool/v1 after adding buf.gen.yaml)"

.PHONY: bench
# 运行基准（所有 *_test.go 中的 Benchmark*），含内存统计。
# 注：app.mk 未提供 bench，本地保留。
bench:
	go test -bench=. -benchmem -run=^$ ./...
