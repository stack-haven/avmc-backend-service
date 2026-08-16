package data

import (
	"encoding/json"
	"strings"

	"backend-service/app/platform/service/internal/conf"
	"backend-service/pkg/objectstorage"

	// 触发 S3 供应商注册
	_ "backend-service/pkg/objectstorage/s3"
)

func NewObjectStorageClient(cfg *conf.OSS) objectstorage.Client {
	if cfg == nil || cfg.Minio == nil {
		return nil
	}
	minio := cfg.Minio
	if strings.TrimSpace(minio.Endpoint) == "" ||
		strings.TrimSpace(minio.AccessKey) == "" ||
		strings.TrimSpace(minio.SecretKey) == "" {
		return nil
	}
	configJSON, err := json.Marshal(map[string]interface{}{
		"endpoint":         minio.Endpoint,
		"access_key":       minio.AccessKey,
		"secret_key":       minio.SecretKey,
		"session_token":    minio.Token,
		"use_ssl":          minio.UseSsl,
		"force_path_style": true,
		"public_base_url":  minio.DownloadHost,
	})
	if err != nil {
		return nil
	}
	client, err := objectstorage.NewClient("s3-compatible", configJSON)
	if err != nil {
		return nil
	}
	return client
}
