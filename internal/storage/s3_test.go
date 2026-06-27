package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// s3TestConfig returns the S3 driver configuration from environment variables.
// Returns nil if required credentials are not set, in which case tests should be skipped.
func s3TestConfig(t *testing.T) *S3Driver {
	t.Helper()

	if os.Getenv("S3_ACCESS_KEY") == "" || os.Getenv("S3_SECRET_KEY") == "" {
		t.Skip("S3_ACCESS_KEY and S3_SECRET_KEY not set; skipping S3 integration tests")
	}

	d, err := NewS3DriverFromEnv()
	if err != nil {
		t.Fatalf("NewS3DriverFromEnv: %v", err)
	}
	return d
}

func TestS3Driver_UploadDownload(t *testing.T) {
	d := s3TestConfig(t)

	content := []byte("hello, s3 storage")
	bucket := "test-upload"
	path := "data/hello.txt"

	info, err := d.Upload(context.Background(), bucket, path, bytes.NewReader(content), "text/plain")
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
	if info.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if info.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}
	if info.URL == "" {
		t.Error("URL is empty")
	}

	// Download and verify.
	rc, dlInfo, err := d.Download(context.Background(), bucket, path)
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

	// Cleanup.
	if err := d.Delete(context.Background(), bucket, path); err != nil {
		t.Errorf("cleanup Delete: %v", err)
	}
}

func TestS3Driver_Delete(t *testing.T) {
	d := s3TestConfig(t)

	bucket, path := "test-delete", "records/entry.json"
	_, err := d.Upload(context.Background(), bucket, path, bytes.NewReader([]byte("{}")), "application/json")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if err := d.Delete(context.Background(), bucket, path); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Downloading the deleted file should error.
	rc, _, err := d.Download(context.Background(), bucket, path)
	if err == nil {
		rc.Close()
		t.Fatal("Download of deleted file should error")
	}
}

func TestS3Driver_List(t *testing.T) {
	d := s3TestConfig(t)

	bucket := "test-list"

	// Clean up any leftover objects from previous runs.
	existing, err := d.List(context.Background(), bucket, "")
	if err == nil {
		for _, fi := range existing {
			_ = d.Delete(context.Background(), bucket, fi.Path)
		}
	}

	_, err = d.Upload(context.Background(), bucket, "2024/a.txt", bytes.NewReader([]byte("a")), "text/plain")
	if err != nil {
		t.Fatalf("Upload a: %v", err)
	}
	_, err = d.Upload(context.Background(), bucket, "2024/b.txt", bytes.NewReader([]byte("bb")), "text/plain")
	if err != nil {
		t.Fatalf("Upload b: %v", err)
	}
	_, err = d.Upload(context.Background(), bucket, "2025/c.txt", bytes.NewReader([]byte("ccc")), "text/plain")
	if err != nil {
		t.Fatalf("Upload c: %v", err)
	}
	defer func() {
		_ = d.Delete(context.Background(), bucket, "2024/a.txt")
		_ = d.Delete(context.Background(), bucket, "2024/b.txt")
		_ = d.Delete(context.Background(), bucket, "2025/c.txt")
	}()

	// List all.
	all, err := d.List(context.Background(), bucket, "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("List all: got %d items, want 3", len(all))
	}

	// List with prefix.
	filtered, err := d.List(context.Background(), bucket, "2024/")
	if err != nil {
		t.Fatalf("List 2024/: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("List 2024/: got %d items, want 2", len(filtered))
	}

	// List non-existent bucket should return empty, not error.
	empty, err := d.List(context.Background(), "nosuchbucket", "")
	if err != nil {
		t.Errorf("List non-existent bucket: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty list for non-existent bucket, got %d items", len(empty))
	}
}

func TestS3Driver_EmptyBucketOrPath(t *testing.T) {
	d := s3TestConfig(t)

	r := bytes.NewReader([]byte("x"))

	if _, err := d.Upload(context.Background(), "", "path", r, "text/plain"); err == nil {
		t.Error("Upload with empty bucket should error")
	}
	if _, err := d.Upload(context.Background(), "bucket", "", r, "text/plain"); err == nil {
		t.Error("Upload with empty path should error")
	}
	if _, _, err := d.Download(context.Background(), "", "path"); err == nil {
		t.Error("Download with empty bucket should error")
	}
	if _, _, err := d.Download(context.Background(), "bucket", ""); err == nil {
		t.Error("Download with empty path should error")
	}
	if err := d.Delete(context.Background(), "", "path"); err == nil {
		t.Error("Delete with empty bucket should error")
	}
	if err := d.Delete(context.Background(), "bucket", ""); err == nil {
		t.Error("Delete with empty path should error")
	}
}

func TestS3Driver_PublicURL(t *testing.T) {
	// Test with SSL enabled.
	d := &S3Driver{
		endpoint: "s3.fr-par.scw.cloud",
		useSSL:   true,
	}
	url := d.publicURL("mybucket", "path/to/file.txt")
	if !strings.Contains(url, "https://") {
		t.Errorf("expected https URL, got %q", url)
	}
	if !strings.Contains(url, "mybucket.s3.fr-par.scw.cloud") {
		t.Errorf("expected virtual hosted-style URL, got %q", url)
	}

	// Test with SSL disabled.
	d2 := &S3Driver{
		endpoint: "localhost:9000",
		useSSL:   false,
	}
	url2 := d2.publicURL("mybucket", "file.txt")
	if !strings.HasPrefix(url2, "http://") {
		t.Errorf("expected http URL, got %q", url2)
	}
}

func TestS3Driver_BucketName(t *testing.T) {
	d := &S3Driver{bucketPrefix: "sovrabase"}
	if got := d.bucketName("images"); got != "sovrabase-images" {
		t.Errorf("bucketName = %q, want %q", got, "sovrabase-images")
	}
}

func TestS3Driver_NewS3Driver_Validation(t *testing.T) {
	// Empty endpoint.
	_, err := NewS3Driver("", "key", "secret", "pref", true)
	if err == nil {
		t.Error("expected error for empty endpoint")
	}

	// Empty credentials.
	_, err = NewS3Driver("endpoint", "", "secret", "pref", true)
	if err == nil {
		t.Error("expected error for empty access key")
	}
	_, err = NewS3Driver("endpoint", "key", "", "pref", true)
	if err == nil {
		t.Error("expected error for empty secret key")
	}
}

func TestS3Driver_DefaultBucketPrefix(t *testing.T) {
	// When bucketPrefix is empty, it should default to "sovrabase".
	d, err := NewS3Driver("s3.example.com", "key", "secret", "", true)
	if err != nil {
		t.Fatalf("NewS3Driver: %v", err)
	}
	if d.bucketPrefix != defaultS3BucketPrefix {
		t.Errorf("bucketPrefix = %q, want %q", d.bucketPrefix, defaultS3BucketPrefix)
	}
}
