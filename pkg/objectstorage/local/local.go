package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"backend-service/pkg/objectstorage"
)
func init() {
	objectstorage.Register("local", func(raw json.RawMessage) (objectstorage.Client, error) {
		var jc jsonConfig
		if err := json.Unmarshal(raw, &jc); err != nil {
			return nil, err
		}
		return New(jc.toConfig())
	})
}

// ───────────────────────────── Config ─────────────────────────────

// Config 本地文件存储配置
type Config struct {
	BasePath      string
	PublicBaseURL string
}

type jsonConfig struct {
	BasePath      string `json:"base_path"`
	PublicBaseURL string `json:"public_base_url"`
}

func (j jsonConfig) toConfig() Config {
	return Config{
		BasePath:      j.BasePath,
		PublicBaseURL: j.PublicBaseURL,
	}
}

// ───────────────────────────── Client ─────────────────────────────

type client struct {
	basePath      string
	publicBaseURL *url.URL
}

// New 创建本地文件存储客户端
func New(config Config) (*client, error) {
	basePath := strings.TrimSpace(config.BasePath)
	if basePath == "" {
		return nil, objectstorage.ErrInvalidConfig
	}
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}
	var publicBase *url.URL
	if strings.TrimSpace(config.PublicBaseURL) != "" {
		publicBase, err = url.Parse(strings.TrimRight(config.PublicBaseURL, "/"))
		if err != nil || publicBase.Scheme == "" || publicBase.Host == "" {
			return nil, objectstorage.ErrInvalidConfig
		}
	}
	return &client{basePath: abs, publicBaseURL: publicBase}, nil
}

func (c *client) PutObject(_ context.Context, bucket string, key string, body io.Reader, opts objectstorage.PutOptions) (*objectstorage.ObjectInfo, error) {
	if err := validateObject(bucket, key); err != nil {
		return nil, err
	}
	target, err := c.objectPath(bucket, key)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return nil, err
	}
	tmp := target + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return nil, err
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(file, io.TeeReader(body, hasher))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return nil, closeErr
	}
	if err = os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	_ = opts
	return &objectstorage.ObjectInfo{
		Bucket: bucket,
		Key:    key,
		ETag:   hex.EncodeToString(hasher.Sum(nil)),
		Size:   size,
	}, nil
}

func (c *client) DeleteObject(_ context.Context, bucket string, key string) error {
	if err := validateObject(bucket, key); err != nil {
		return err
	}
	target, err := c.objectPath(bucket, key)
	if err != nil {
		return err
	}
	if err = os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (c *client) PresignGetObject(_ context.Context, bucket string, key string, _ objectstorage.PresignOptions) (string, error) {
	return c.PublicURL(bucket, key)
}

func (c *client) PresignPutObject(context.Context, string, string, objectstorage.PresignOptions) (string, error) {
	return "", objectstorage.ErrUnsupportedProvider
}

func (c *client) PublicURL(bucket string, key string) (string, error) {
	if err := validateObject(bucket, key); err != nil {
		return "", err
	}
	if c.publicBaseURL == nil {
		return "", objectstorage.ErrUnsupportedProvider
	}
	u := *c.publicBaseURL
	basePath := strings.TrimRight(u.Path, "/")
	u.Path = path.Join(basePath, bucket, key)
	u.RawPath = path.Join(basePath, url.PathEscape(bucket), escapePath(key))
	if !strings.HasPrefix(u.RawPath, "/") {
		u.RawPath = "/" + u.RawPath
	}
	return u.String(), nil
}

func (c *client) objectPath(bucket string, key string) (string, error) {
	cleanBucket := filepath.Clean(bucket)
	cleanKey := filepath.Clean(filepath.FromSlash(key))
	if cleanBucket == "." || cleanKey == "." || strings.HasPrefix(cleanBucket, "..") || strings.HasPrefix(cleanKey, "..") || filepath.IsAbs(cleanBucket) || filepath.IsAbs(cleanKey) {
		return "", objectstorage.ErrInvalidObject
	}
	target := filepath.Join(c.basePath, cleanBucket, cleanKey)
	rel, err := filepath.Rel(c.basePath, target)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("%w: path escapes base", objectstorage.ErrInvalidObject)
	}
	return target, nil
}

func escapePath(value string) string {
	segments := strings.Split(value, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func validateObject(bucket string, key string) error {
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(key) == "" || strings.HasPrefix(key, "/") {
		return objectstorage.ErrInvalidObject
	}
	return nil
}
