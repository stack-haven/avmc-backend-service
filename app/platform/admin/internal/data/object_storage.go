package data

import (
	"strings"

	"backend-service/app/platform/admin/internal/conf"
	"backend-service/pkg/objectstorage"
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
	client, err := objectstorage.NewClient(objectstorage.Config{
		Provider:       objectstorage.ProviderS3Compatible,
		Endpoint:       minio.Endpoint,
		AccessKey:      minio.AccessKey,
		SecretKey:      minio.SecretKey,
		SessionToken:   minio.Token,
		UseSSL:         minio.UseSsl,
		ForcePathStyle: true,
		PublicBaseURL:  minio.DownloadHost,
		DefaultBucket:  "tenant-files",
	})
	if err != nil {
		return nil
	}
	return client
}
