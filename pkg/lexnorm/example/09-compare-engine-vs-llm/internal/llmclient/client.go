// Package llmclient 提供 DeepSeek API 的最小客户端（OpenAI 兼容协议）。
//
// 仅用于本 example 的演示。生产环境应使用成熟的 SDK（DeepSeek 官方 SDK 或
// OpenAI Go SDK），并加入 retry / circuit breaker / metrics / cache。
//
// 依赖：仅标准库（net/http + encoding/json）。
package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeepSeek 默认 endpoint（OpenAI 兼容）。
const DefaultDeepSeekBaseURL = "https://api.deepseek.com/v1"

// DefaultDeepSeekModel 推荐模型（中文强、价格低）。
const DefaultDeepSeekModel = "deepseek-chat"

// Message 是 OpenAI ChatCompletion 的单条消息。
type Message struct {
	Role    string `json:"role"` // system|user|assistant
	Content string `json:"content"`
}

// Request 是 ChatCompletion 请求体（只取需要的字段）。
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// Choice 是响应里的单个候选。
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage 是 token 用量（可选）。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Response 是 ChatCompletion 响应。
type Response struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Client 是 DeepSeek 客户端（也兼容任何 OpenAI ChatCompletion endpoint）。
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// New 构造 Client。apiKey 为空时返回 nil（让调用方早 fail）。
func New(baseURL, apiKey string) *Client {
	if apiKey == "" {
		return nil
	}
	if baseURL == "" {
		baseURL = DefaultDeepSeekBaseURL
	}
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Chat 发起一次对话，返回 (raw response content, full response)。
//
// 第二个返回值便于审计与统计 token 用量。
//
// 错误策略：网络错误返回 error；HTTP 非 2xx 返回带 body 的 error；
// JSON 解析错误返回 error。
func (c *Client) Chat(ctx context.Context, req Request) (string, *Response, error) {
	if c == nil {
		return "", nil, fmt.Errorf("llmclient: nil client (api key empty?)")
	}
	if req.Model == "" {
		req.Model = DefaultDeepSeekModel
	}
	if req.Temperature == 0 {
		req.Temperature = 0.0
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", nil, fmt.Errorf("http: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("http %d: %s", httpResp.StatusCode, string(respBody))
	}

	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", nil, fmt.Errorf("unmarshal: %w (body=%s)", err, truncate(string(respBody), 200))
	}
	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("no choices in response (body=%s)", truncate(string(respBody), 200))
	}

	return resp.Choices[0].Message.Content, &resp, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
