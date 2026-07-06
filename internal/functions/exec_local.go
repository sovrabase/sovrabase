package functions

import (
	"context"
	"time"
)

// RunLocalJob executes a function in-process (used by the worker subprocess).
func RunLocalJob(ctx context.Context, job WorkerJob, host HostAPI) WorkerResponse {
	start := time.Now()
	fn := &Function{
		Code:      job.Code,
		TimeoutMs: job.TimeoutMs,
		Language:  job.Language,
	}

	var result *RunResult
	switch NormalizeLanguage(job.Language) {
	case "go":
		rt := &goRuntimeImpl{}
		switch job.Mode {
		case WorkerModeHTTP:
			result = rt.runLocalHTTP(ctx, fn, job.HTTPReq, host)
		default:
			result = rt.runLocalEvent(ctx, fn, job.Event, host)
		}
	default:
		rt := &jsRuntimeImpl{}
		switch job.Mode {
		case WorkerModeHTTP:
			result = rt.runLocalHTTP(ctx, fn, job.HTTPReq, host)
		default:
			result = rt.runLocalEvent(ctx, fn, job.Event, host)
		}
	}

	resp := WorkerResponse{
		HTTPResponse: result.HTTPResponse,
		Output:       result.Output,
		DurationMs:   result.Duration.Milliseconds(),
	}
	if result.Error != nil {
		resp.Error = result.Error.Error()
	}
	if resp.DurationMs == 0 {
		resp.DurationMs = time.Since(start).Milliseconds()
	}
	return resp
}

func workerResponseToRunResult(resp WorkerResponse) *RunResult {
	res := &RunResult{
		HTTPResponse: resp.HTTPResponse,
		Output:       resp.Output,
		Duration:     time.Duration(resp.DurationMs) * time.Millisecond,
		Metrics:      resp.Metrics,
	}
	if resp.Error != "" {
		res.Error = errString(resp.Error)
	}
	return res
}

type errString string

func (e errString) Error() string { return string(e) }
