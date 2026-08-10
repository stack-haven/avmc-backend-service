package objectstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// ───────────────────────────── Shared Errors ─────────────────────────────

var (
	ErrInvalidConfig       = errors.New("objectstorage: invalid config")
	ErrInvalidObject       = errors.New("objectstorage: invalid bucket or key")
	ErrUnsupportedProvider = errors.New("objectstorage: unsupported provider")
)

// ───────────────────────────── Shared Types ─────────────────────────────

// PutOptions 上传选项
type PutOptions struct {
	ContentType string
	Metadata    map[string]string
}

// PresignOptions 预签名选项
type PresignOptions struct {
	ContentType string
	Expires     time.Duration
	Metadata    map[string]string
}

// ObjectInfo 对象信息
type ObjectInfo struct {
	Bucket string
	Key    string
	ETag   string
	Size   int64
}

// ───────────────────────────── Client Interface ─────────────────────────────

// Client 对象存储客户端接口
type Client interface {
	PutObject(context.Context, string, string, io.Reader, PutOptions) (*ObjectInfo, error)
	DeleteObject(context.Context, string, string) error
	PresignGetObject(context.Context, string, string, PresignOptions) (string, error)
	PresignPutObject(context.Context, string, string, PresignOptions) (string, error)
	PublicURL(string, string) (string, error)
}

// ───────────────────────────── 供应商工厂 ─────────────────────────────

// Factory 存储后端构造函数。raw 为 provider 专属配置的 JSON。
type Factory func(raw json.RawMessage) (Client, error)

var (
	factories   = make(map[string]Factory)
	factoriesMu sync.RWMutex
)

// Register 注册存储后端工厂。子包应在 init() 中调用。
func Register(typ string, factory Factory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[typ] = factory
}

// NewClient 根据 type 创建对象存储客户端。configJSON 为 provider 专属配置。
// 使用方需在 main 或 bootstrap 中空白导入子包以触发注册：
//
//	import _ "backend-service/pkg/objectstorage/s3"
//	import _ "backend-service/pkg/objectstorage/local"
func NewClient(typ string, configJSON json.RawMessage) (Client, error) {
	factoriesMu.RLock()
	factory, ok := factories[typ]
	factoriesMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, typ)
	}
	return factory(configJSON)
}
