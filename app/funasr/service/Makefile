# FunASR 语音识别服务 —— 常用便捷命令
#
# 用法：make <target>（或直接 make 查看帮助）
#
# 可用变量覆盖：
#   PYTHON  Python 解释器（默认 python，若用 conda 环境可指定绝对路径）
#   DOCKER  Docker 命令（默认 docker）

PYTHON ?= python
DOCKER ?= docker

.DEFAULT_GOAL := help

.PHONY: help
help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# ---- 本地开发 ----

.PHONY: install
install: ## 安装依赖（含开发依赖）
	$(PYTHON) -m pip install -e ".[dev]"

.PHONY: run
run: ## 启动批量 + 流式两个服务
	$(PYTHON) -m funasr_server

.PHONY: run-preload
run-preload: ## 启动并预加载模型（启动慢、首请求快）
	$(PYTHON) -m funasr_server --preload

.PHONY: batch
batch: ## 仅启动批量识别服务（SenseVoice）
	$(PYTHON) -m funasr_server --batch-only --preload

.PHONY: stream
stream: ## 仅启动流式识别服务（paraformer-streaming）
	$(PYTHON) -m funasr_server --stream-only --preload

.PHONY: kill
kill: ## 停止本地服务（释放 18000/18001 端口）
	@for port in 18000 18001; do \
		lsof -ti :$$port 2>/dev/null | xargs -r kill -9 2>/dev/null || true; \
	done

.PHONY: health
health: ## 健康检查（批量 + 流式）
	@curl -s http://localhost:18000/health && echo "" && \
	 curl -s http://localhost:18001/health && echo ""

# ---- 测试与质量 ----

.PHONY: test
test: ## 运行单元测试
	$(PYTHON) -m pytest tests/ -v

.PHONY: fmt
fmt: ## 格式化代码（ruff format）
	$(PYTHON) -m ruff format funasr_server tests

.PHONY: lint
lint: ## 代码检查（ruff）
	$(PYTHON) -m ruff check funasr_server tests

.PHONY: check
check: ## 语法编译检查
	$(PYTHON) -m py_compile funasr_server/*.py

# ---- 模型 ----

.PHONY: models
models: ## 准备离线模型（复制模型缓存到 models/）
	./scripts/prepare_models.sh

# ---- Docker ----

.PHONY: build
build: ## 构建 Docker 镜像
	$(DOCKER) build -t funasr-server:latest .

.PHONY: up
up: ## 启动 Docker Compose（构建 + 后台运行）
	cd deploy && $(DOCKER) compose up -d --build

.PHONY: down
down: ## 停止 Docker Compose
	cd deploy && $(DOCKER) compose down

.PHONY: logs
logs: ## 查看容器日志
	cd deploy && $(DOCKER) compose logs -f

.PHONY: compose-config
compose-config: ## 校验 docker-compose 配置
	cd deploy && $(DOCKER) compose config

# ---- 清理 ----

.PHONY: clean
clean: ## 清理缓存与构建产物
	@find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@rm -rf .pytest_cache *.egg-info build dist
