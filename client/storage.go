package client

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
)

// ─── File Storage ─────────────────────────────────────────────────────────────

// UploadFile uploads a file to the specified bucket and path.
// reader is the file content, size is its total size in bytes, and contentType is the MIME type.
// If path is empty, the path must be derivable from the form data.
func (c *Client) UploadFile(bucket, path string, reader io.Reader, size int64, contentType string) (*FileInfo, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create form file field.
	part, err := writer.CreateFormFile("file", path)
	if err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}

	if _, err := io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}

	// Add the path field if provided.
	if path != "" {
		if err := writer.WriteField("path", path); err != nil {
			return nil, fmt.Errorf("upload: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}

	apiPath := fmt.Sprintf("/api/v1/storage/%s/upload", url.PathEscape(bucket))
	var info FileInfo
	if err := c.doMultipart(apiPath, &buf, writer.FormDataContentType(), &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// DownloadFile downloads a file from the specified bucket and path.
// Returns a ReadCloser (caller must close) and file metadata.
func (c *Client) DownloadFile(bucket, path string) (io.ReadCloser, *FileInfo, error) {
	apiPath := fmt.Sprintf("/api/v1/storage/%s/%s", url.PathEscape(bucket), url.PathEscape(path))

	bodyReader, _, headers, err := c.doRaw("GET", apiPath)
	if err != nil {
		return nil, nil, err
	}

	info := &FileInfo{
		Bucket: bucket,
		Path:   path,
	}
	if ct := headers.Get("Content-Type"); ct != "" {
		info.ContentType = ct
	}
	if cl := headers.Get("Content-Length"); cl != "" {
		fmt.Sscanf(cl, "%d", &info.Size)
	}

	return bodyReader, info, nil
}

// ListFiles lists files in a bucket with an optional prefix filter.
func (c *Client) ListFiles(bucket, prefix string) ([]FileInfo, error) {
	apiPath := fmt.Sprintf("/api/v1/storage/%s/list", url.PathEscape(bucket))
	if prefix != "" {
		apiPath += "?prefix=" + url.QueryEscape(prefix)
	}

	var files []FileInfo
	if err := c.doJSON("GET", apiPath, nil, &files); err != nil {
		return nil, err
	}
	if files == nil {
		files = []FileInfo{}
	}
	return files, nil
}

// DeleteFile deletes a file from the specified bucket and path.
func (c *Client) DeleteFile(bucket, path string) error {
	apiPath := fmt.Sprintf("/api/v1/storage/%s/%s", url.PathEscape(bucket), url.PathEscape(path))
	return c.doJSON("DELETE", apiPath, nil, nil)
}
