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
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

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
	// 提前验证目录可写，避免上传时才报 read-only file system。
	if err := ensureWritable(abs); err != nil {
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

// ensureWritable 创建目录并写入探针文件，验证路径可写。
func ensureWritable(basePath string) error {
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return fmt.Errorf("local storage: create base path: %w", err)
	}
	probe := filepath.Join(basePath, ".write-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o640); err != nil {
		return fmt.Errorf("local storage: base path not writable (%s): %w", basePath, err)
	}
	_ = os.Remove(probe)
	return nil
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

// GetObject 从本地磁盘读取对象完整内容。
func (c *client) GetObject(_ context.Context, bucket string, key string) ([]byte, error) {
	if err := validateObject(bucket, key); err != nil {
		return nil, err
	}
	target, err := c.objectPath(bucket, key)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (c *client) PresignPutObject(context.Context, string, string, objectstorage.PresignOptions) (string, error) {
	return "", objectstorage.ErrUnsupportedProvider
}

// ───────────────────────────── Multipart 分片上传 ─────────────────────────────
//
// 本地渠道用临时目录承载分片，uploadID 即临时目录绝对路径；合并时按分片序
// 号顺序写入最终文件。对象存储渠道未来实现原生 Multipart Upload 时，方法签名
// 保持一致，仅 uploadID 语义变为供应商的 multipart upload id。

func (c *client) CreateMultipartUpload(_ context.Context, bucket string, key string) (string, error) {
	if err := validateObject(bucket, key); err != nil {
		return "", err
	}
	uploadID := filepath.Join(c.basePath, ".multipart", uuid.NewString())
	if err := os.MkdirAll(uploadID, 0o750); err != nil {
		return "", fmt.Errorf("local storage: create multipart dir: %w", err)
	}
	return uploadID, nil
}

func (c *client) UploadPart(_ context.Context, _ string, _ string, uploadID string, partNumber int32, body io.Reader, _ objectstorage.PutOptions) (string, error) {
	if err := validateUploadID(c.basePath, uploadID); err != nil {
		return "", err
	}
	partPath := filepath.Join(uploadID, fmt.Sprintf("part-%d", partNumber))
	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return "", fmt.Errorf("local storage: create part file: %w", err)
	}
	hasher := sha256.New()
	if _, err := io.Copy(file, io.TeeReader(body, hasher)); err != nil {
		_ = file.Close()
		_ = os.Remove(partPath)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(partPath)
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (c *client) ListMultipartParts(_ context.Context, _ string, _ string, uploadID string) ([]objectstorage.MultipartPart, error) {
	if err := validateUploadID(c.basePath, uploadID); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(uploadID)
	if err != nil {
		if os.IsNotExist(err) {
			return []objectstorage.MultipartPart{}, nil
		}
		return nil, err
	}
	parts := make([]objectstorage.MultipartPart, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "part-") {
			continue
		}
		partNumber, err := strconv.ParseInt(strings.TrimPrefix(entry.Name(), "part-"), 10, 32)
		if err != nil {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(uploadID, entry.Name()))
		if readErr != nil {
			continue
		}
		hasher := sha256.New()
		_, _ = hasher.Write(content)
		parts = append(parts, objectstorage.MultipartPart{
			PartNumber: int32(partNumber),
			ETag:       hex.EncodeToString(hasher.Sum(nil)),
		})
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	return parts, nil
}

func (c *client) CompleteMultipartUpload(_ context.Context, bucket string, key string, uploadID string, parts []objectstorage.MultipartPart) (string, error) {
	if err := validateObject(bucket, key); err != nil {
		return "", err
	}
	if err := validateUploadID(c.basePath, uploadID); err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("local storage: no parts to complete")
	}
	target, err := c.objectPath(bucket, key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return "", err
	}
	tmp := target + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Close() }()

	// 按分片序号排序后顺序合并，避免乱序写入。
	sortedParts := make([]objectstorage.MultipartPart, len(parts))
	copy(sortedParts, parts)
	sort.Slice(sortedParts, func(i, j int) bool { return sortedParts[i].PartNumber < sortedParts[j].PartNumber })

	hasher := sha256.New()
	for _, part := range sortedParts {
		partPath := filepath.Join(uploadID, fmt.Sprintf("part-%d", part.PartNumber))
		f, openErr := os.Open(partPath)
		if openErr != nil {
			_ = os.Remove(tmp)
			return "", fmt.Errorf("local storage: open part %d: %w", part.PartNumber, openErr)
		}
		if _, copyErr := io.Copy(out, io.TeeReader(f, hasher)); copyErr != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return "", copyErr
		}
		_ = f.Close()
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.RemoveAll(uploadID)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (c *client) AbortMultipartUpload(_ context.Context, _ string, _ string, uploadID string) error {
	if err := validateUploadID(c.basePath, uploadID); err != nil {
		return err
	}
	return os.RemoveAll(uploadID)
}

// validateUploadID 校验 uploadID 是否位于 basePath 的分片临时目录内，防路径穿越。
func validateUploadID(basePath, uploadID string) error {
	clean := filepath.Clean(uploadID)
	multipartRoot := filepath.Join(basePath, ".multipart")
	if clean == multipartRoot || !strings.HasPrefix(clean, multipartRoot+string(filepath.Separator)) {
		return fmt.Errorf("%w: invalid multipart upload id", objectstorage.ErrInvalidObject)
	}
	return nil
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
