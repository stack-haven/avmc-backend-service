include ../../../app.mk


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
demo:
	@echo "evie/tool demo"

# === 文档目标 ===

.PHONY: doc
# 生成 OpenAPI v3 文档（Kratos assets embed）
doc:
	@cd ../../../proto && \
	buf generate --template evie/tool/v1/buf.openapi.gen.yaml 2>/dev/null || \
	echo "OpenAPI generation skipped (run from proto/evie/tool/v1 after adding buf.gen.yaml)"