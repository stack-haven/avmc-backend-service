#!/usr/bin/env bash
# asr_recognize.sh · 一键 ASR 识别测试
#
# 用途：本地 curl 测试 evie/tool 的 asr:recognize 端点。
# 适用：人工测试 / CI smoke / 调试。
#
# 用法：
#   ./asr_recognize.sh                          # 默认 mp3 + 默认 token
#   ./asr_recognize.sh /path/to/other.mp3      # 自定义音频
#   TOKEN=xxx ./asr_recognize.sh                # 自定义 token
#   BASE_URL=http://other-host:8110 ./asr_recognize.sh
#
# 环境变量：
#   BASE_URL     默认 http://127.0.0.1:8110
#   TOKEN        默认 6dcd5e06b0284b3eb572322c5ac71e50
#   SESSION_ID   默认 postman-test
#   PROVIDER     默认 funasr
#   LANGUAGE     默认 zh
#   ENABLE_ENHANCE 默认 true

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_SERVICE_DIR="$(cd "$SCRIPT_DIR/../../../../.." && pwd)"

# 默认值
BASE_URL="${BASE_URL:-http://127.0.0.1:8110}"
TOKEN="${TOKEN:-6dcd5e06b0284b3eb572322c5ac71e50}"
SESSION_ID="${SESSION_ID:-postman-test}"
PROVIDER="${PROVIDER:-funasr}"
LANGUAGE="${LANGUAGE:-zh}"
ENABLE_ENHANCE="${ENABLE_ENHANCE:-true}"

# 音频路径（第一个位置参数或默认）
AUDIO_PATH="${1:-$BACKEND_SERVICE_DIR/app/evie/tool/testdata/晨会录音.mp3}"

# 检查音频文件
if [ ! -f "$AUDIO_PATH" ]; then
  echo "[!] audio not found: $AUDIO_PATH" >&2
  exit 1
fi

# 编码
B64=$(base64 -i "$AUDIO_PATH" | tr -d '\n')

# 构造 body（用 jq 更安全，缺 jq 时 fallback 到 here-doc）
BODY=$(cat <<EOF
{
  "format": {"encoding": "mp3", "sampleRate": 16000, "language": "$LANGUAGE"},
  "audioData": "$B64",
  "language": "$LANGUAGE",
  "providerName": "$PROVIDER",
  "enableEnhancement": $ENABLE_ENHANCE,
  "sessionId": "$SESSION_ID"
}
EOF
)

echo "[*] POST $BASE_URL/evie/tool/v1/asr:recognize"
echo "    audio   : $AUDIO_PATH ($(wc -c < "$AUDIO_PATH") bytes)"
echo "    token   : ${TOKEN:0:8}..."
echo "    session : $SESSION_ID"
echo

# 用临时文件传 body（避免命令行长度限制）
TMP_BODY=$(mktemp)
trap "rm -f $TMP_BODY" EXIT
echo "$BODY" > "$TMP_BODY"

# 执行
RESP=$(curl -sS -X POST "$BASE_URL/evie/tool/v1/asr:recognize" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --max-time 120 \
  -w "\n--- HTTP %{http_code} time=%{time_total}s ---\n" \
  -d @"$TMP_BODY")

echo "$RESP"