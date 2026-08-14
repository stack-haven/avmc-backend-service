package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend-service/pkg/objectstorage"
)

func TestLocalClientPutDeleteObject(t *testing.T) {
	client, err := New(Config{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	info, err := client.PutObject(context.Background(), "tenant-files", "tenants/1/readme.txt", strings.NewReader("hello"), objectstorage.PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	if info.Size != 5 || info.Bucket != "tenant-files" || info.Key != "tenants/1/readme.txt" || info.ETag == "" {
		t.Fatalf("PutObject() info = %+v", info)
	}
	target, err := client.objectPath("tenant-files", "tenants/1/readme.txt")
	if err != nil {
		t.Fatalf("objectPath() error = %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("stored content = %q", content)
	}
	if err = client.DeleteObject(context.Background(), "tenant-files", "tenants/1/readme.txt"); err != nil {
		t.Fatalf("DeleteObject() error = %v", err)
	}
	if _, err = os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("deleted file stat error = %v", err)
	}
}

func TestLocalClientRejectsPathTraversal(t *testing.T) {
	client, err := New(Config{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = client.PutObject(context.Background(), "tenant-files", "../escape.txt", strings.NewReader("bad"), objectstorage.PutOptions{})
	if !errors.Is(err, objectstorage.ErrInvalidObject) {
		t.Fatalf("PutObject() error = %v, want ErrInvalidObject", err)
	}
	_, err = client.objectPath("tenant-files", filepath.Join("..", "escape.txt"))
	if !errors.Is(err, objectstorage.ErrInvalidObject) {
		t.Fatalf("objectPath() error = %v, want ErrInvalidObject", err)
	}
}

func TestLocalClientPublicURL(t *testing.T) {
	client, err := New(Config{
		BasePath:      t.TempDir(),
		PublicBaseURL: "https://files.example.test/base",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	got, err := client.PublicURL("tenant-files", "tenants/1/report 1.pdf")
	if err != nil {
		t.Fatalf("PublicURL() error = %v", err)
	}
	want := "https://files.example.test/base/tenant-files/tenants/1/report%201.pdf"
	if got != want {
		t.Fatalf("PublicURL() = %q, want %q", got, want)
	}
}

func TestLocalClientPresignPutUnsupported(t *testing.T) {
	client, err := New(Config{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err = client.PresignPutObject(context.Background(), "tenant-files", "a.txt", objectstorage.PresignOptions{}); !errors.Is(err, objectstorage.ErrUnsupportedProvider) {
		t.Fatalf("PresignPutObject() error = %v, want ErrUnsupportedProvider", err)
	}
}

func TestLocalMultipartUpload(t *testing.T) {
	client, err := New(Config{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()
	uploadID, err := client.CreateMultipartUpload(ctx, "tenant-files", "big/file.bin")
	if err != nil {
		t.Fatalf("CreateMultipartUpload() error = %v", err)
	}
	parts := []objectstorage.MultipartPart{}
	for i, chunk := range []string{"hello ", "world", "!"} {
		etag, err := client.UploadPart(ctx, "tenant-files", "big/file.bin", uploadID, int32(i+1), strings.NewReader(chunk), objectstorage.PutOptions{})
		if err != nil {
			t.Fatalf("UploadPart(%d) error = %v", i+1, err)
		}
		if etag == "" {
			t.Fatalf("UploadPart(%d) etag empty", i+1)
		}
		parts = append(parts, objectstorage.MultipartPart{PartNumber: int32(i + 1), ETag: etag})
	}
	finalETag, err := client.CompleteMultipartUpload(ctx, "tenant-files", "big/file.bin", uploadID, parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload() error = %v", err)
	}
	if finalETag == "" {
		t.Fatal("final etag empty")
	}
	// 合并后的内容应顺序拼接
	content, err := client.GetObject(ctx, "tenant-files", "big/file.bin")
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	if string(content) != "hello world!" {
		t.Fatalf("merged content = %q, want %q", string(content), "hello world!")
	}
}

func TestLocalMultipartAbort(t *testing.T) {
	client, err := New(Config{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()
	uploadID, err := client.CreateMultipartUpload(ctx, "tenant-files", "abort.bin")
	if err != nil {
		t.Fatalf("CreateMultipartUpload() error = %v", err)
	}
	if err := client.AbortMultipartUpload(ctx, "tenant-files", "abort.bin", uploadID); err != nil {
		t.Fatalf("AbortMultipartUpload() error = %v", err)
	}
	// 校验 uploadID 目录已删除，后续上传应失败
	if _, err := client.UploadPart(ctx, "tenant-files", "abort.bin", uploadID, 1, strings.NewReader("x"), objectstorage.PutOptions{}); err == nil {
		t.Fatal("UploadPart() after abort = nil, want error")
	}
}
