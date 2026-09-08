# evie/tool 前端演示 Demo

为 evie/tool HTTP 接口提供的两个零依赖前端演示页面，可直接双击打开：

- **`html/`** — 纯 HTML + JS + CSS（单文件，无构建工具）
- **`vue/`** — Vue 3 单文件组件（CDN 引入，无需 npm install）

两个 demo 都演示 evie/tool 的核心 HTTP 接口：

| 端点 | 方法 | 用途 |
|---|---|---|
| `/evie/tool/v1/asr:recognize` | POST | 音频 → 文本 + 增强 |
| `/evie/tool/v1/enhance` | POST | 纯文本增强（不发音频） |
| `/health/ready` | GET | 服务健康检查（含 redis / qua / asr 依赖） |

---

## 1. HTML 版本（推荐演示用）

```
html/
├── index.html        # 单页入口（双击即可打开）
├── app.js            # 业务逻辑（fetch + UI 渲染）
└── style.css         # 样式
```

**运行方式：**

```bash
# 直接打开
open html/index.html

# 或启动本地 http server（避免 file:// CORS 问题）
cd html && python3 -m http.server 8080
# 访问 http://localhost:8080
```

**特性：**
- ✅ 零依赖（无 Vue / React / npm）
- ✅ 单文件友好（js / css 可内联，本目录已分离便于阅读）
- ✅ 文件拖拽 / 选择 / URL 三种音频输入方式
- ✅ Base64 编码后 JSON POST 到 `/evie/tool/v1/asr:recognize`
- ✅ 原始文本 vs 增强文本 diff 高亮
- ✅ 13 个时间字段可视化（每层 processor 耗时）
- ✅ 错误状态可视化（HTTP 401 / 422 / 5xx）

---

## 2. Vue 3 版本（参考生产集成）

```
vue/
├── index.html        # Vue 3 CDN 引入（无需构建）
├── app.js            # Composition API（响应式 state）
├── components.js     # 单文件组件风格的 template render 函数
└── style.css         # 设计 token + 响应式
```

**运行方式：**

```bash
cd vue && python3 -m http.server 8080
# 访问 http://localhost:8080
```

**特性：**
- ✅ Vue 3 Composition API + 响应式 state
- ✅ 组件拆分（AudioUploader / TextEnhancer / DiffViewer / StatusPanel）
- ✅ Vben Admin / Ant Design 风格的卡片化布局
- ✅ 同样的接口 + 字段覆盖
- ✅ 可作为正式前端项目（Vben Admin / Ant Design Vue）集成的参考

---

## 接口速览

### 通用请求头

```http
Authorization: Bearer <token>
Content-Type: application/json
```

### POST `/evie/tool/v1/asr:recognize`

**请求：**
```json
{
  "format": {"encoding": "wav", "sampleRate": 16000, "bitDepth": 16, "channels": 1},
  "audioData": "<base64 编码音频>",
  "sessionId": "demo-001",
  "enableEnhancement": true
}
```

**响应：**
```json
{
  "requestId": "uuid",
  "sessionId": "demo-001",
  "rawText": "金种子 是 我们的 主要 产品",
  "enhancedText": "金种籽 是 我们的 主要 产品",
  "changes": [
    {"original": "金种子", "replacement": "金种籽", "kind": "vocab", "confidence": 0.95}
  ],
  "status": 1,
  "providerName": "funasr",
  "confidence": 0.92,
  "durationMs": 12345,
  "processingTimeMs": 67,
  "cleaningTimeMs": 2,
  "fillerTimeMs": 1,
  "vocabMatchTimeMs": 12,
  "aliasTimeMs": 3,
  "deterministicTimeMs": 5,
  "pinyinTimeMs": 8,
  "fuzzyTimeMs": 24,
  "contextTimeMs": 12
}
```

### POST `/evie/tool/v1/enhance`

**请求：**
```json
{"text": "金种子 是 我们的 主要 产品"}
```

**响应：**
```json
{
  "originalText": "金种子 是 我们的 主要 产品",
  "enhancedText": "金种籽 是 我们的 主要 产品",
  "changes": [{"original": "金种子", "replacement": "金种籽", "kind": "vocab", "confidence": 0.95}],
  "status": 1,
  "processingTimeMs": 67
}
```

---

## 配置

进入 demo 页面后，顶部有 **配置区**：

- **Base URL**：默认 `http://localhost:8110`（evie/tool 默认端口）
- **Token**：Bearer Token 必须是 **OAuth access_token**（存在 Redis db=14，key 前缀 `oauth2_access_token:`），**不是 qua 平台 sync_token**。混淆这两者会得到 `401 TOKEN_INVALID`。
- **音频格式**：mp3 / wav / pcm

### 401 TOKEN_INVALID 排查

```bash
# 1) 确认 token 存在于 Redis（设 REDIS_PASSWORD 来自 .env.local）
REDIS_PASSWORD=xxx go run ./cmd/redis_check <token>
# → ✓ FOUND key=oauth2_access_token:<token>  表示是 access_token，可用作 Bearer
# → ✗ key NOT found                                  表示 token 不在 Redis
```

**常见混淆**：
| Token 类型 | 来源 | 是否可作 Bearer |
|---|---|---|
| OAuth access_token | Redis `oauth2_access_token:*`，用户登录后下发 | ✅ 可作 Bearer |
| qua sync_token | qua 平台内部用，存于 `configs/tenants.json` 的 `sync_token` 字段 | ❌ 仅服务内部使用 |
| qua client credentials | qua 服务自己的 client id/secret | ❌ 不用于本服务 |

---

## 截图示意

```
┌─────────────────────────────────────────────────────┐
│  evie/tool Demo                          [● OK]      │
├─────────────────────────────────────────────────────┤
│  Base URL: [http://localhost:8110]                   │
│  Token:    [182ed78960304d90a...]                    │
├─────────────────────────────────────────────────────┤
│  ┌── ASR + 增强 ──┐  ┌── 纯文本增强 ──┐              │
│  │ 拖拽 / 选择音频 │  │ 输入框:        │              │
│  │ [▶ 识别]        │  │ 金种子是...    │              │
│  │                 │  │ [▶ 增强]        │              │
│  └─────────────────┘  └────────────────┘              │
├─────────────────────────────────────────────────────┤
│  原始: 金种子 是 我们的 主要 产品                      │
│  增强: 金种籽 是 我们的 主要 产品                      │
│  改动: 金种子→金种籽 (vocab, 0.95)                   │
├─────────────────────────────────────────────────────┤
│  耗时: 总 67ms · cleaning 2 · vocab 12 · fuzzy 24 ... │
└─────────────────────────────────────────────────────┘
```
