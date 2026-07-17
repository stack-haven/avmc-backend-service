package objectstorage

import (
	"context"
	"io"
	"net/http"
	"time"
)

type Provider string

const (
	ProviderS3Compatible Provider = "s3-compatible"
)

type Config struct {
	Provider       Provider
	Endpoint       string
	Region         string
	AccessKey      string
	SecretKey      string
	SessionToken   string
	UseSSL         bool
	ForcePathStyle bool
	PublicBaseURL  string
	DefaultBucket  string
	HTTPClient     *http.Client
}

type PutOptions struct {
	ContentType string
	Metadata    map[string]string
}

type PresignOptions struct {
	ContentType string
	Expires     time.Duration
	Metadata    map[string]string
}

type ObjectInfo struct {
	Bucket string
	Key    string
	ETag   string
	Size   int64
}

type Client interface {
	PutObject(context.Context, string, string, io.Reader, PutOptions) (*ObjectInfo, error)
	DeleteObject(context.Context, string, string) error
	PresignGetObject(context.Context, string, string, PresignOptions) (string, error)
	PresignPutObject(context.Context, string, string, PresignOptions) (string, error)
	PublicURL(string, string) (string, error)
}

func NewClient(config Config) (Client, error) {
	if config.Provider == "" || config.Provider == ProviderS3Compatible {
		return NewS3CompatibleClient(config)
	}
	return nil, ErrUnsupportedProvider
}
