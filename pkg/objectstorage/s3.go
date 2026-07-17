package objectstorage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	awsAlgorithm  = "AWS4-HMAC-SHA256"
	awsEmptyHash  = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	maxPresignTTL = 7 * 24 * time.Hour
)

type S3CompatibleClient struct {
	endpoint       *url.URL
	publicBaseURL  *url.URL
	region         string
	accessKey      string
	secretKey      string
	sessionToken   string
	forcePathStyle bool
	httpClient     *http.Client
	now            func() time.Time
}

func NewS3CompatibleClient(config Config) (*S3CompatibleClient, error) {
	endpoint, err := normalizeEndpoint(config.Endpoint, config.UseSSL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(config.AccessKey) == "" || strings.TrimSpace(config.SecretKey) == "" {
		return nil, ErrInvalidConfig
	}
	region := strings.TrimSpace(config.Region)
	if region == "" {
		region = "us-east-1"
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	var publicBase *url.URL
	if strings.TrimSpace(config.PublicBaseURL) != "" {
		publicBase, err = url.Parse(strings.TrimRight(config.PublicBaseURL, "/"))
		if err != nil || publicBase.Scheme == "" || publicBase.Host == "" {
			return nil, ErrInvalidConfig
		}
	}
	return &S3CompatibleClient{
		endpoint:       endpoint,
		publicBaseURL:  publicBase,
		region:         region,
		accessKey:      strings.TrimSpace(config.AccessKey),
		secretKey:      strings.TrimSpace(config.SecretKey),
		sessionToken:   strings.TrimSpace(config.SessionToken),
		forcePathStyle: config.ForcePathStyle,
		httpClient:     httpClient,
		now:            time.Now,
	}, nil
}

func (c *S3CompatibleClient) PutObject(ctx context.Context, bucket string, key string, body io.Reader, opts PutOptions) (*ObjectInfo, error) {
	if err := validateObject(bucket, key); err != nil {
		return nil, err
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	objectURL := c.objectURL(bucket, key, false)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	if opts.ContentType != "" {
		req.Header.Set("Content-Type", opts.ContentType)
	}
	for name, value := range opts.Metadata {
		req.Header.Set("x-amz-meta-"+strings.ToLower(name), value)
	}
	req.Header.Set("x-amz-content-sha256", hashHex(payload))
	req.ContentLength = int64(len(payload))
	c.signHeader(req, payload)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("objectstorage: put object failed: %s", resp.Status)
	}
	return &ObjectInfo{
		Bucket: bucket,
		Key:    key,
		ETag:   strings.Trim(resp.Header.Get("ETag"), `"`),
		Size:   int64(len(payload)),
	}, nil
}

func (c *S3CompatibleClient) DeleteObject(ctx context.Context, bucket string, key string) error {
	if err := validateObject(bucket, key); err != nil {
		return err
	}
	objectURL := c.objectURL(bucket, key, false)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, objectURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-amz-content-sha256", awsEmptyHash)
	c.signHeader(req, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("objectstorage: delete object failed: %s", resp.Status)
	}
	return nil
}

func (c *S3CompatibleClient) PresignGetObject(_ context.Context, bucket string, key string, opts PresignOptions) (string, error) {
	return c.presign(http.MethodGet, bucket, key, opts)
}

func (c *S3CompatibleClient) PresignPutObject(_ context.Context, bucket string, key string, opts PresignOptions) (string, error) {
	return c.presign(http.MethodPut, bucket, key, opts)
}

func (c *S3CompatibleClient) PublicURL(bucket string, key string) (string, error) {
	if err := validateObject(bucket, key); err != nil {
		return "", err
	}
	return c.objectURLWithBase(bucket, key, c.publicBaseURL, true).String(), nil
}

func (c *S3CompatibleClient) presign(method string, bucket string, key string, opts PresignOptions) (string, error) {
	if err := validateObject(bucket, key); err != nil {
		return "", err
	}
	expires := opts.Expires
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	if expires > maxPresignTTL {
		expires = maxPresignTTL
	}
	t := c.now().UTC()
	objectURL := c.objectURL(bucket, key, false)
	query := objectURL.Query()
	credentialScope := c.credentialScope(t)
	query.Set("X-Amz-Algorithm", awsAlgorithm)
	query.Set("X-Amz-Credential", c.accessKey+"/"+credentialScope)
	query.Set("X-Amz-Date", amzDate(t))
	query.Set("X-Amz-Expires", strconv.FormatInt(int64(expires/time.Second), 10))
	query.Set("X-Amz-SignedHeaders", "host")
	if c.sessionToken != "" {
		query.Set("X-Amz-Security-Token", c.sessionToken)
	}
	objectURL.RawQuery = query.Encode()
	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI(objectURL),
		canonicalQuery(objectURL.Query()),
		"host:" + objectURL.Host + "\n",
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")
	signature := c.signature(t, canonicalRequest)
	query.Set("X-Amz-Signature", signature)
	objectURL.RawQuery = query.Encode()
	return objectURL.String(), nil
}

func (c *S3CompatibleClient) signHeader(req *http.Request, payload []byte) {
	t := c.now().UTC()
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", amzDate(t))
	if c.sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", c.sessionToken)
	}
	payloadHash := req.Header.Get("x-amz-content-sha256")
	if payloadHash == "" {
		payloadHash = hashHex(payload)
		req.Header.Set("x-amz-content-sha256", payloadHash)
	}
	signedHeaders, canonicalHeaders := canonicalSignedHeaders(req.Header, req.URL.Host)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL),
		canonicalQuery(req.URL.Query()),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	credentialScope := c.credentialScope(t)
	auth := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		awsAlgorithm,
		c.accessKey,
		credentialScope,
		signedHeaders,
		c.signature(t, canonicalRequest),
	)
	req.Header.Set("Authorization", auth)
}

func (c *S3CompatibleClient) signature(t time.Time, canonicalRequest string) string {
	stringToSign := strings.Join([]string{
		awsAlgorithm,
		amzDate(t),
		c.credentialScope(t),
		hashHex([]byte(canonicalRequest)),
	}, "\n")
	signingKey := hmacSHA256([]byte("AWS4"+c.secretKey), shortDate(t))
	signingKey = hmacSHA256(signingKey, c.region)
	signingKey = hmacSHA256(signingKey, "s3")
	signingKey = hmacSHA256(signingKey, "aws4_request")
	return hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
}

func (c *S3CompatibleClient) credentialScope(t time.Time) string {
	return shortDate(t) + "/" + c.region + "/s3/aws4_request"
}

func (c *S3CompatibleClient) objectURL(bucket string, key string, usePublicBase bool) *url.URL {
	var base *url.URL
	if usePublicBase && c.publicBaseURL != nil {
		base = c.publicBaseURL
	} else {
		base = c.endpoint
	}
	return c.objectURLWithBase(bucket, key, base, usePublicBase)
}

func (c *S3CompatibleClient) objectURLWithBase(bucket string, key string, base *url.URL, public bool) *url.URL {
	u := *base
	if !c.forcePathStyle && !public {
		u.Host = bucket + "." + u.Host
		u.Path = "/" + key
		u.RawPath = "/" + escapePath(key)
		return &u
	}
	basePath := strings.TrimRight(u.Path, "/")
	u.Path = path.Join(basePath, bucket, key)
	u.RawPath = path.Join(basePath, url.PathEscape(bucket), escapePath(key))
	if !strings.HasPrefix(u.RawPath, "/") {
		u.RawPath = "/" + u.RawPath
	}
	return &u
}

func normalizeEndpoint(endpoint string, useSSL bool) (*url.URL, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, ErrInvalidConfig
	}
	if !strings.Contains(endpoint, "://") {
		scheme := "http"
		if useSSL {
			scheme = "https"
		}
		endpoint = scheme + "://" + endpoint
	}
	u, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, ErrInvalidConfig
	}
	return u, nil
}

func validateObject(bucket string, key string) error {
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(key) == "" || strings.HasPrefix(key, "/") {
		return ErrInvalidObject
	}
	return nil
}

func canonicalURI(u *url.URL) string {
	if u.EscapedPath() == "" {
		return "/"
	}
	return u.EscapedPath()
}

func canonicalQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(values))
	for _, key := range keys {
		vals := append([]string(nil), values[key]...)
		sort.Strings(vals)
		escapedKey := url.QueryEscape(key)
		for _, value := range vals {
			parts = append(parts, escapedKey+"="+url.QueryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func canonicalSignedHeaders(header http.Header, host string) (string, string) {
	headers := map[string]string{"host": host}
	for name, values := range header {
		lower := strings.ToLower(name)
		if lower == "authorization" || lower == "user-agent" {
			continue
		}
		headers[lower] = strings.Join(values, ",")
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+":"+strings.Join(strings.Fields(headers[key]), " "))
	}
	return strings.Join(keys, ";"), strings.Join(lines, "\n") + "\n"
}

func escapePath(value string) string {
	segments := strings.Split(value, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func hashHex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func amzDate(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func shortDate(t time.Time) string {
	return t.UTC().Format("20060102")
}
