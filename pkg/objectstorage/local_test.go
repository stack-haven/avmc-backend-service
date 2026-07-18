package objectstorage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalClientPutDeleteObject(t *testing.T) {
	client, err := NewLocalClient(Config{Provider: ProviderLocal, LocalBasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewLocalClient() error = %v", err)
	}
	info, err := client.PutObject(context.Background(), "tenant-files", "tenants/1/readme.txt", strings.NewReader("hello"), PutOptions{ContentType: "text/plain"})
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
	client, err := NewLocalClient(Config{Provider: ProviderLocal, LocalBasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewLocalClient() error = %v", err)
	}
	_, err = client.PutObject(context.Background(), "tenant-files", "../escape.txt", strings.NewReader("bad"), PutOptions{})
	if !errors.Is(err, ErrInvalidObject) {
		t.Fatalf("PutObject() error = %v, want ErrInvalidObject", err)
	}
	_, err = client.objectPath("tenant-files", filepath.Join("..", "escape.txt"))
	if !errors.Is(err, ErrInvalidObject) {
		t.Fatalf("objectPath() error = %v, want ErrInvalidObject", err)
	}
}

func TestLocalClientPublicURL(t *testing.T) {
	client, err := NewLocalClient(Config{
		Provider:      ProviderLocal,
		LocalBasePath: t.TempDir(),
		PublicBaseURL: "https://files.example.test/base",
	})
	if err != nil {
		t.Fatalf("NewLocalClient() error = %v", err)
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
	client, err := NewLocalClient(Config{Provider: ProviderLocal, LocalBasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewLocalClient() error = %v", err)
	}
	if _, err = client.PresignPutObject(context.Background(), "tenant-files", "a.txt", PresignOptions{}); !errors.Is(err, ErrUnsupportedProvider) {
		t.Fatalf("PresignPutObject() error = %v, want ErrUnsupportedProvider", err)
	}
}
