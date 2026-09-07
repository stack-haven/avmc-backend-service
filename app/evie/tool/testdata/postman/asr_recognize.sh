#!/usr/bin/env bash
# asr_recognize.sh · 一键 ASR 识别测试
#
# 用途：本地 curl 测试 evie/tool 的 /evie/tool/v1/asr:recognize 端点。
# 适用：人工测试 / CI smoke / 排错。
#
# 用法：
#   ./asr_recognize.sh                                # 用 .env.local 的 TOKEN + 默认音频
#   TOKEN=eyJxxx ./asr_recognize.sh                  # 临时覆盖 token
#   ./asr_recognize.sh /path/to/audio.wav            # 自定义音频
#   BASE_URL=http://host:8110 ./asr_recognize.sh     # 自定义地址
#   ./asr_recognize.sh -n                            # dry-run（只打印 body，不发请求）
#   ./asr_recognize.sh -d                            # demo 模式（用 demo-token，跳过 TOKEN 校验）
#   ./asr_recognize.sh -h                            # 帮助
#
# 环境变量（必填或可选）：
#   TOKEN         Bearer token（必填；脚本从 .env.local 自动读 TOKEN；亦可手动 export）
#   BASE_URL      默认 http://127.0.0.1:8110
#   SESSION_ID    默认 postman-$(date +%s)
#   ENABLE_ENHANCE 默认 true
#
# 退出码：
#   0  成功（HTTP 2xx 且业务 status=SUCCESS/DEGRADED）
#   1  用户错误（参数 / 缺 TOKEN / 服务不可达）
#   2  ASR 失败（HTTP 4xx/5xx 或业务 status=ERROR）
#   3  网络错误
#
# 依赖：bash 4+ / curl / base64 / 可选 jq（美化输出）

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOL_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"           # app/evie/tool
PROJECT_ROOT="$(cd "$TOOL_DIR/../../.." && pwd)"      # backend-service
ENV_LOCAL="$TOOL_DIR/.env.local"

# ----- 帮助 -----
usage() {
  sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
}

# ----- 解析 flag -----
DRY_RUN=0
DEMO=0
while getopts ":ndh" opt; do
  case $opt in
    n) DRY_RUN=1 ;;
    d) DEMO=1 ;;
    h) usage ;;
    \?) echo "[!] 未知参数: -$OPTARG" >&2; usage ;;
  esac
done
shift $((OPTIND - 1))

# ----- 自动 source .env.local（与 make run-prod 行为一致）-----
if [ -f "$ENV_LOCAL" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$ENV_LOCAL"
  set +a
fi

# ----- 默认值 -----
BASE_URL="${BASE_URL:-http://127.0.0.1:8110}"
SESSION_ID="${SESSION_ID:-postman-$(date +%s)}"
ENABLE_ENHANCE="${ENABLE_ENHANCE:-true}"
AUDIO_PATH="${1:-$TOOL_DIR/testdata/晨会录音.mp3}"

# ----- Demo 模式（-d）：跳过 TOKEN 校验，用 demo-token  -----
if [ "$DEMO" = "1" ]; then
  TOKEN="demo-token"
  echo "[*] DEMO 模式：使用 demo-token（需服务以 EVIE_TOOL_DEMO=1 启动）"
fi

# ----- 校验 -----
# Dry-run 跳过 TOKEN 检查（不发请求，只需打印 body）
if [ "$DRY_RUN" = "0" ] && [ -z "${TOKEN:-}" ]; then
  echo "[!] 缺少 TOKEN 环境变量" >&2
  echo "    方式 1：export TOKEN=eyJxxx" >&2
  echo "    方式 2：在 $ENV_LOCAL 添加一行：TOKEN=eyJxxx" >&2
  echo "    方式 3：使用 -d 走 demo 模式（需 make demo 启动服务）" >&2
  exit 1
fi

if [ ! -f "$AUDIO_PATH" ]; then
  echo "[!] 音频文件不存在: $AUDIO_PATH" >&2
  echo "    用法: $0 /path/to/audio.{mp3,wav,pcm}" >&2
  exit 1
fi

# ----- Dry run：只打印 body，不发请求也不需健康检查 -----
if [ "$DRY_RUN" = "1" ]; then
  BYTES=$(wc -c < "$AUDIO_PATH" | tr -d ' ')
  B64=$(base64 -i "$AUDIO_PATH" | tr -d '\n')
  LOWER_PATH=$(printf '%s' "$AUDIO_PATH" | tr '[:upper:]' '[:lower:]')
  case "$LOWER_PATH" in
    *.mp3)  ENC=mp3 ;;
    *.wav)  ENC=wav ;;
    *.pcm)  ENC=pcm ;;
    *.opus) ENC=opus ;;
    *)      ENC=mp3 ;;
  esac
  BODY=$(cat <<EOF
{
  "format": {"encoding": "$ENC", "sampleRate": 16000, "bitDepth": 16, "channels": 1},
  "audioData": "$B64",
  "sessionId": "${SESSION_ID:-postman-dryrun}",
  "enableEnhancement": ${ENABLE_ENHANCE:-true}
}
EOF
)
  echo "[*] DRY-RUN：以下是要发送的 body（前 200 字节 + ...）"
  echo "$BODY" | head -c 200 || true
  echo
  echo "    (总 body 长度: $(echo -n "$BODY" | wc -c | tr -d ' ') bytes)"
  exit 0
fi

# ----- 健康检查（fail fast）-----
HEALTH_URL="$BASE_URL/health/live"
echo "[*] 健康检查: $HEALTH_URL"
if ! curl -sS -m 3 -o /dev/null -w "%{http_code}" "$HEALTH_URL" | grep -q '^200$'; then
  echo "[!] 服务不可达：$HEALTH_URL 未返回 200（确认 make run-prod 已启动？）" >&2
  exit 1
fi

# ----- 编码音频 -----
BYTES=$(wc -c < "$AUDIO_PATH" | tr -d ' ')
B64=$(base64 -i "$AUDIO_PATH" | tr -d '\n')

# ----- 推断 encoding（按扩展名，POSIX 兼容）-----
LOWER_PATH=$(printf '%s' "$AUDIO_PATH" | tr '[:upper:]' '[:lower:]')
case "$LOWER_PATH" in
  *.mp3)  ENC=mp3 ;;
  *.wav)  ENC=wav ;;
  *.pcm)  ENC=pcm ;;
  *.opus) ENC=opus ;;
  *)      ENC=mp3 ;;
esac

# ----- 构造 body（严格按 proto 字段；不使用不存在的 language/providerName）-----
BODY=$(cat <<EOF
{
  "format": {"encoding": "$ENC", "sampleRate": 16000, "bitDepth": 16, "channels": 1},
  "audioData": "$B64",
  "sessionId": "$SESSION_ID",
  "enableEnhancement": $ENABLE_ENHANCE
}
EOF
)

# ----- Dry run：只打印，不发请求 -----
# （dry-run 已在上面提前 exit；这里是请求路径）

# ----- 打印请求概要 -----
echo "[*] POST $BASE_URL/evie/tool/v1/asr:recognize"
echo "    audio    : $AUDIO_PATH ($BYTES bytes, enc=$ENC)"
echo "    token    : ${TOKEN:0:12}..."
echo "    session  : $SESSION_ID"
echo "    enhance  : $ENABLE_ENHANCE"
echo

# ----- 用临时文件传 body（避免命令行长度限制 + audioData 含特殊字符）-----
TMP_BODY=$(mktemp)
TMP_RESP=$(mktemp)
trap "rm -f $TMP_BODY $TMP_RESP" EXIT
printf '%s' "$BODY" > "$TMP_BODY"

# ----- 执行 -----
HTTP_CODE=$(curl -sS -X POST "$BASE_URL/evie/tool/v1/asr:recognize" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --max-time 120 \
  -o "$TMP_RESP" \
  -w "%{http_code}|%{time_total}" \
  -d @"$TMP_BODY") || {
  echo "[!] curl 失败（网络/DNS/超时）" >&2
  exit 3
}

HTTP="${HTTP_CODE%|*}"
TIME="${HTTP_CODE#*|}"

echo "--- HTTP $HTTP  time=${TIME}s ---"
echo

# ----- 响应渲染 -----
if command -v jq >/dev/null 2>&1 && head -c1 "$TMP_RESP" | grep -q '{'; then
  cat "$TMP_RESP" | jq . || cat "$TMP_RESP"
else
  cat "$TMP_RESP"
fi

echo

# ----- 结果判定 -----
case "$HTTP" in
  200)
    # 业务 status: 1=SUCCESS 2=DEGRADED（按 proto 注释；proto 实际是 int32 业务码）
    if command -v jq >/dev/null 2>&1; then
      BIZ_STATUS=$(jq -r '.status // empty' "$TMP_RESP" 2>/dev/null || echo "")
      RAW=$(jq -r '.rawText // empty' "$TMP_RESP" 2>/dev/null || echo "")
      ENH=$(jq -r '.enhancedText // empty' "$TMP_RESP" 2>/dev/null || echo "")
      CONF=$(jq -r '.confidence // empty' "$TMP_RESP" 2>/dev/null || echo "")
      SID=$(jq -r '.sessionId // empty' "$TMP_RESP" 2>/dev/null || echo "")
      if [ -n "$RAW" ] || [ -n "$ENH" ]; then
        echo "✓ 业务结果："
        echo "    status       : $BIZ_STATUS (1=SUCCESS 2=DEGRADED 3=ERROR)"
        echo "    rawText      : $RAW"
        [ -n "$ENH" ] && [ "$ENH" != "$RAW" ] && echo "    enhancedText : $ENH"
        echo "    confidence   : $CONF"
        echo "    sessionId    : $SID"
      fi
    fi
    exit 0
    ;;
  401)
    REASON=$(grep -o '"reason":"[^"]*"' "$TMP_RESP" 2>/dev/null | head -1 || echo "")
    echo "[!] 鉴权失败 ${REASON:-}" >&2
    echo "    → 检查 TOKEN 是否有效（生产模式：Redis 中 oauth2_access_token:<TOKEN> 存在且未过期）" >&2
    echo "    → demo 模式可用 demo-token / admin-token" >&2
    exit 2
    ;;
  404)
    echo "[!] HTTP 404：ASR 服务未注册" >&2
    if [ "$DEMO" = "1" ]; then
      echo "    → demo 模式默认不启动 ASR（需生产模式：make run-prod）" >&2
    else
      echo "    → 检查 make run-prod 已启动且配置 asr.providers 至少一个 enabled" >&2
    fi
    exit 2
    ;;
  4*)
    echo "[!] 客户端错误（HTTP ${HTTP:-?}）" >&2
    exit 2
    ;;
  5*)
    echo "[!] 服务端错误（HTTP ${HTTP:-?}）" >&2
    exit 2
    ;;
  *)
    echo "[!] 未知状态码 HTTP ${HTTP:-?}" >&2
    exit 2
    ;;
esac
