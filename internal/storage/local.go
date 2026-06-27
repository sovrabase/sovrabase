package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LocalDriver implements Driver using the local filesystem.
// It is the MVP storage backend.
type LocalDriver struct {
	basePath string   // root directory for all buckets
	baseURL  string   // public URL prefix (e.g. "https://cdn.example.com")
	mu       sync.Mutex // only guards bucket dir creation
}

// NewLocalDriver creates a LocalDriver that stores files under
// basePath/{bucket}/{path} and returns public URLs of the form
// <baseURL>/{bucket}/{path}.
//
// If basePath does not exist it is created.
func NewLocalDriver(basePath, baseURL string) (*LocalDriver, error) {
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve base path: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create base directory: %w", err)
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &LocalDriver{
		basePath: abs,
		baseURL:  baseURL,
	}, nil
}

func (d *LocalDriver) BasePath() string { return d.basePath }
func (d *LocalDriver) BaseURL() string  { return d.baseURL }

// Upload implements Driver.
func (d *LocalDriver) Upload(ctx context.Context, bucket, path string, reader io.Reader, contentType string) (*FileInfo, error) {
	if bucket == "" || path == "" {
		return nil, fmt.Errorf("storage: bucket and path are required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	diskPath, dir := d.diskPath(bucket, path)

	d.mu.Lock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		d.mu.Unlock()
		return nil, fmt.Errorf("storage: create bucket dir %s: %w", dir, err)
	}
	d.mu.Unlock()

	f, err := os.Create(diskPath)
	if err != nil {
		return nil, fmt.Errorf("storage: open for write %s: %w", diskPath, err)
	}
	defer f.Close()

	size, err := io.Copy(f, reader)
	if err != nil {
		os.Remove(diskPath)
		os.Remove(metaPathFor(diskPath))
		return nil, fmt.Errorf("storage: write %s: %w", diskPath, err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("storage: close %s: %w", diskPath, err)
	}

	now := time.Now().UTC()
	info := &FileInfo{
		Bucket:      bucket,
		Path:        path,
		Size:        size,
		ContentType: contentType,
		CreatedAt:   now,
		UpdatedAt:   now,
		URL:         d.publicURL(bucket, path),
	}

	if err := saveMetadata(diskPath, info); err != nil {
		return nil, fmt.Errorf("storage: save metadata: %w", err)
	}
	return info, nil
}

// Download implements Driver.
func (d *LocalDriver) Download(ctx context.Context, bucket, path string) (io.ReadCloser, *FileInfo, error) {
	if bucket == "" || path == "" {
		return nil, nil, fmt.Errorf("storage: bucket and path are required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	diskPath, _ := d.diskPath(bucket, path)
	info, err := loadMetadata(diskPath)
	if err != nil {
		return nil, nil, fmt.Errorf("storage: load metadata for %s/%s: %w", bucket, path, err)
	}

	f, err := os.Open(diskPath)
	if err != nil {
		return nil, nil, fmt.Errorf("storage: open for read %s: %w", diskPath, err)
	}
	return f, info, nil
}

// Delete implements Driver.
func (d *LocalDriver) Delete(ctx context.Context, bucket, path string) error {
	if bucket == "" || path == "" {
		return fmt.Errorf("storage: bucket and path are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	diskPath, _ := d.diskPath(bucket, path)
	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: remove file %s: %w", diskPath, err)
	}
	if err := removeMetadata(diskPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: remove metadata %s: %w", diskPath, err)
	}
	return nil
}

// List implements Driver.
func (d *LocalDriver) List(ctx context.Context, bucket, prefix string) ([]FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	bucketDir := filepath.Join(d.basePath, bucket)
	infos, err := listMetadata(bucketDir, maxListMetadata)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("storage: list metadata: %w", err)
	}

	if prefix == "" {
		return infos, nil
	}

	var filtered []FileInfo
	for _, info := range infos {
		if strings.HasPrefix(info.Path, prefix) {
			filtered = append(filtered, info)
		}
	}
	return filtered, nil
}

func (d *LocalDriver) diskPath(bucket, path string) (string, string) {
	dir := filepath.Join(d.basePath, bucket, filepath.Dir(path))
	full := filepath.Join(d.basePath, bucket, path)
	return full, dir
}

func (d *LocalDriver) publicURL(bucket, path string) string {
	if d.baseURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", d.baseURL, bucket, path)
}

func (d *LocalDriver) ListBuckets(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(d.basePath)
	if err != nil {
		return nil, fmt.Errorf("storage: list buckets: %w", err)
	}
	var buckets []string
	for _, entry := range entries {
		if entry.IsDir() {
			buckets = append(buckets, entry.Name())
		}
	}
	return buckets, nil
}

func (d *LocalDriver) CreateBucket(ctx context.Context, bucket string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	bucketDir := filepath.Join(d.basePath, bucket)
	if err := os.MkdirAll(bucketDir, 0o755); err != nil {
		return fmt.Errorf("storage: create bucket: %w", err)
	}
	return nil
}

func (d *LocalDriver) DeleteBucket(ctx context.Context, bucket string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	bucketDir := filepath.Join(d.basePath, bucket)
	if err := os.RemoveAll(bucketDir); err != nil {
		return fmt.Errorf("storage: delete bucket: %w", err)
	}
	return nil
}
