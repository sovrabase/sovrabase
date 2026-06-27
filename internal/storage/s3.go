package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Default S3 configuration constants.
const (
	defaultS3Endpoint     = "s3.fr-par.scw.cloud" // Scaleway Paris
	defaultS3BucketPrefix = "sovrabase"
	defaultS3UseSSL       = true
)

// S3Driver implements Driver using any S3-compatible object store
// (Scaleway, OVHcloud, Hetzner Object Storage, AWS S3, MinIO, etc.).
type S3Driver struct {
	client       *minio.Client
	bucketPrefix string
	endpoint     string
	useSSL       bool
	bucketsKnown sync.Map // cache of known-existing buckets — avoids HEAD per upload
}

// NewS3Driver creates an S3Driver connected to the given endpoint.
// endpoint is the S3-compatible host (e.g. "s3.fr-par.scw.cloud").
// bucketPrefix is prepended to every bucket name as "{prefix}-{bucket}".
func NewS3Driver(endpoint, accessKey, secretKey, bucketPrefix string, useSSL bool) (*S3Driver, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("storage: S3 endpoint is required")
	}
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("storage: S3 access key and secret key are required")
	}

	endpoint = strings.TrimSpace(endpoint)

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: create minio client: %w", err)
	}

	if bucketPrefix == "" {
		bucketPrefix = defaultS3BucketPrefix
	}

	return &S3Driver{
		client:       client,
		bucketPrefix: bucketPrefix,
		endpoint:     endpoint,
		useSSL:       useSSL,
	}, nil
}

// NewS3DriverFromEnv creates an S3Driver using environment variables:
//
//	S3_ENDPOINT      – S3-compatible endpoint (default: s3.fr-par.scw.cloud)
//	S3_ACCESS_KEY    – access key ID (required)
//	S3_SECRET_KEY    – secret access key (required)
//	S3_BUCKET_PREFIX – prefix for bucket names (default: sovrabase)
//	S3_USE_SSL       – use TLS (default: true)
func NewS3DriverFromEnv() (*S3Driver, error) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		endpoint = defaultS3Endpoint
	}

	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")

	bucketPrefix := os.Getenv("S3_BUCKET_PREFIX")
	if bucketPrefix == "" {
		bucketPrefix = defaultS3BucketPrefix
	}

	useSSL := defaultS3UseSSL
	if v := os.Getenv("S3_USE_SSL"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("storage: invalid S3_USE_SSL value %q: %w", v, err)
		}
		useSSL = parsed
	}

	return NewS3Driver(endpoint, accessKey, secretKey, bucketPrefix, useSSL)
}

// bucketName returns the full S3 bucket name: "{prefix}-{bucket}".
func (d *S3Driver) bucketName(bucket string) string {
	return d.bucketPrefix + "-" + bucket
}

// publicURL returns the public URL for an object.
func (d *S3Driver) publicURL(bucket, path string) string {
	scheme := "http"
	if d.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s.%s/%s", scheme, bucket, d.endpoint, path)
}

// Upload implements Driver.
func (d *S3Driver) Upload(ctx context.Context, bucket, path string, reader io.Reader, contentType string) (*FileInfo, error) {
	if bucket == "" || path == "" {
		return nil, fmt.Errorf("storage: bucket and path are required")
	}

	fullBucket := d.bucketName(bucket)

	// Ensure the bucket exists — cached to avoid HEAD on every upload.
	if _, known := d.bucketsKnown.Load(fullBucket); !known {
		exists, err := d.client.BucketExists(ctx, fullBucket)
		if err != nil {
			return nil, fmt.Errorf("storage: check bucket %s: %w", fullBucket, err)
		}
		if !exists {
			if err := d.client.MakeBucket(ctx, fullBucket, minio.MakeBucketOptions{}); err != nil {
				return nil, fmt.Errorf("storage: create bucket %s: %w", fullBucket, err)
			}
		}
		d.bucketsKnown.Store(fullBucket, true)
	}

	info, err := d.client.PutObject(ctx, fullBucket, path, reader, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: upload to %s/%s: %w", fullBucket, path, err)
	}

	now := time.Now().UTC()
	return &FileInfo{
		Bucket:      bucket,
		Path:        path,
		Size:        info.Size,
		ContentType: contentType,
		CreatedAt:   now,
		UpdatedAt:   now,
		URL:         d.publicURL(fullBucket, path),
	}, nil
}

// Download implements Driver.
func (d *S3Driver) Download(ctx context.Context, bucket, path string) (io.ReadCloser, *FileInfo, error) {
	if bucket == "" || path == "" {
		return nil, nil, fmt.Errorf("storage: bucket and path are required")
	}

	fullBucket := d.bucketName(bucket)

	stat, err := d.client.StatObject(ctx, fullBucket, path, minio.StatObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("storage: stat %s/%s: %w", fullBucket, path, err)
	}

	obj, err := d.client.GetObject(ctx, fullBucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("storage: download %s/%s: %w", fullBucket, path, err)
	}

	fi := &FileInfo{
		Bucket:      bucket,
		Path:        path,
		Size:        stat.Size,
		ContentType: stat.ContentType,
		CreatedAt:   stat.LastModified,
		UpdatedAt:   stat.LastModified,
		URL:         d.publicURL(fullBucket, path),
	}

	return obj, fi, nil
}

// Delete implements Driver.
func (d *S3Driver) Delete(ctx context.Context, bucket, path string) error {
	if bucket == "" || path == "" {
		return fmt.Errorf("storage: bucket and path are required")
	}

	fullBucket := d.bucketName(bucket)

	err := d.client.RemoveObject(ctx, fullBucket, path, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("storage: delete %s/%s: %w", fullBucket, path, err)
	}
	return nil
}

// List implements Driver.
func (d *S3Driver) List(ctx context.Context, bucket, prefix string) ([]FileInfo, error) {
	fullBucket := d.bucketName(bucket)

	var results []FileInfo
	for obj := range d.client.ListObjects(ctx, fullBucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("storage: list %s: %w", fullBucket, obj.Err)
		}
		results = append(results, FileInfo{
			Bucket:      bucket,
			Path:        obj.Key,
			Size:        obj.Size,
			ContentType: obj.ContentType,
			CreatedAt:   obj.LastModified,
			UpdatedAt:   obj.LastModified,
			URL:         d.publicURL(fullBucket, obj.Key),
		})
	}

	return results, nil
}

// ListBuckets implements Driver.
func (d *S3Driver) ListBuckets(ctx context.Context) ([]string, error) {
	bucketsInfo, err := d.client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: list buckets: %w", err)
	}

	var buckets []string
	for _, b := range bucketsInfo {
		name := b.Name
		if strings.HasPrefix(name, d.bucketPrefix+"-") {
			name = strings.TrimPrefix(name, d.bucketPrefix+"-")
			buckets = append(buckets, name)
		}
	}
	return buckets, nil
}

// CreateBucket implements Driver.
func (d *S3Driver) CreateBucket(ctx context.Context, bucket string) error {
	fullBucket := d.bucketName(bucket)
	exists, err := d.client.BucketExists(ctx, fullBucket)
	if err != nil {
		return fmt.Errorf("storage: check bucket: %w", err)
	}
	if exists {
		return fmt.Errorf("storage: bucket %s already exists", bucket)
	}
	if err := d.client.MakeBucket(ctx, fullBucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("storage: make bucket: %w", err)
	}
	return nil
}

// DeleteBucket implements Driver.
func (d *S3Driver) DeleteBucket(ctx context.Context, bucket string) error {
	fullBucket := d.bucketName(bucket)
	if err := d.client.RemoveBucket(ctx, fullBucket); err != nil {
		return fmt.Errorf("storage: remove bucket: %w", err)
	}
	return nil
}
