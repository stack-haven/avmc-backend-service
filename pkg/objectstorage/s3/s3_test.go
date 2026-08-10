package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"backend-service/pkg/objectstorage"
)

func TestS3CompatiblePresignPutUsesSigV4Query(t *testing.T) {
	client, err := New(Config{
		Endpoint:       "storage.example.com",
		Region:         "auto",
		AccessKey:      "ak",
		SecretKey:      "sk",
		UseSSL:         true,
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	client.now = func() time.Time {
		return time.Date(2026, 7, 17, 8, 9, 10, 0, time.UTC)
	}

	rawURL, err := client.PresignPutObject(context.Background(), "tenant-files", "avatars/user 1.png", objectstorage.PresignOptions{Expires: time.Hour})
	if err != nil {
		t.Fatalf("PresignPutObject() error = %v", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Parse presigned URL error = %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "storage.example.com" {
		t.Fatalf("presigned URL host = %s://%s", parsed.Scheme, parsed.Host)
	}
	if !strings.Contains(parsed.EscapedPath(), "/tenant-files/avatars/user%201.png") {
		t.Fatalf("presigned path = %s", parsed.EscapedPath())
	}
	query := parsed.Query()
	if query.Get("X-Amz-Algorithm") != awsAlgorithm {
		t.Fatalf("algorithm = %s", query.Get("X-Amz-Algorithm"))
	}
	if query.Get("X-Amz-Credential") != "ak/20260717/auto/s3/aws4_request" {
		t.Fatalf("credential = %s", query.Get("X-Amz-Credential"))
	}
	if query.Get("X-Amz-Date") != "20260717T080910Z" {
		t.Fatalf("date = %s", query.Get("X-Amz-Date"))
	}
	if query.Get("X-Amz-Expires") != "3600" {
		t.Fatalf("expires = %s", query.Get("X-Amz-Expires"))
	}
	if query.Get("X-Amz-Signature") == "" {
		t.Fatal("signature is empty")
	}
}

func TestS3CompatiblePutObjectSignsAndSendsPayload(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotAuth string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("ETag", `"etag-1"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{
		Endpoint:       server.URL,
		Region:         "us-east-1",
		AccessKey:      "ak",
		SecretKey:      "sk",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	client.now = func() time.Time {
		return time.Date(2026, 7, 17, 8, 9, 10, 0, time.UTC)
	}

	info, err := client.PutObject(context.Background(), "bucket", "docs/readme.txt", strings.NewReader("hello"), objectstorage.PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %s", gotMethod)
	}
	if gotPath != "/bucket/docs/readme.txt" {
		t.Fatalf("path = %s", gotPath)
	}
	if !strings.Contains(gotAuth, "AWS4-HMAC-SHA256 Credential=ak/20260717/us-east-1/s3/aws4_request") {
		t.Fatalf("authorization = %s", gotAuth)
	}
	if gotBody != "hello" {
		t.Fatalf("body = %q", gotBody)
	}
	if info.ETag != "etag-1" || info.Size != 5 {
		t.Fatalf("object info = %+v", info)
	}
}

func TestObjectStoragePublicURLUsesPublicBase(t *testing.T) {
	client, err := New(Config{
		Endpoint:       "minio:9000",
		PublicBaseURL:  "https://cdn.example.com/assets",
		AccessKey:      "ak",
		SecretKey:      "sk",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rawURL, err := client.PublicURL("bucket", "a/b.txt")
	if err != nil {
		t.Fatalf("PublicURL() error = %v", err)
	}
	if rawURL != "https://cdn.example.com/assets/bucket/a/b.txt" {
		t.Fatalf("PublicURL() = %s", rawURL)
	}
}
