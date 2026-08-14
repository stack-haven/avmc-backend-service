package aliyun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"backend-service/pkg/objectstorage"
)

func init() {
	objectstorage.Register("aliyun-oss", func(raw json.RawMessage) (objectstorage.Client, error) {
		var cfg Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		return New(cfg)
	})
}

// ───────────────────────────── Config ─────────────────────────────

// Config 阿里云 OSS 配置
type Config struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	UseSSL          bool   `json:"use_ssl"`
	PublicBaseURL   string `json:"public_base_url,omitempty"`
}

// ───────────────────────────── Client ─────────────────────────────

type client struct {
	cfg  Config
	conn *oss.Client
}

func New(cfg Config) (*client, error) {
	if cfg.Endpoint == "" || cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, fmt.Errorf("%w: endpoint, access_key_id, access_key_secret required", objectstorage.ErrInvalidConfig)
	}
	conn, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("oss: connect: %w", err)
	}
	return &client{cfg: cfg, conn: conn}, nil
}

func (c *client) bucket(name string) (*oss.Bucket, error) {
	return c.conn.Bucket(name)
}

func (c *client) PutObject(ctx context.Context, bucket string, key string, body io.Reader, opts objectstorage.PutOptions) (*objectstorage.ObjectInfo, error) {
	b, err := c.bucket(bucket)
	if err != nil {
		return nil, fmt.Errorf("oss: put: %w", err)
	}
	ossOpts := []oss.Option{}
	if opts.ContentType != "" {
		ossOpts = append(ossOpts, oss.ContentType(opts.ContentType))
	}
	for k, v := range opts.Metadata {
		ossOpts = append(ossOpts, oss.Meta(k, v))
	}
	if err := b.PutObject(key, body, ossOpts...); err != nil {
		return nil, fmt.Errorf("oss: put: %w", err)
	}
	header, err := b.GetObjectDetailedMeta(key)
	var etag string
	var size int64
	if err == nil {
		etag = strings.Trim(header.Get("ETag"), `"`)
		if cl := header.Get("Content-Length"); cl != "" {
			fmt.Sscanf(cl, "%d", &size)
		}
	}
	return &objectstorage.ObjectInfo{Bucket: bucket, Key: key, ETag: etag, Size: size}, nil
}

func (c *client) DeleteObject(ctx context.Context, bucket string, key string) error {
	b, err := c.bucket(bucket)
	if err != nil {
		return fmt.Errorf("oss: delete: %w", err)
	}
	if err := b.DeleteObject(key); err != nil {
		return fmt.Errorf("oss: delete: %w", err)
	}
	return nil
}

func (c *client) PresignGetObject(_ context.Context, bucket string, key string, opts objectstorage.PresignOptions) (string, error) {
	b, err := c.bucket(bucket)
	if err != nil {
		return "", fmt.Errorf("oss: presign: %w", err)
	}
	expires := opts.Expires
	if expires <= 0 || expires > 7*24*time.Hour {
		expires = time.Hour
	}
	url, err := b.SignURL(key, oss.HTTPGet, int64(expires.Seconds()))
	if err != nil {
		return "", fmt.Errorf("oss: presign get: %w", err)
	}
	if c.cfg.PublicBaseURL != "" {
		url = strings.Replace(url, c.cfg.Endpoint, c.cfg.PublicBaseURL, 1)
	}
	return url, nil
}

func (c *client) PresignPutObject(_ context.Context, bucket string, key string, opts objectstorage.PresignOptions) (string, error) {
	b, err := c.bucket(bucket)
	if err != nil {
		return "", fmt.Errorf("oss: presign: %w", err)
	}
	expires := opts.Expires
	if expires <= 0 || expires > 7*24*time.Hour {
		expires = time.Hour
	}
	signOpts := []oss.Option{oss.Expires(time.Now().Add(expires))}
	if opts.ContentType != "" {
		signOpts = append(signOpts, oss.ContentType(opts.ContentType))
	}
	url, err := b.SignURL(key, oss.HTTPPut, int64(expires.Seconds()), signOpts...)
	if err != nil {
		return "", fmt.Errorf("oss: presign put: %w", err)
	}
	if c.cfg.PublicBaseURL != "" {
		url = strings.Replace(url, c.cfg.Endpoint, c.cfg.PublicBaseURL, 1)
	}
	return url, nil
}

// GetObject 对象存储渠道不支持后端代理读取，下载走预签名 URL。
func (*client) GetObject(context.Context, string, string) ([]byte, error) {
	return nil, objectstorage.ErrUnsupportedProvider
}

func (c *client) PublicURL(bucket string, key string) (string, error) {
	if c.cfg.PublicBaseURL == "" {
		return "", fmt.Errorf("oss: public_base_url not configured")
	}
	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(c.cfg.PublicBaseURL, "/"), bucket, key), nil
}

// Ensure client implements the interface.
var _ objectstorage.Client = (*client)(nil)

// ── Multipart 预留：对象存储渠道未来实现原生分片上传 ──
func (*client) CreateMultipartUpload(context.Context, string, string) (string, error) {
	return "", objectstorage.ErrUnsupportedProvider
}

func (*client) UploadPart(context.Context, string, string, string, int32, io.Reader, objectstorage.PutOptions) (string, error) {
	return "", objectstorage.ErrUnsupportedProvider
}


func (*client) ListMultipartParts(context.Context, string, string, string) ([]objectstorage.MultipartPart, error) {
	return nil, objectstorage.ErrUnsupportedProvider
}

func (*client) CompleteMultipartUpload(context.Context, string, string, string, []objectstorage.MultipartPart) (string, error) {
	return "", objectstorage.ErrUnsupportedProvider
}

func (*client) AbortMultipartUpload(context.Context, string, string, string) error {
	return objectstorage.ErrUnsupportedProvider
}

