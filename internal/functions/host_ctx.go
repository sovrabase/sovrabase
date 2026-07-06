package functions

import "context"

// contextualHost propagates execution context to host calls (fetch, storage).
type contextualHost struct {
	ctx context.Context
	HostAPI
}

func withContext(ctx context.Context, host HostAPI) HostAPI {
	if host == nil {
		host = DefaultHost
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return contextualHost{ctx: ctx, HostAPI: host}
}

func (h contextualHost) Fetch(ctx context.Context, method, url string, headers map[string]string, body interface{}) (int, []byte, error) {
	return h.HostAPI.Fetch(h.ctx, method, url, headers, body)
}

func (h contextualHost) StorageUpload(ctx context.Context, bucket, path, contentType string, data []byte) (map[string]interface{}, error) {
	return h.HostAPI.StorageUpload(h.ctx, bucket, path, contentType, data)
}

func (h contextualHost) StorageDownload(ctx context.Context, bucket, path string) ([]byte, map[string]interface{}, error) {
	return h.HostAPI.StorageDownload(h.ctx, bucket, path)
}

func (h contextualHost) StorageDelete(ctx context.Context, bucket, path string) error {
	return h.HostAPI.StorageDelete(h.ctx, bucket, path)
}

func (h contextualHost) StorageList(ctx context.Context, bucket, prefix string) ([]map[string]interface{}, error) {
	return h.HostAPI.StorageList(h.ctx, bucket, prefix)
}
