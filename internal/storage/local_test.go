package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalDriver_UploadDownload(t *testing.T) {
	dir := t.TempDir()
	d, err := NewLocalDriver(dir, "http://localhost:6070/cdn")
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}

	content := []byte("hello, storage")
	bucket := "user-content"
	path := "images/hello.txt"

	info, err := d.Upload(bucket, path, bytes.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if info.Bucket != bucket {
		t.Errorf("Bucket = %q, want %q", info.Bucket, bucket)
	}
	if info.Path != path {
		t.Errorf("Path = %q, want %q", info.Path, path)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", info.Size, len(content))
	}
	if info.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want %q", info.ContentType, "text/plain")
	}
	if info.URL != "http://localhost:6070/cdn/user-content/images/hello.txt" {
		t.Errorf("URL = %q, want %q", info.URL, "http://localhost:6070/cdn/user-content/images/hello.txt")
	}
	if info.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if info.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}

	// Verify file exists on disk.
	diskPath := filepath.Join(dir, bucket, path)
	if _, err := os.Stat(diskPath); err != nil {
		t.Errorf("file not on disk: %v", err)
	}
	// Metadata file should exist.
	if _, err := os.Stat(metaPathFor(diskPath)); err != nil {
		t.Errorf("metadata file not on disk: %v", err)
	}

	// Download and verify.
	rc, dlInfo, err := d.Download(bucket, path)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("downloaded content = %q, want %q", got, content)
	}
	if dlInfo.Size != info.Size {
		t.Errorf("download Size = %d, want %d", dlInfo.Size, info.Size)
	}
}

func TestLocalDriver_Delete(t *testing.T) {
	dir := t.TempDir()
	d, err := NewLocalDriver(dir, "")
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}

	bucket, path := "data", "records/1.json"
	_, err = d.Upload(bucket, path, bytes.NewReader([]byte("{}")), "application/json")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if err := d.Delete(bucket, path); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	diskPath := filepath.Join(dir, bucket, path)
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Errorf("file still exists after delete")
	}
	if _, err := os.Stat(metaPathFor(diskPath)); !os.IsNotExist(err) {
		t.Errorf("metadata still exists after delete")
	}

	// Deleting again should not error.
	if err := d.Delete(bucket, path); err != nil {
		t.Errorf("Delete on missing file: %v", err)
	}
}

func TestLocalDriver_List(t *testing.T) {
	dir := t.TempDir()
	d, err := NewLocalDriver(dir, "http://cdn.example.com")
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}

	bucket := "photos"

	_, err = d.Upload(bucket, "2024/a.jpg", bytes.NewReader([]byte("a")), "image/jpeg")
	if err != nil {
		t.Fatalf("Upload a: %v", err)
	}
	_, err = d.Upload(bucket, "2024/b.jpg", bytes.NewReader([]byte("bb")), "image/jpeg")
	if err != nil {
		t.Fatalf("Upload b: %v", err)
	}
	_, err = d.Upload(bucket, "2025/c.png", bytes.NewReader([]byte("ccc")), "image/png")
	if err != nil {
		t.Fatalf("Upload c: %v", err)
	}

	// List all.
	all, err := d.List(bucket, "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("List all: got %d items, want 3", len(all))
	}

	// List with prefix.
	filtered, err := d.List(bucket, "2024/")
	if err != nil {
		t.Fatalf("List 2024/: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("List 2024/: got %d items, want 2", len(filtered))
	}

	// List empty prefix returns nothing for non-existent bucket.
	missing, err := d.List("nosuchbucket", "")
	if err != nil {
		t.Errorf("List non-existent bucket: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for non-existent bucket, got %v", missing)
	}
}

func TestLocalDriver_NoBaseURL(t *testing.T) {
	dir := t.TempDir()
	d, err := NewLocalDriver(dir, "")
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}

	info, err := d.Upload("b", "f.txt", bytes.NewReader([]byte("x")), "text/plain")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if info.URL != "" {
		t.Errorf("URL with empty baseURL = %q, want empty", info.URL)
	}
}

func TestLocalDriver_DownloadMissing(t *testing.T) {
	dir := t.TempDir()
	d, err := NewLocalDriver(dir, "")
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}

	_, _, err = d.Download("b", "nope.txt")
	if err == nil {
		t.Fatal("Download of missing file should error")
	}
}

func TestLocalDriver_SlashesInBaseURL(t *testing.T) {
	dir := t.TempDir()
	d, err := NewLocalDriver(dir, "https://cdn.example.com/")
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}

	info, err := d.Upload("b", "k.txt", bytes.NewReader([]byte("x")), "text/plain")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !strings.HasPrefix(info.URL, "https://cdn.example.com/b/k.txt") {
		t.Errorf("URL = %q, want prefix https://cdn.example.com/b/k.txt", info.URL)
	}
}

func TestLocalDriver_LargeFile(t *testing.T) {
	dir := t.TempDir()
	d, err := NewLocalDriver(dir, "")
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}

	size := 1 << 20 // 1 MiB
	content := bytes.Repeat([]byte("x"), size)

	info, err := d.Upload("large", "big.bin", bytes.NewReader(content), "application/octet-stream")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if info.Size != int64(size) {
		t.Errorf("Size = %d, want %d", info.Size, size)
	}

	rc, _, err := d.Download("large", "big.bin")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != size {
		t.Errorf("downloaded %d bytes, want %d", len(got), size)
	}
}

func TestLocalDriver_EmptyBucketOrPath(t *testing.T) {
	dir := t.TempDir()
	d, err := NewLocalDriver(dir, "")
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}

	r := bytes.NewReader([]byte("x"))

	if _, err := d.Upload("", "path", r, "text/plain"); err == nil {
		t.Error("Upload with empty bucket should error")
	}
	if _, err := d.Upload("bucket", "", r, "text/plain"); err == nil {
		t.Error("Upload with empty path should error")
	}
	if _, _, err := d.Download("", "path"); err == nil {
		t.Error("Download with empty bucket should error")
	}
	if _, _, err := d.Download("bucket", ""); err == nil {
		t.Error("Download with empty path should error")
	}
	if err := d.Delete("", "path"); err == nil {
		t.Error("Delete with empty bucket should error")
	}
	if err := d.Delete("bucket", ""); err == nil {
		t.Error("Delete with empty path should error")
	}
}
