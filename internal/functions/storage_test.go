package functions

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type slowFetchHost struct {
	noopHost
}

func (slowFetchHost) Fetch(ctx context.Context, method, url string, headers map[string]string, body interface{}) (int, []byte, error) {
	select {
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return 200, []byte("ok"), nil
	}
}

func TestRunLocalJobTimeoutInfiniteLoop(t *testing.T) {
	job := WorkerJob{
		Mode:      WorkerModeHTTP,
		Language:  "javascript",
		Code:      `function handler(req) { while (true) {} }`,
		TimeoutMs: 500,
		HTTPReq:   HTTPRequest{Method: "GET"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), FunctionTimeout(&Function{TimeoutMs: job.TimeoutMs}))
	defer cancel()

	start := time.Now()
	resp := RunLocalJob(ctx, job, DefaultHost)
	if resp.Error == "" {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(resp.Error, "timeout") {
		t.Fatalf("expected timeout message, got: %s", resp.Error)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("timeout took too long: %v", time.Since(start))
	}
}

func TestRunLocalJobTimeoutSlowFetch(t *testing.T) {
	job := WorkerJob{
		Mode:     WorkerModeHTTP,
		Language: "javascript",
		Code: `function handler(req) {
			fetch("http://127.0.0.1/slow");
			return { status: 200, body: {} };
		}`,
		TimeoutMs: 500,
		HTTPReq:   HTTPRequest{Method: "GET"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), FunctionTimeout(&Function{TimeoutMs: job.TimeoutMs}))
	defer cancel()

	start := time.Now()
	resp := RunLocalJob(ctx, job, slowFetchHost{})
	if resp.Error == "" {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(resp.Error, "timeout") {
		t.Fatalf("expected timeout message, got: %s", resp.Error)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("timeout took too long: %v", time.Since(start))
	}
}

func TestIsolatedSubprocessTimeoutInfiniteLoop(t *testing.T) {
	exe := buildWorkerBinary(t)
	t.Setenv("SOVRABASE_FN_WORKER_EXE", exe)

	job := WorkerJob{
		Mode:      WorkerModeHTTP,
		Language:  "javascript",
		Code:      `function handler(req) { while (true) {} }`,
		TimeoutMs: 500,
		HTTPReq:   HTTPRequest{Method: "GET"},
	}
	start := time.Now()
	result := runIsolatedSubprocess(context.Background(), job, DefaultHost)
	elapsed := time.Since(start)
	if result.Error == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(result.Error.Error(), "timeout") {
		t.Fatalf("expected timeout message, got: %s", result.Error.Error())
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("isolated timeout took too long: %v", elapsed)
	}
}

type memStorageHost struct {
	noopHost
	mu    sync.Mutex
	files map[string]map[string][]byte
	meta  map[string]map[string]map[string]interface{}
}

func newMemStorageHost() *memStorageHost {
	return &memStorageHost{
		files: make(map[string]map[string][]byte),
		meta:  make(map[string]map[string]map[string]interface{}),
	}
}

func (h *memStorageHost) StorageUpload(ctx context.Context, bucket, path, contentType string, data []byte) (map[string]interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.files[bucket] == nil {
		h.files[bucket] = make(map[string][]byte)
		h.meta[bucket] = make(map[string]map[string]interface{})
	}
	h.files[bucket][path] = append([]byte(nil), data...)
	info := map[string]interface{}{
		"bucket": bucket, "path": path, "size": len(data), "content_type": contentType,
	}
	h.meta[bucket][path] = info
	return info, nil
}

func (h *memStorageHost) StorageDownload(ctx context.Context, bucket, path string) ([]byte, map[string]interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	bucketFiles, ok := h.files[bucket]
	if !ok {
		return nil, nil, fmt.Errorf("not found")
	}
	data, ok := bucketFiles[path]
	if !ok {
		return nil, nil, fmt.Errorf("not found")
	}
	return append([]byte(nil), data...), h.meta[bucket][path], nil
}

func (h *memStorageHost) StorageDelete(ctx context.Context, bucket, path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.files[bucket], path)
	delete(h.meta[bucket], path)
	return nil
}

func (h *memStorageHost) StorageList(ctx context.Context, bucket, prefix string) ([]map[string]interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []map[string]interface{}
	for path, info := range h.meta[bucket] {
		if prefix == "" || strings.HasPrefix(path, prefix) {
			out = append(out, info)
		}
	}
	return out, nil
}

func TestRunLocalJobStorageBinding(t *testing.T) {
	host := newMemStorageHost()
	job := WorkerJob{
		Mode:     WorkerModeHTTP,
		Language: "javascript",
		Code: `function handler(req) {
			sovrabase.storage.upload("avatars", "a.txt", "hello", { contentType: "text/plain" });
			var f = sovrabase.storage.download("avatars", "a.txt");
			var files = sovrabase.storage.list("avatars");
			sovrabase.storage.delete("avatars", "a.txt");
			return { status: 200, body: { data: f.data, count: files.length } };
		}`,
		TimeoutMs: 5000,
		HTTPReq:   HTTPRequest{Method: "GET"},
	}
	resp := RunLocalJob(context.Background(), job, host)
	if resp.Error != "" {
		t.Fatalf("storage binding failed: %s", resp.Error)
	}
	if resp.HTTPResponse == nil || resp.HTTPResponse.Status != 200 {
		t.Fatalf("unexpected response: %+v", resp.HTTPResponse)
	}
}

func TestIsolatedSubprocessStorageRPC(t *testing.T) {
	exe := buildWorkerBinary(t)
	t.Setenv("SOVRABASE_FN_WORKER_EXE", exe)

	host := newMemStorageHost()
	job := WorkerJob{
		Mode:     WorkerModeHTTP,
		Language: "javascript",
		Code: `function handler(req) {
			sovrabase.storage.upload("data", "f.bin", "abc");
			return { status: 200, body: sovrabase.storage.download("data", "f.bin") };
		}`,
		TimeoutMs: 5000,
		HTTPReq:   HTTPRequest{Method: "GET"},
	}
	result := runIsolatedSubprocess(context.Background(), job, host)
	if result.Error != nil {
		t.Fatalf("isolated storage failed: %v", result.Error)
	}
	if result.HTTPResponse == nil || result.HTTPResponse.Status != 200 {
		t.Fatalf("unexpected response: %+v", result.HTTPResponse)
	}
}

func TestDecodeStorageDataBase64(t *testing.T) {
	data, err := DecodeStorageData("aGVsbG8=")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("got %q", data)
	}
}
