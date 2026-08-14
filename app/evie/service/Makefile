include ../../../app.mk

.PHONY: generate-check
# regenerate evie service generated code and verify generated outputs are committed
generate-check:
	$(MAKE) api
	$(MAKE) ent
	$(MAKE) config
	$(MAKE) wire
	$(MAKE) doc
	@cd ../../.. && go mod tidy
	$(MAKE) diff-check

.PHONY: diff-check
# verify evie service generated outputs are committed
diff-check:
	@cd ../../.. && git diff --check
	@cd ../../.. && git diff --exit-code -- \
		go.mod \
		go.sum \
		api \
		app/evie/service/cmd/server/assets/openapi.yaml \
		app/evie/service/internal/conf \
		app/evie/service/internal/data/ent/gen
	@cd ../../.. && git diff --cached --exit-code -- \
		go.mod \
		go.sum \
		api \
		app/evie/service/cmd/server/assets/openapi.yaml \
		app/evie/service/internal/conf \
		app/evie/service/internal/data/ent/gen
	@test -z "$$(cd ../../.. && git ls-files --others --exclude-standard -- \
		api \
		app/evie/service/cmd/server/assets/openapi.yaml \
		app/evie/service/internal/conf \
		app/evie/service/internal/data/ent/gen)" || \
		(echo "Untracked generated files found:"; \
		 cd ../../.. && git ls-files --others --exclude-standard -- \
			api \
			app/evie/service/cmd/server/assets/openapi.yaml \
			app/evie/service/internal/conf \
			app/evie/service/internal/data/ent/gen; \
		 exit 1)

.PHONY: race
# run race detection for evie service packages
race:
	go test -race -timeout 240s \
		./internal/authzpolicy \
		./internal/biz \
		./internal/data \
		./internal/server \
		./internal/service

# generate OpenAPI v3 doc
.PHONY: doc
doc:
	@cd ../../../proto && \
	buf generate --template evie/service/v1/buf.openapi.gen.yaml

# generate config
.PHONY: config
config:
	@cd ../../../proto && \
	buf generate --template ../app/evie/service/buf.gen.yaml
	rm -rf internal/conf && \
	mv proto/common/conf internal && \
	rm -rf proto

# generate protobuf ts code
.PHONY: ts
ts:
	@cd ../../../proto && \
	buf generate --template ../app/evie/service/buf.typescript.gen.yaml

.PHONY: migrate
migrate:
	@go run ./cmd/migrate -conf ./configs

.PHONY: policy
policy:
	@go run ./cmd/policy -conf ./configs

.PHONY: kill
# kill running evie service server and release ports 8000/9000
kill:
	@pkill -f "cmd/server/main.go" || true
	@for port in 8100 9100; do \
		lsof -ti :$$port 2>/dev/null | xargs -r kill -9 2>/dev/null || true; \
	done

.PHONY: mock
mock:
	@go run -tags mock ./cmd/mock -conf ./configs

.PHONY: demo
demo:
	@echo "demo"

.PHONY: newent
newent:
	@cd internal/data && \
	echo "new ent" && \
	echo "please input ent name:" && \
	read name; \
	printf "ent name: %s\n" "$$name" && \
	go run -mod=mod entgo.io/ent/cmd/ent new "$$name"
