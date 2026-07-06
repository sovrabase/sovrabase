package functions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	workerBinaryOnce sync.Once
	workerBinaryPath string
	workerBinaryErr  error
)

func buildWorkerBinary(t *testing.T) string {
	t.Helper()
	workerBinaryOnce.Do(func() {
		root, err := findModuleRoot()
		if err != nil {
			workerBinaryErr = err
			return
		}
		dir, err := os.MkdirTemp("", "sovrabase-worker-build-*")
		if err != nil {
			workerBinaryErr = err
			return
		}
		exe := filepath.Join(dir, "sovrabase-worker")
		if runtime.GOOS == "windows" {
			exe += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", exe, "./cmd/sovrabase")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			workerBinaryErr = fmt.Errorf("%v\n%s", err, out)
			return
		}
		workerBinaryPath = exe
	})
	if workerBinaryErr != nil {
		t.Fatalf("build worker binary: %v", workerBinaryErr)
	}
	return workerBinaryPath
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := findModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunLocalJobHTTP(t *testing.T) {
	job := WorkerJob{
		Mode:      WorkerModeHTTP,
		Language:  "javascript",
		Code:      `function handler(req) { return { status: 200, body: { ok: true } }; }`,
		TimeoutMs: 5000,
		HTTPReq:   HTTPRequest{Method: "GET"},
	}
	resp := RunLocalJob(context.Background(), job, DefaultHost)
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.HTTPResponse == nil || resp.HTTPResponse.Status != 200 {
		t.Fatalf("unexpected response: %+v", resp.HTTPResponse)
	}
}

func TestIsolatedSubprocessHTTP(t *testing.T) {
	exe := buildWorkerBinary(t)
	t.Setenv("SOVRABASE_FN_WORKER_EXE", exe)

	job := WorkerJob{
		Mode:      WorkerModeHTTP,
		Language:  "javascript",
		Code:      `function handler(req) { return { status: 200, body: { ok: true } }; }`,
		TimeoutMs: 5000,
		HTTPReq:   HTTPRequest{Method: "GET"},
	}
	result := runIsolatedSubprocess(context.Background(), job, DefaultHost)
	if result.Error != nil {
		t.Fatalf("isolated run failed: %v", result.Error)
	}
	if result.HTTPResponse == nil || result.HTTPResponse.Status != 200 {
		t.Fatalf("unexpected response: %+v", result.HTTPResponse)
	}
	if !result.Metrics.Isolated {
		t.Fatal("expected isolated metrics")
	}
	if result.Metrics.DurationMs <= 0 {
		t.Fatal("expected positive duration")
	}
}

func TestIsolatedDisabledFallback(t *testing.T) {
	SetIsolatedExecution(false)
	t.Cleanup(func() { SetIsolatedExecution(true) })

	fn := &Function{
		Language:  "javascript",
		Code:      `function handler(req) { return { status: 200, body: {} }; }`,
		TimeoutMs: 5000,
	}
	result := runFunctionIsolated(context.Background(), fn, WorkerModeHTTP, HTTPRequest{Method: "GET"}, EventPayload{}, DefaultHost)
	if result.Error != nil {
		t.Fatalf("fallback run failed: %v", result.Error)
	}
	if result.Metrics.Isolated {
		t.Fatal("expected non-isolated metrics in fallback mode")
	}
}

func TestIsFnWorkerProcess(t *testing.T) {
	if !IsFnWorkerProcess([]string{"sovrabase", "__fn_worker__"}) {
		t.Fatal("expected worker argv detection")
	}
	if IsFnWorkerProcess(os.Args) {
		t.Fatal("test process should not be worker")
	}
}
