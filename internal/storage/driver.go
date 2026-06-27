// Package storage provides a pluggable file-storage abstraction.
// The MVP implementation is a local filesystem driver; the interface is
// designed so that an S3 (or other object-store) driver can be dropped in
// later without changing any consumer code.
package storage

import (
	"context"
	"io"
	"time"
)

// FileInfo contains metadata about a stored file.
type FileInfo struct {
	Bucket      string    `json:"bucket"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	URL         string    `json:"url"`
}

// Driver is the interface that all storage backends must implement.
// All methods accept a context.Context for cancellation and timeout propagation.
type Driver interface {
	// Upload stores data read from reader at the given bucket and path.
	// contentType is a MIME type (e.g. "image/png"). The returned FileInfo
	// includes the public URL to the stored file.
	Upload(ctx context.Context, bucket, path string, reader io.Reader, contentType string) (*FileInfo, error)

	// Download returns a reader for the file at bucket/path together with
	// its metadata. The caller must close the returned ReadCloser.
	Download(ctx context.Context, bucket, path string) (io.ReadCloser, *FileInfo, error)

	// Delete removes the file at bucket/path and its metadata.
	Delete(ctx context.Context, bucket, path string) error

	// List returns metadata for all files in the given bucket whose path
	// starts with prefix. An empty prefix lists everything in the bucket.
	List(ctx context.Context, bucket, prefix string) ([]FileInfo, error)

	// ListBuckets returns the names of all buckets.
	ListBuckets(ctx context.Context) ([]string, error)

	// CreateBucket creates a new bucket.
	CreateBucket(ctx context.Context, bucket string) error

	// DeleteBucket deletes a bucket and all its contents.
	DeleteBucket(ctx context.Context, bucket string) error
}
