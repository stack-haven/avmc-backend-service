package qiniu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"backend-service/pkg/objectstorage"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"
)

func init() {
	objectstorage.Register("qiniu-kodo", func(raw json.RawMessage) (objectstorage.Client, error) {
		var cfg Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		return New(cfg)
	})
}

type Config struct {
	AccessKey  string `json:"access_key"`
	SecretKey  string `json:"secret_key"`
	Zone       string `json:"zone,omitempty"` // "z0"/"z1"/"z2"/"na0" — auto if empty
	UseHTTPS   bool   `json:"use_https"`
	PublicBase string `json:"public_base_url,omitempty"`
}

type client struct {
	cfg        Config
	mac        *auth.Credentials
	bucketMgr  *storage.BucketManager
	cfgStorage storage.Config
	publicBase string
}

func New(cfg Config) (*client, error) {
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("%w: access_key, secret_key required", objectstorage.ErrInvalidConfig)
	}
	mac := auth.New(cfg.AccessKey, cfg.SecretKey)
	cfgStorage := storage.Config{UseHTTPS: cfg.UseHTTPS}
	if cfg.Zone != "" {
		if region, ok := storage.GetRegionByID(storage.RegionID(cfg.Zone)); ok {
			cfgStorage.Region = &region
			cfgStorage.Zone = &region
		}
	}
	bucketMgr := storage.NewBucketManager(mac, &cfgStorage)
	return &client{
		cfg:        cfg,
		mac:        mac,
		bucketMgr:  bucketMgr,
		cfgStorage: cfgStorage,
		publicBase: strings.TrimRight(cfg.PublicBase, "/"),
	}, nil
}

func (c *client) PutObject(ctx context.Context, bucket string, key string, body io.Reader, opts objectstorage.PutOptions) (*objectstorage.ObjectInfo, error) {
	// form 上传需要知道文件大小，先读入内存计算，避免 qiniu PutRet 不返回 size。
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("kodo: read body: %w", err)
	}
	size := int64(len(data))
	upToken := c.uploadToken(bucket, key)
	formUploader := storage.NewFormUploader(&c.cfgStorage)
	putExtra := storage.PutExtra{MimeType: opts.ContentType}
	if len(opts.Metadata) > 0 {
		putExtra.Params = make(map[string]string, len(opts.Metadata))
		for k, v := range opts.Metadata {
			putExtra.Params["x-qn-meta-"+k] = v
		}
	}
	ret := storage.PutRet{}
	if err := formUploader.Put(ctx, &ret, upToken, key, bytes.NewReader(data), size, &putExtra); err != nil {
		return nil, fmt.Errorf("kodo: put: %w", err)
	}
	return &objectstorage.ObjectInfo{
		Bucket: bucket, Key: key, ETag: ret.Hash, Size: size,
	}, nil
}

func (c *client) DeleteObject(ctx context.Context, bucket string, key string) error {
	if err := c.bucketMgr.Delete(bucket, key); err != nil {
		return fmt.Errorf("kodo: delete: %w", err)
	}
	return nil
}

func (c *client) PresignGetObject(_ context.Context, bucket string, key string, opts objectstorage.PresignOptions) (string, error) {
	expires := opts.Expires
	if expires <= 0 || expires > 7*24*time.Hour {
		expires = time.Hour
	}
	deadline := time.Now().Add(expires).Unix()
	domain := c.domain(bucket)
	return storage.MakePrivateURL(c.mac, domain, key, deadline), nil
}

func (c *client) PresignPutObject(_ context.Context, bucket string, key string, opts objectstorage.PresignOptions) (string, error) {
	upToken := c.uploadToken(bucket, key)
	return fmt.Sprintf("%s?token=%s", c.publicURL(key), upToken), nil
}

// GetObject 对象存储渠道不支持后端代理读取，下载走预签名 URL。
func (*client) GetObject(context.Context, string, string) ([]byte, error) {
	return nil, objectstorage.ErrUnsupportedProvider
}

func (c *client) PublicURL(bucket string, key string) (string, error) {
	if c.publicBase == "" {
		return "", fmt.Errorf("kodo: public_base_url not configured")
	}
	return c.publicURL(key), nil
}

// domain 返回下载域名：优先公开基础域名，否则回退到上传入口。
func (c *client) domain(bucket string) string {
	if c.publicBase != "" {
		return c.publicBase
	}
	if c.cfgStorage.Region != nil && len(c.cfgStorage.Region.SrcUpHosts) > 0 {
		return fmt.Sprintf("%s.%s", bucket, c.cfgStorage.Region.SrcUpHosts[0])
	}
	return bucket
}

func (c *client) publicURL(key string) string {
	return fmt.Sprintf("%s/%s", c.publicBase, key)
}

func (c *client) uploadToken(bucket, key string) string {
	putPolicy := storage.PutPolicy{
		Scope:   fmt.Sprintf("%s:%s", bucket, key),
		Expires: 3600,
	}
	return putPolicy.UploadToken(c.mac)
}

// Ensure interface compliance.
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

