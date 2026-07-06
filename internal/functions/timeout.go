package functions

import (
	"context"
	"fmt"
	"time"
)

const defaultFunctionTimeout = 5 * time.Second

// FunctionTimeout returns the execution deadline for a function.
func FunctionTimeout(fn *Function) time.Duration {
	if fn == nil || fn.TimeoutMs <= 0 {
		return defaultFunctionTimeout
	}
	return time.Duration(fn.TimeoutMs) * time.Millisecond
}

func functionTimeoutMs(fn *Function) int64 {
	if fn == nil || fn.TimeoutMs <= 0 {
		return int64(defaultFunctionTimeout / time.Millisecond)
	}
	return int64(fn.TimeoutMs)
}

func contextRunError(ctx context.Context, fn *Function) error {
	if ctx.Err() == context.Canceled {
		return fmt.Errorf("function cancelled")
	}
	return fmt.Errorf("function timeout after %dms", functionTimeoutMs(fn))
}

func joinAfterInterrupt(done <-chan struct{}) {
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
	}
}
