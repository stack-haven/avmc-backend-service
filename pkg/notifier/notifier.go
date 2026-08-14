package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ───────────────────────────── Shared Errors ─────────────────────────────

var (
	ErrInvalidConfig       = fmt.Errorf("notifier: invalid config")
	ErrUnsupportedChannel  = fmt.Errorf("notifier: unsupported channel")
	ErrRecipientRequired   = fmt.Errorf("notifier: recipient required")
)

// ───────────────────────────── Shared Types ─────────────────────────────

// Recipient 通知接收人。不同渠道使用不同字段：
//   - in-app：UserID
//   - sms：Phone
//   - email：Email
type Recipient struct {
	UserID uint32
	Phone  string
	Email  string
}

// Message 待发送的通知消息（渠道无关的抽象表示）。
type Message struct {
	Channel      string
	TenantID     uint32
	Title        string
	Content      string
	Variables    map[string]string
	Recipients   []Recipient
	TemplateID   uint32 // 站内信用：模板 ID（可选）
	TemplateCode string // 站内信用：模板 code（可选）
	BusinessType string
	BusinessID   string
	Priority     int32
	SenderUserID uint32
	SenderName   string
}

// Result 发送结果。
type Result struct {
	SuccessCount int
	FailCount    int
}

// Sender 通知发送器。每个渠道（in-app / sms / email / webhook）实现一个 Sender。
type Sender interface {
	// Channel 返回渠道标识（如 "in-app"、"sms"）。
	Channel() string
	// Send 发送一条通知。
	Send(context.Context, Message) (Result, error)
}

// Factory 渠道发送器构造函数。raw 为渠道专属配置 JSON。
type Factory func(raw json.RawMessage) (Sender, error)

// ───────────────────────────── 工厂注册 ─────────────────────────────

var (
	factories   = make(map[string]Factory)
	factoriesMu sync.RWMutex
)

// Register 注册渠道发送器工厂。各渠道实现在 init() 中调用。
func Register(channel string, factory Factory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[channel] = factory
}

// NewSender 根据渠道创建发送器。
func NewSender(channel string, config json.RawMessage) (Sender, error) {
	factoriesMu.RLock()
	factory, ok := factories[channel]
	factoriesMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedChannel, channel)
	}
	return factory(config)
}

// RegisteredChannels 返回已注册的渠道列表（供健康检查/能力枚举使用）。
func RegisteredChannels() []string {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	channels := make([]string, 0, len(factories))
	for channel := range factories {
		channels = append(channels, channel)
	}
	return channels
}
