#!/usr/bin/env bash
# 准备离线模型：将本机 ModelScope 模型缓存复制到 ./models，供 Docker 挂载。
#
# 用法：
#   ./scripts/prepare_models.sh
#
# 模型缓存默认位于 ~/.cache/modelscope/hub/models，可用环境变量覆盖：
#   MODELSCOPE_CACHE=/path/to/modelscope ./scripts/prepare_models.sh
#
# 复制后，docker compose 会将 ./models 挂载到容器 /root/.cache/modelscope/hub/models，
# 启动时不再从 ModelScope 拉取模型。
set -euo pipefail

SRC="${MODELSCOPE_CACHE:-$HOME/.cache/modelscope/hub/models}"
DST="$(cd "$(dirname "$0")/.." && pwd)/models"

if [ ! -d "$SRC/iic" ]; then
  echo "错误：未找到模型缓存 $SRC/iic" >&2
  echo "请先在本地运行一次 funasr-server 下载模型，或手动放置模型到该目录。" >&2
  exit 1
fi

echo "复制模型缓存：$SRC/iic -> $DST/iic"
mkdir -p "$DST"

if command -v rsync >/dev/null 2>&1; then
  rsync -a --info=progress2 "$SRC/iic/" "$DST/iic/"
else
  cp -r "$SRC/iic/" "$DST/"
fi

echo "完成。模型目录：$DST"
echo "所需模型（约 2.8GB）："
du -sh "$DST/iic"/*/ 2>/dev/null || true
