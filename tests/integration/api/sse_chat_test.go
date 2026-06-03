package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"backend-service/pkg/kratos/transport/sse"

	"github.com/stretchr/testify/assert"
)

func skipIfNetworkNotPermitted(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "operation not permitted") {
		t.Skipf("network is not permitted in this environment: %v", err)
	}
}

// TestSSEChatStreamCreation 测试 SSE 聊天的流创建功能
func TestSSEChatStreamCreation(t *testing.T) {
	// 创建 SSE 服务器
	server := sse.NewServer(
		sse.WithPath("/api/v1/chat/stream"),
		sse.WithAddress(":0"),
	)

	// 创建测试数据流
	streamID := sse.StreamID("test-stream")
	stream := server.CreateStream(streamID)

	// 检查流是否创建成功
	assert.NotNil(t, stream)
}

// TestSSEChatPublishData 测试 SSE 聊天的数据发布功能
func TestSSEChatPublishData(t *testing.T) {
	// 创建 SSE 服务器
	server := sse.NewServer(
		sse.WithPath("/api/v1/chat/stream"),
		sse.WithAddress(":0"),
	)

	// 创建测试数据流
	streamID := sse.StreamID("test-stream")
	testData := map[string]string{"message": "Hello, SSE!"}

	// 创建流
	server.CreateStream(streamID)

	// 发布测试数据
	err := server.PublishData(nil, streamID, testData)
	assert.NoError(t, err)
}

// TestSSEServerStartAndStop 测试 SSE 服务器的启动和停止功能
func TestSSEServerStartAndStop(t *testing.T) {
	// 创建 SSE 服务器
	server := sse.NewServer(
		sse.WithPath("/api/v1/chat/stream"),
		sse.WithAddress(":0"),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(context.Background())
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-errCh:
		skipIfNetworkNotPermitted(t, err)
		assert.NoError(t, err)
	default:
	}

	// 停止服务器
	err := server.Stop(context.Background())
	assert.NoError(t, err)
}

// TestSSEStreamSubscription 测试 SSE 流订阅功能
func TestSSEStreamSubscription(t *testing.T) {
	// 创建 SSE 服务器
	server := sse.NewServer(
		sse.WithPath("/api/v1/chat/stream"),
		sse.WithAddress(":7001"),
	)

	// 启动服务器
	go func() {
		if err := server.Start(context.Background()); err != nil {
			t.Logf("SSE server error: %v", err)
		}
	}()
	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)
	defer server.Stop(context.Background())

	// 模拟客户端订阅 SSE 流
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://localhost:7001/api/v1/chat/stream", nil)
	assert.NoError(t, err)

	// 设置 SSE 头部
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// 发送请求
	resp, err := client.Do(req)
	skipIfNetworkNotPermitted(t, err)
	if !assert.NoError(t, err) {
		return
	}
	defer resp.Body.Close()

	// 验证响应状态码
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

// TestSSEMultipleStreams 测试多个 SSE 流的管理功能
func TestSSEMultipleStreams(t *testing.T) {
	// 创建 SSE 服务器
	server := sse.NewServer(
		sse.WithPath("/api/v1/chat/stream"),
		sse.WithAddress(":0"),
	)

	// 创建多个测试数据流
	streamID1 := sse.StreamID("test-stream-1")
	streamID2 := sse.StreamID("test-stream-2")

	// 创建流
	stream1 := server.CreateStream(streamID1)
	stream2 := server.CreateStream(streamID2)

	// 检查流是否创建成功
	assert.NotNil(t, stream1)
	assert.NotNil(t, stream2)

	// 发布不同流的数据
	testData1 := map[string]string{"message": "Hello from stream 1"}
	testData2 := map[string]string{"message": "Hello from stream 2"}

	err1 := server.PublishData(nil, streamID1, testData1)
	err2 := server.PublishData(nil, streamID2, testData2)

	// 检查数据发布是否成功
	assert.NoError(t, err1)
	assert.NoError(t, err2)
}
