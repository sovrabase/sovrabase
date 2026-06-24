package storage

import (
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
	mu       sync.RWMutex
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
	// Strip trailing / to keep URL generation predictable.
	baseURL = strings.TrimRight(baseURL, "/")
	return &LocalDriver{
		basePath: abs,
		baseURL:  baseURL,
	}, nil
}

// BasePath returns the absolute local filesystem root.
func (d *LocalDriver) BasePath() string { return d.basePath }

// BaseURL returns the public URL prefix.
func (d *LocalDriver) BaseURL() string { return d.baseURL }

// Upload implements Driver.
func (d *LocalDriver) Upload(bucket, path string, reader io.Reader, contentType string) (*FileInfo, error) {
	if bucket == "" || path == "" {
		return nil, fmt.Errorf("storage: bucket and path are required")
	}

	diskPath, dir := d.diskPath(bucket, path)

	d.mu.Lock()
	defer d.mu.Unlock()

	// Ensure the bucket directory exists.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create bucket dir %s: %w", dir, err)
	}

	// Write the file.
	f, err := os.Create(diskPath)
	if err != nil {
		return nil, fmt.Errorf("storage: open for write %s: %w", diskPath, err)
	}
	defer f.Close()

	size, err := io.Copy(f, reader)
	if err != nil {
		// Best-effort cleanup on partial write.
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
func (d *LocalDriver) Download(bucket, path string) (io.ReadCloser, *FileInfo, error) {
	if bucket == "" || path == "" {
		return nil, nil, fmt.Errorf("storage: bucket and path are required")
	}

	diskPath, _ := d.diskPath(bucket, path)

	d.mu.RLock()
	defer d.mu.RUnlock()

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
func (d *LocalDriver) Delete(bucket, path string) error {
	if bucket == "" || path == "" {
		return fmt.Errorf("storage: bucket and path are required")
	}

	diskPath, _ := d.diskPath(bucket, path)

	d.mu.Lock()
	defer d.mu.Unlock()

	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: remove file %s: %w", diskPath, err)
	}
	if err := removeMetadata(diskPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: remove metadata %s: %w", diskPath, err)
	}
	return nil
}

// List implements Driver.
func (d *LocalDriver) List(bucket, prefix string) ([]FileInfo, error) {
	bucketDir := filepath.Join(d.basePath, bucket)

	d.mu.RLock()
	defer d.mu.RUnlock()

	infos, err := listMetadata(bucketDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("storage: list metadata: %w", err)
	}

	if prefix == "" {
		return infos, nil
	}

	// Filter by prefix.
	var filtered []FileInfo
	for _, info := range infos {
		if strings.HasPrefix(info.Path, prefix) {
			filtered = append(filtered, info)
		}
	}
	return filtered, nil
}

// diskPath computes the absolute filesystem path for a bucket/path pair.
func (d *LocalDriver) diskPath(bucket, path string) (string, string) {
	dir := filepath.Join(d.basePath, bucket, filepath.Dir(path))
	full := filepath.Join(d.basePath, bucket, path)
	return full, dir
}

// publicURL builds a public URL from baseURL + bucket + path.
func (d *LocalDriver) publicURL(bucket, path string) string {
	if d.baseURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", d.baseURL, bucket, path)
}

// ListBuckets implements Driver.
func (d *LocalDriver) ListBuckets() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

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

// CreateBucket implements Driver.
func (d *LocalDriver) CreateBucket(bucket string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	bucketDir := filepath.Join(d.basePath, bucket)
	if err := os.MkdirAll(bucketDir, 0o755); err != nil {
		return fmt.Errorf("storage: create bucket: %w", err)
	}
	return nil
}

// DeleteBucket implements Driver.
func (d *LocalDriver) DeleteBucket(bucket string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	bucketDir := filepath.Join(d.basePath, bucket)
	if err := os.RemoveAll(bucketDir); err != nil {
		return fmt.Errorf("storage: delete bucket: %w", err)
	}
	return nil
}
