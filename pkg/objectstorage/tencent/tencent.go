package tencent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	cli *cos.Client
}

func New(cfg Config) (*client, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" || cfg.Region == "" {
		return nil, fmt.Errorf("%w: secret_id, secret_key, region required", objectstorage.ErrInvalidConfig)
	}
	scheme := "https"
	if !cfg.UseSSL {
		scheme = "http"
	}
	bucketURL, err := url.Parse(fmt.Sprintf("%s://%s.cos.%s.myqcloud.com", scheme, cfg.AppID+"-"+cfg.AppID, cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("cos: parse bucket url: %w", err)
	}
	baseURL := &cos.BaseURL{BucketURL: bucketURL}
	cli := cos.NewClient(baseURL, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})
	return &client{cfg: cfg, cli: cli}, nil
}

func (c *client) PutObject(ctx context.Context, bucket string, key string, body io.Reader, opts objectstorage.PutOptions) (*objectstorage.ObjectInfo, error) {
	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: opts.ContentType},
	}
	if len(opts.Metadata) > 0 {
		opt.ObjectPutHeaderOptions.XCosMetaXXX = &http.Header{}
		for k, v := range opts.Metadata {
			opt.ObjectPutHeaderOptions.XCosMetaXXX.Set("x-cos-meta-"+k, v)
		}
	}
	resp, err := c.cli.Object.Put(ctx, key, body, opt)
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
	_, err := c.cli.Object.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("cos: delete: %w", err)
	}
	return nil
}

func (c *client) PresignGetObject(_ context.Context, bucket string, key string, opts objectstorage.PresignOptions) (string, error) {
	expires := opts.Expires
	if expires <= 0 || expires > 7*24*time.Hour {
		expires = time.Hour
	}
	u, err := c.cli.Object.GetPresignedURL(context.Background(), http.MethodGet, key, c.cfg.SecretID, c.cfg.SecretKey, expires, nil)
	if err != nil {
		return "", fmt.Errorf("cos: presign get: %w", err)
	}
	if c.cfg.PublicBase != "" {
		return c.publicURL(key), nil
	}
	return u.String(), nil
}

func (c *client) PresignPutObject(_ context.Context, bucket string, key string, opts objectstorage.PresignOptions) (string, error) {
	expires := opts.Expires
	if expires <= 0 || expires > 7*24*time.Hour {
		expires = time.Hour
	}
	var hdrs *http.Header
	if opts.ContentType != "" {
		hdrs = &http.Header{}
		hdrs.Set("Content-Type", opts.ContentType)
	}
	u, err := c.cli.Object.GetPresignedURL(context.Background(), http.MethodPut, key, c.cfg.SecretID, c.cfg.SecretKey, expires, hdrs)
	if err != nil {
		return "", fmt.Errorf("cos: presign put: %w", err)
	}
	if c.cfg.PublicBase != "" {
		return c.publicURL(key), nil
	}
	return u.String(), nil
}

func (c *client) PublicURL(bucket string, key string) (string, error) {
	return c.publicURL(key), nil
}

func (c *client) publicURL(key string) string {
	return fmt.Sprintf("%s/%s", strings.TrimRight(c.cfg.PublicBase, "/"), key)
}

var _ objectstorage.Client = (*client)(nil)
