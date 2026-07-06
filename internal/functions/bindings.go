package functions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// StorageRPCMaxBytes is the max payload size for storage RPC transfers.
	StorageRPCMaxBytes = 10 << 20
)

// HostAPI exposes controlled host capabilities to function sandboxes.
type HostAPI interface {
	DBGet(collection, id string) (map[string]interface{}, error)
	DBCreate(collection string, data map[string]interface{}) (map[string]interface{}, error)
	DBUpdate(collection, id string, data map[string]interface{}) (map[string]interface{}, error)
	DBDelete(collection, id string) error
	Fetch(ctx context.Context, method, url string, headers map[string]string, body interface{}) (int, []byte, error)
	StorageUpload(ctx context.Context, bucket, path, contentType string, data []byte) (map[string]interface{}, error)
	StorageDownload(ctx context.Context, bucket, path string) ([]byte, map[string]interface{}, error)
	StorageDelete(ctx context.Context, bucket, path string) error
	StorageList(ctx context.Context, bucket, prefix string) ([]map[string]interface{}, error)
}

// FetchHTTP performs an outbound HTTP request with size limits.
func FetchHTTP(ctx context.Context, method, url string, headers map[string]string, body interface{}) (int, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return 0, nil, fmt.Errorf("fetch: invalid URL")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = strings.NewReader(string(b))
	}
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequestWithContext(reqCtx, method, url, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}

// noopHost is used when no bindings are configured.
type noopHost struct{}

func (noopHost) DBGet(string, string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("sovrabase.db: bindings not configured")
}
func (noopHost) DBCreate(string, map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("sovrabase.db: bindings not configured")
}
func (noopHost) DBUpdate(string, string, map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("sovrabase.db: bindings not configured")
}
func (noopHost) DBDelete(string, string) error {
	return fmt.Errorf("sovrabase.db: bindings not configured")
}
func (noopHost) Fetch(ctx context.Context, method, url string, headers map[string]string, body interface{}) (int, []byte, error) {
	return FetchHTTP(ctx, method, url, headers, body)
}
func (noopHost) StorageUpload(context.Context, string, string, string, []byte) (map[string]interface{}, error) {
	return nil, fmt.Errorf("sovrabase.storage: bindings not configured")
}
func (noopHost) StorageDownload(context.Context, string, string) ([]byte, map[string]interface{}, error) {
	return nil, nil, fmt.Errorf("sovrabase.storage: bindings not configured")
}
func (noopHost) StorageDelete(context.Context, string, string) error {
	return fmt.Errorf("sovrabase.storage: bindings not configured")
}
func (noopHost) StorageList(context.Context, string, string) ([]map[string]interface{}, error) {
	return nil, fmt.Errorf("sovrabase.storage: bindings not configured")
}

// DefaultHost provides fetch-only bindings.
var DefaultHost HostAPI = noopHost{}

// SetDefaultHost sets the host API used when none is passed explicitly.
func SetDefaultHost(h HostAPI) {
	if h != nil {
		DefaultHost = h
	}
}

// DecodeStorageData decodes base64 or raw string upload payloads.
func DecodeStorageData(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, nil
	}
	if b, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		if len(b) > StorageRPCMaxBytes {
			return nil, fmt.Errorf("storage: payload exceeds %d bytes", StorageRPCMaxBytes)
		}
		return b, nil
	}
	if len(encoded) > StorageRPCMaxBytes {
		return nil, fmt.Errorf("storage: payload exceeds %d bytes", StorageRPCMaxBytes)
	}
	return []byte(encoded), nil
}
