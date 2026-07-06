package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

const fnWorkerArg = "__fn_worker__"

var (
	isolatedEnabled = true
	isolatedMu      sync.RWMutex
)

// SetIsolatedExecution toggles subprocess isolation (for tests).
func SetIsolatedExecution(enabled bool) {
	isolatedMu.Lock()
	isolatedEnabled = enabled
	isolatedMu.Unlock()
}

func runFunctionIsolated(
	ctx context.Context,
	fn *Function,
	mode WorkerMode,
	req HTTPRequest,
	event EventPayload,
	host HostAPI,
) *RunResult {
	isolatedMu.RLock()
	useIsolated := isolatedEnabled
	isolatedMu.RUnlock()

	if !useIsolated {
		return runFunctionInProcess(ctx, fn, mode, req, event, host)
	}

	job := WorkerJob{
		Mode:      mode,
		Language:  fn.Language,
		Code:      fn.Code,
		TimeoutMs: fn.TimeoutMs,
		HTTPReq:   req,
		Event:     event,
	}
	return runIsolatedSubprocess(ctx, job, host)
}

func runFunctionInProcess(
	ctx context.Context,
	fn *Function,
	mode WorkerMode,
	req HTTPRequest,
	event EventPayload,
	host HostAPI,
) *RunResult {
	collector := (&MetricsCollector{}).Start()
	job := WorkerJob{
		Mode: mode, Language: fn.Language, Code: fn.Code, TimeoutMs: fn.TimeoutMs,
		HTTPReq: req, Event: event,
	}
	resp := RunLocalJob(ctx, job, host)
	res := workerResponseToRunResult(resp)
	metrics := collector.Stop()
	metrics.Isolated = false
	res.Metrics = metrics
	return res
}

func runIsolatedSubprocess(ctx context.Context, job WorkerJob, host HostAPI) *RunResult {
	start := time.Now()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return &RunResult{Error: fmt.Errorf("isolated: listen rpc: %w", err), Duration: time.Since(start)}
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	rpcReady := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			rpcReady <- conn
		}
	}()

	exe, err := resolveWorkerExecutable()
	if err != nil {
		return &RunResult{Error: err, Duration: time.Since(start)}
	}

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return &RunResult{Error: err, Duration: time.Since(start)}
	}

	timeout := FunctionTimeout(&Function{TimeoutMs: job.TimeoutMs})
	execCtx, cancel := context.WithTimeout(ctx, timeout+500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(execCtx, exe, fnWorkerArg)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SOVRABASE_FN_RPC_PORT=%d", port))
	cmd.Stdin = bytes.NewReader(jobJSON)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return &RunResult{Error: fmt.Errorf("isolated: start worker: %w", err), Duration: time.Since(start)}
	}

	pid := cmd.Process.Pid
	collector := StartChildMetrics(pid)

	var rpcConn net.Conn
	select {
	case rpcConn = <-rpcReady:
		go serveHostRPC(rpcConn, host)
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return &RunResult{Error: fmt.Errorf("isolated: rpc handshake timeout"), Duration: time.Since(start)}
	}

	waitErr := cmd.Wait()
	metrics := collector.Stop(cmd.ProcessState)
	metrics.Isolated = true

	var resp WorkerResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		errMsg := fmt.Sprintf("isolated: decode response: %v", err)
		if waitErr != nil {
			errMsg = waitErr.Error()
		}
		return &RunResult{
			Error:    errString(errMsg),
			Duration: time.Since(start),
			Metrics:  metrics,
		}
	}

	result := workerResponseToRunResult(resp)
	result.Metrics = metrics
	if result.Duration == 0 {
		result.Duration = time.Since(start)
	}
	if waitErr != nil && result.Error == nil {
		result.Error = waitErr
	}
	return result
}

// WorkerMain is the entrypoint for the isolated function worker subprocess.
func WorkerMain() error {
	var job WorkerJob
	if err := json.NewDecoder(os.Stdin).Decode(&job); err != nil {
		return fmt.Errorf("worker: decode job: %w", err)
	}
	port := os.Getenv("SOVRABASE_FN_RPC_PORT")
	if port == "" {
		return fmt.Errorf("worker: missing SOVRABASE_FN_RPC_PORT")
	}
	conn, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		return fmt.Errorf("worker: rpc dial: %w", err)
	}
	defer conn.Close()

	timeout := FunctionTimeout(&Function{TimeoutMs: job.TimeoutMs})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	host := newRPCHost(conn, ctx)

	resp := RunLocalJob(ctx, job, host)
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(resp)
}

// IsFnWorkerProcess reports whether argv indicates worker mode.
func IsFnWorkerProcess(args []string) bool {
	return len(args) > 1 && args[1] == fnWorkerArg
}

func resolveWorkerExecutable() (string, error) {
	if p := os.Getenv("SOVRABASE_FN_WORKER_EXE"); p != "" {
		return p, nil
	}
	return os.Executable()
}
