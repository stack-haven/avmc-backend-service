package tencent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"backend-service/pkg/objectstorage"

	"github.com/tencentyun/cos-go-sdk-v5"
)

func init() {
	objectstorage.Register("tencent-cos", func(raw json.RawMessage) (objectstorage.Client, error) {
		var cfg Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		return New(cfg)
	})
}

type Config struct {
	AppID      string `json:"app_id"`
	Region     string `json:"region"`
	SecretID   string `json:"secret_id"`
	SecretKey  string `json:"secret_key"`
	UseSSL     bool   `json:"use_ssl"`
	PublicBase string `json:"public_base_url,omitempty"`
}

type client struct {
	cfg Config

	mu      sync.RWMutex
	buckets map[string]*cos.Client
}

func New(cfg Config) (*client, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" || cfg.Region == "" {
		return nil, fmt.Errorf("%w: secret_id, secret_key, region required", objectstorage.ErrInvalidConfig)
	}
	return &client{cfg: cfg, buckets: make(map[string]*cos.Client)}, nil
}

// cosClient 按 bucket 动态构建 COS client。
// 腾讯 COS 的 bucket URL 格式为 {bucket}-{appid}.cos.{region}.myqcloud.com，
// bucket 是运行时传入的，不能固定在一个 client 上。
func (c *client) cosClient(bucket string) (*cos.Client, error) {
	c.mu.RLock()
	if cli, ok := c.buckets[bucket]; ok {
		c.mu.RUnlock()
		return cli, nil
	}
	c.mu.RUnlock()

	scheme := "https"
	if !c.cfg.UseSSL {
		scheme = "http"
	}
	host := fmt.Sprintf("%s-%s.cos.%s.myqcloud.com", bucket, c.cfg.AppID, c.cfg.Region)
	bucketURL, err := url.Parse(fmt.Sprintf("%s://%s", scheme, host))
	if err != nil {
		return nil, fmt.Errorf("cos: parse bucket url: %w", err)
	}
	cli := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  c.cfg.SecretID,
			SecretKey: c.cfg.SecretKey,
		},
	})

	c.mu.Lock()
	if existing, ok := c.buckets[bucket]; ok {
		c.mu.Unlock()
		return existing, nil
	}
	c.buckets[bucket] = cli
	c.mu.Unlock()
	return cli, nil
}

func (c *client) PutObject(ctx context.Context, bucket string, key string, body io.Reader, opts objectstorage.PutOptions) (*objectstorage.ObjectInfo, error) {
	cli, err := c.cosClient(bucket)
	if err != nil {
		return nil, err
	}
	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: opts.ContentType},
	}
	if len(opts.Metadata) > 0 {
		opt.ObjectPutHeaderOptions.XCosMetaXXX = &http.Header{}
		for k, v := range opts.Metadata {
			opt.ObjectPutHeaderOptions.XCosMetaXXX.Set("x-cos-meta-"+k, v)
		}
	}
	resp, err := cli.Object.Put(ctx, key, body, opt)
	if err != nil {
		return nil, fmt.Errorf("cos: put: %w", err)
	}
	var etag string
	var size int64
	if resp != nil {
		etag = strings.Trim(resp.Header.Get("ETag"), `"`)
		fmt.Sscanf(resp.Header.Get("Content-Length"), "%d", &size)
	}
	return &objectstorage.ObjectInfo{Bucket: bucket, Key: key, ETag: etag, Size: size}, nil
}

func (c *client) DeleteObject(ctx context.Context, bucket string, key string) error {
	cli, err := c.cosClient(bucket)
	if err != nil {
		return err
	}
	_, err = cli.Object.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("cos: delete: %w", err)
	}
	return nil
}

func (c *client) PresignGetObject(_ context.Context, bucket string, key string, opts objectstorage.PresignOptions) (string, error) {
	cli, err := c.cosClient(bucket)
	if err != nil {
		return "", err
	}
	expires := opts.Expires
	if expires <= 0 || expires > 7*24*time.Hour {
		expires = time.Hour
	}
	u, err := cli.Object.GetPresignedURL(context.Background(), http.MethodGet, key, c.cfg.SecretID, c.cfg.SecretKey, expires, nil)
	if err != nil {
		return "", fmt.Errorf("cos: presign get: %w", err)
	}
	if c.cfg.PublicBase != "" {
		return c.publicURL(key), nil
	}
	return u.String(), nil
}

func (c *client) PresignPutObject(_ context.Context, bucket string, key string, opts objectstorage.PresignOptions) (string, error) {
	cli, err := c.cosClient(bucket)
	if err != nil {
		return "", err
	}
	expires := opts.Expires
	if expires <= 0 || expires > 7*24*time.Hour {
		expires = time.Hour
	}
	var hdrs *http.Header
	if opts.ContentType != "" {
		hdrs = &http.Header{}
		hdrs.Set("Content-Type", opts.ContentType)
	}
	u, err := cli.Object.GetPresignedURL(context.Background(), http.MethodPut, key, c.cfg.SecretID, c.cfg.SecretKey, expires, hdrs)
	if err != nil {
		return "", fmt.Errorf("cos: presign put: %w", err)
	}
	if c.cfg.PublicBase != "" {
		return c.publicURL(key), nil
	}
	return u.String(), nil
}

func (c *client) PublicURL(_ string, key string) (string, error) {
	return c.publicURL(key), nil
}

// GetObject 对象存储渠道不支持后端代理读取，下载走预签名 URL。
func (*client) GetObject(context.Context, string, string) ([]byte, error) {
	return nil, objectstorage.ErrUnsupportedProvider
}

func (c *client) publicURL(key string) string {
	return fmt.Sprintf("%s/%s", strings.TrimRight(c.cfg.PublicBase, "/"), key)
}

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

