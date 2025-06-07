include ../../../app.mk

# generate OpenAPI v3 doc
.PHONY: doc
doc:
	@cd ../../../proto && \
	buf generate --template avmc/admin/v1/buf.openapi.gen.yaml


# generate config
.PHONY: config
config:
	@cd ../../../proto && \
	buf generate --template ../app/version/service/buf.gen.yaml
	@rm -rf internal/conf && \
	mv proto/common/conf internal && \
	rm -rf proto

.PHONY: config1
config1:
	@cd../../../proto && \
	buf generate --template ../app/avmc/admin/buf.gen.yaml && \
	sudo mv -f common/conf internal && \
	rm -rf common

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
