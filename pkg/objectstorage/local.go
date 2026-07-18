package objectstorage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type LocalClient struct {
	basePath      string
	publicBaseURL *url.URL
}

func NewLocalClient(config Config) (*LocalClient, error) {
	basePath := strings.TrimSpace(config.LocalBasePath)
	if basePath == "" {
		basePath = strings.TrimSpace(config.Endpoint)
	}
	if basePath == "" {
		return nil, ErrInvalidConfig
	}
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}
	var publicBase *url.URL
	if strings.TrimSpace(config.PublicBaseURL) != "" {
		publicBase, err = url.Parse(strings.TrimRight(config.PublicBaseURL, "/"))
		if err != nil || publicBase.Scheme == "" || publicBase.Host == "" {
			return nil, ErrInvalidConfig
		}
	}
	return &LocalClient{basePath: abs, publicBaseURL: publicBase}, nil
}

func (c *LocalClient) PutObject(_ context.Context, bucket string, key string, body io.Reader, opts PutOptions) (*ObjectInfo, error) {
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
	return &ObjectInfo{
		Bucket: bucket,
		Key:    key,
		ETag:   hex.EncodeToString(hasher.Sum(nil)),
		Size:   size,
	}, nil
}

func (c *LocalClient) DeleteObject(_ context.Context, bucket string, key string) error {
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

func (c *LocalClient) PresignGetObject(_ context.Context, bucket string, key string, _ PresignOptions) (string, error) {
	return c.PublicURL(bucket, key)
}

func (c *LocalClient) PresignPutObject(context.Context, string, string, PresignOptions) (string, error) {
	return "", ErrUnsupportedProvider
}

func (c *LocalClient) PublicURL(bucket string, key string) (string, error) {
	if err := validateObject(bucket, key); err != nil {
		return "", err
	}
	if c.publicBaseURL == nil {
		return "", ErrUnsupportedProvider
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

func (c *LocalClient) objectPath(bucket string, key string) (string, error) {
	cleanBucket := filepath.Clean(bucket)
	cleanKey := filepath.Clean(filepath.FromSlash(key))
	if cleanBucket == "." || cleanKey == "." || strings.HasPrefix(cleanBucket, "..") || strings.HasPrefix(cleanKey, "..") || filepath.IsAbs(cleanBucket) || filepath.IsAbs(cleanKey) {
		return "", ErrInvalidObject
	}
	target := filepath.Join(c.basePath, cleanBucket, cleanKey)
	rel, err := filepath.Rel(c.basePath, target)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("%w: path escapes base", ErrInvalidObject)
	}
	return target, nil
}
