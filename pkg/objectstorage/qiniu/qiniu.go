package qiniu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	AccessKey    string `json:"access_key"`
	SecretKey    string `json:"secret_key"`
	Zone         string `json:"zone,omitempty"` // "z0"/"z1"/"z2"/"na0" — auto if empty
	UseHTTPS     bool   `json:"use_https"`
	PublicBase   string `json:"public_base_url,omitempty"`
}

type client struct {
	cfg         Config
	mac         *auth.Credentials
	bucketMgr   *storage.BucketManager
	cfgStorage  storage.Config
	publicBase  string
}

func New(cfg Config) (*client, error) {
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("%w: access_key, secret_key required", objectstorage.ErrInvalidConfig)
	}
	mac := auth.New(cfg.AccessKey, cfg.SecretKey)
	cfgStorage := storage.Config{UseHTTPS: cfg.UseHTTPS}
	if cfg.Zone != "" {
		region, err := storage.GetRegion(cfg.AccessKey, "")
		if err == nil && region != nil {
			if z, ok := region.Zones[cfg.Zone]; ok {
				cfgStorage.Zone = z
				cfgStorage.Region = region
			}
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
	upToken := c.uploadToken(bucket, key)
	formUploader := storage.NewFormUploader(&c.cfgStorage)
	putExtra := storage.PutExtra{}
	if len(opts.Metadata) > 0 {
		putExtra.Metadata = opts.Metadata
	}
	ret := storage.PutRet{}
	if err := formUploader.Put(ctx, &ret, upToken, key, body, -1, &putExtra); err != nil {
		return nil, fmt.Errorf("kodo: put: %w", err)
	}
	return &objectstorage.ObjectInfo{
		Bucket: bucket, Key: key, ETag: ret.Hash, Size: ret.Fsize,
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
	url := storage.MakePublicURLv2(c.domain(bucket), key)
	token := auth.New(c.cfg.AccessKey, c.cfg.SecretKey)
	signedURL := token.Privateurl(url, deadline)
	if c.publicBase != "" {
		signedURL = c.publicURL(key)
	}
	return signedURL, nil
}

func (c *client) PresignPutObject(_ context.Context, bucket string, key string, opts objectstorage.PresignOptions) (string, error) {
	upToken := c.uploadToken(bucket, key)
	return fmt.Sprintf("%s?token=%s", c.publicURL(key), upToken), nil
}

func (c *client) PublicURL(bucket string, key string) (string, error) {
	if c.publicBase == "" {
		return "", fmt.Errorf("kodo: public_base_url not configured")
	}
	return c.publicURL(key), nil
}

func (c *client) domain(bucket string) string {
	return fmt.Sprintf("%s.%s", bucket, c.cfgStorage.Region.SrcUpHosts[0])
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
var _ = auth.Token // trigger type check
var _ = http.StatusOK
