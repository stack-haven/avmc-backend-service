# SSE 服务使用文档

## 简介

SSE (Server-Sent Events) 是一种服务器推送技术，允许服务器向客户端发送实时数据。本服务集成了 SSE 功能，用于支持 AI 聊天的流式响应。

## 配置

### 配置文件

在 `proto/common/conf/server.proto` 文件中，可以配置 SSE 服务的相关参数：

```proto
// SSE
message SSE {
  string network = 1; // 网络
  string addr = 2;  // 服务监听地址
  string path = 3;  // 路径
  google.protobuf.Duration timeout = 4; // 超时时间
  bool enable_metrics = 5;  // 启用指标监控
}

// 在 Server 消息中添加
SSE sse = 7;  // SSE服务
```

### 配置示例

```yaml
server:
  http:
    addr: 0.0.0.0:8000
  sse:
    addr: 0.0.0.0:8000
    path: /api/v1/chat/stream
    timeout: 10s
    enable_metrics: true
```

## 使用方法

### 客户端连接

客户端可以通过以下方式连接到 SSE 服务：

```javascript
const eventSource = new EventSource('/api/v1/chat/stream');

eventSource.onmessage = function(event) {
  console.log('收到消息:', event.data);
};

eventSource.onerror = function(error) {
  console.error('连接错误:', error);
  eventSource.close();
};
```

### 服务器端发布消息

服务器端可以通过以下方式向客户端发布消息：

```go
// 获取 SSE 服务器实例
var sseServer *sse.Server

// 发布消息
streamID := sse.StreamID("test-stream")
testData := map[string]string{"message": "Hello, SSE!"}
err := sseServer.PublishData(context.Background(), streamID, testData)
if err != nil {
  log.Errorf("发布消息失败: %v", err)
}
```

### AI 聊天流式响应

在 AI 聊天服务中，可以使用 SSE 实现流式响应：

```go
// StreamChat 处理 AI 聊天的流式响应
func (s *ChatServiceService) StreamChat(ctx context.Context, req *pbCore.StreamChatRequest) (*pbCore.StreamChatResponse, error) {
  if req.GetMessage() == "" {
    return nil, errors.New(1001, "消息内容不能为空", "message content is required")
  }
  s.log.Infof("处理 AI 聊天流式请求，消息内容：%v", req.GetMessage())
  
  // 调用 AI 服务获取流式响应
  // 然后通过 SSE 发送给客户端
  
  return &pbCore.StreamChatResponse{}, nil
}
```

## 监控指标

SSE 服务提供了以下监控指标：

- `active_connections`: 当前活跃连接数
- `events_sent`: 已发送的事件数
- `error_count`: 错误数
- `connections_opened`: 已打开的连接数
- `connections_closed`: 已关闭的连接数
- `total_streams`: 总流数

可以通过访问 `/metrics` 端点获取这些指标：

```bash
curl http://localhost:8000/metrics
```

## 健康检查

可以通过访问 `/healthz` 端点检查 SSE 服务的健康状态：

```bash
curl http://localhost:8000/healthz
```

## 示例代码

### 客户端示例

```javascript
// 创建 SSE 连接
const eventSource = new EventSource('/api/v1/chat/stream');

// 处理消息
eventSource.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('收到 AI 回复:', data.message);
  
  // 更新 UI
  document.getElementById('chat-messages').innerHTML += `<div class="message ai">${data.message}</div>`;
};

// 处理错误
eventSource.onerror = function(error) {
  console.error('SSE 连接错误:', error);
  eventSource.close();
};

// 发送消息
function sendMessage() {
  const message = document.getElementById('message-input').value;
  if (!message) return;
  
  // 更新 UI
  document.getElementById('chat-messages').innerHTML += `<div class="message user">${message}</div>`;
  document.getElementById('message-input').value = '';
  
  // 发送请求到服务器
  fetch('/api/v1/chat', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ message })
  })
  .then(response => response.json())
  .then(data => {
    console.log('请求成功:', data);
  })
  .catch(error => {
    console.error('请求失败:', error);
  });
}
```

### 服务器端示例

```go
// 在 HTTP 服务器中注册 SSE 端点
func NewHTTPServer(c *conf.Server, logger log.Logger, authenticator authnEngine.Authenticator, authorizer authzEngine.Authorizer, sseServer *sse.Server, authService *service.AuthServiceService, userService *service.UserServiceService, deptService *service.DeptServiceService, menuService *service.MenuServiceService, roleService *service.RoleServiceService, postService *service.PostServiceService) *http.Server {
  // ... 其他代码
  
  // 注册 SSE 聊天端点
  srv.HandleFunc("/api/v1/chat/stream", func(w http.ResponseWriter, r *http.Request) {
    sseServer.ServeHTTP(w, r)
  })
  
  // ... 其他代码
  
  return srv
}

// 在 chat 服务中实现流式响应
func (s *ChatServiceService) StreamChat(ctx context.Context, req *pbCore.StreamChatRequest) (*pbCore.StreamChatResponse, error) {
  if req.GetMessage() == "" {
    return nil, errors.New(1001, "消息内容不能为空", "message content is required")
  }
  s.log.Infof("处理 AI 聊天流式请求，消息内容：%v", req.GetMessage())
  
  // 生成流 ID
  streamID := sse.StreamID(strconv.FormatInt(time.Now().UnixNano(), 10))
  
  // 模拟 AI 响应
  go func() {
    responses := []string{"你好！", "我是 AI 助手。", "有什么我可以帮助你的吗？"}
    for _, response := range responses {
      time.Sleep(1 * time.Second)
      s.sseServer.PublishData(context.Background(), streamID, map[string]string{"message": response})
    }
  }()
  
  return &pbCore.StreamChatResponse{StreamId: string(streamID)}, nil
}
```

## 注意事项

1. SSE 连接默认是长连接，会保持打开状态，直到客户端关闭或服务器出错。
2. SSE 只支持服务器向客户端单向通信，客户端不能通过 SSE 向服务器发送数据。
3. 如果需要双向通信，可以考虑使用 WebSocket。
4. SSE 连接可能会被代理或防火墙关闭，建议在客户端实现重连机制。
5. 对于大型应用，建议使用负载均衡器来分发 SSE 连接。
