package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

type jsRuntimeImpl struct {
	logger *slog.Logger
}

var jsRuntime FunctionRuntime = &jsRuntimeImpl{}

func (rt *jsRuntimeImpl) loggerOrDefault() *slog.Logger {
	if rt.logger != nil {
		return rt.logger
	}
	return slog.Default().With("component", "functions-runtime-js")
}

func (rt *jsRuntimeImpl) RunHTTP(ctx context.Context, fn *Function, req HTTPRequest, host HostAPI) *RunResult {
	return runFunctionIsolated(ctx, fn, WorkerModeHTTP, req, EventPayload{}, host)
}

func (rt *jsRuntimeImpl) RunEvent(ctx context.Context, fn *Function, event EventPayload, host HostAPI) *RunResult {
	return runFunctionIsolated(ctx, fn, WorkerModeEvent, HTTPRequest{}, event, host)
}

func (rt *jsRuntimeImpl) runLocalHTTP(ctx context.Context, fn *Function, req HTTPRequest, host HostAPI) *RunResult {
	if host == nil {
		host = DefaultHost
	}
	host = withContext(ctx, host)
	start := time.Now()
	result := &RunResult{}

	vm := goja.New()
	logBuf := &consoleBuffer{logger: rt.loggerOrDefault()}
	rt.bindConsole(vm, logBuf)
	rt.bindJSON(vm)
	rt.bindHost(vm, host)

	reqVal, err := gojaToValue(vm, req)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}
	if err := vm.Set("req", reqVal); err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	done := make(chan struct{})
	var runErr error
	go func() {
		defer close(done)
		if _, err := vm.RunString(fn.Code); err != nil {
			runErr = err
			return
		}
		handler := vm.Get("handler")
		if handler != nil && !goja.IsUndefined(handler) && !goja.IsNull(handler) {
			if callable, ok := goja.AssertFunction(handler); ok {
				ret, err := callable(goja.Undefined(), reqVal)
				if err != nil {
					runErr = err
					return
				}
				result.HTTPResponse = parseHTTPResponse(vm, ret)
			}
		}
	}()

	go func() {
		<-ctx.Done()
		vm.Interrupt("timeout")
	}()

	timedOut := false
	select {
	case <-ctx.Done():
		timedOut = true
		result.Error = contextRunError(ctx, fn)
	case <-done:
		result.Error = runErr
	}
	if timedOut {
		joinAfterInterrupt(done)
	}

	if result.HTTPResponse == nil && result.Error == nil {
		result.HTTPResponse = &HTTPResponse{Status: 200, Body: map[string]string{"status": "ok"}}
	}
	result.Output = logBuf.String()
	result.Duration = time.Since(start)
	return result
}

func (rt *jsRuntimeImpl) runLocalEvent(ctx context.Context, fn *Function, event EventPayload, host HostAPI) *RunResult {
	if host == nil {
		host = DefaultHost
	}
	host = withContext(ctx, host)
	start := time.Now()
	result := &RunResult{}

	vm := goja.New()
	logBuf := &consoleBuffer{logger: rt.loggerOrDefault()}
	rt.bindConsole(vm, logBuf)
	rt.bindJSON(vm)
	rt.bindHost(vm, host)

	eventVal, err := gojaToValue(vm, event)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}
	if err := vm.Set("event", eventVal); err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	done := make(chan struct{})
	var runErr error
	go func() {
		defer close(done)
		if _, err := vm.RunString(fn.Code); err != nil {
			runErr = err
		}
	}()

	go func() {
		<-ctx.Done()
		vm.Interrupt("timeout")
	}()

	timedOut := false
	select {
	case <-ctx.Done():
		timedOut = true
		result.Error = contextRunError(ctx, fn)
	case <-done:
		result.Error = runErr
	}
	if timedOut {
		joinAfterInterrupt(done)
	}

	result.Output = logBuf.String()
	result.Duration = time.Since(start)
	return result
}

type consoleBuffer struct {
	mu     sync.Mutex
	lines  []string
	logger *slog.Logger
}

func (b *consoleBuffer) log(args ...interface{}) {
	line := fmt.Sprint(args...)
	b.mu.Lock()
	b.lines = append(b.lines, line)
	b.mu.Unlock()
	b.logger.Info("function log", "line", line)
}

func (b *consoleBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Join(b.lines, "\n")
}

func (rt *jsRuntimeImpl) bindConsole(vm *goja.Runtime, buf *consoleBuffer) {
	console := vm.NewObject()
	_ = console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, a := range call.Arguments {
			args[i] = a.Export()
		}
		buf.log(args...)
		return goja.Undefined()
	})
	_ = vm.Set("console", console)
}

func (rt *jsRuntimeImpl) bindJSON(vm *goja.Runtime) {
	jsonObj := vm.NewObject()
	_ = jsonObj.Set("stringify", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("null")
		}
		b, err := json.Marshal(call.Arguments[0].Export())
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return vm.ToValue(string(b))
	})
	_ = jsonObj.Set("parse", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		s := call.Arguments[0].String()
		var v interface{}
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		val, err := gojaToValue(vm, v)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return val
	})
	_ = vm.Set("JSON", jsonObj)
}

func (rt *jsRuntimeImpl) bindHost(vm *goja.Runtime, host HostAPI) {
	sb := vm.NewObject()
	db := vm.NewObject()
	_ = db.Set("get", func(call goja.FunctionCall) goja.Value {
		coll := call.Argument(0).String()
		id := call.Argument(1).String()
		v, err := host.DBGet(coll, id)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		val, _ := gojaToValue(vm, v)
		return val
	})
	_ = db.Set("create", func(call goja.FunctionCall) goja.Value {
		coll := call.Argument(0).String()
		data, _ := call.Argument(1).Export().(map[string]interface{})
		v, err := host.DBCreate(coll, data)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		val, _ := gojaToValue(vm, v)
		return val
	})
	_ = db.Set("update", func(call goja.FunctionCall) goja.Value {
		coll := call.Argument(0).String()
		id := call.Argument(1).String()
		data, _ := call.Argument(2).Export().(map[string]interface{})
		v, err := host.DBUpdate(coll, id, data)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		val, _ := gojaToValue(vm, v)
		return val
	})
	_ = db.Set("delete", func(call goja.FunctionCall) goja.Value {
		coll := call.Argument(0).String()
		id := call.Argument(1).String()
		if err := host.DBDelete(coll, id); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return goja.Undefined()
	})
	_ = sb.Set("db", db)

	storage := vm.NewObject()
	_ = storage.Set("upload", func(call goja.FunctionCall) goja.Value {
		bucket := call.Argument(0).String()
		path := call.Argument(1).String()
		data := exportJSBytes(call.Argument(2))
		contentType := "application/octet-stream"
		if len(call.Arguments) > 3 && !goja.IsUndefined(call.Argument(3)) {
			if opts, ok := call.Argument(3).Export().(map[string]interface{}); ok {
				if ct, ok := opts["contentType"].(string); ok {
					contentType = ct
				}
			}
		}
		v, err := host.StorageUpload(context.Background(), bucket, path, contentType, data)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		val, _ := gojaToValue(vm, v)
		return val
	})
	_ = storage.Set("download", func(call goja.FunctionCall) goja.Value {
		bucket := call.Argument(0).String()
		path := call.Argument(1).String()
		data, meta, err := host.StorageDownload(context.Background(), bucket, path)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		result := map[string]interface{}{"data": string(data)}
		for k, v := range meta {
			result[k] = v
		}
		val, _ := gojaToValue(vm, result)
		return val
	})
	_ = storage.Set("delete", func(call goja.FunctionCall) goja.Value {
		bucket := call.Argument(0).String()
		path := call.Argument(1).String()
		if err := host.StorageDelete(context.Background(), bucket, path); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return goja.Undefined()
	})
	_ = storage.Set("list", func(call goja.FunctionCall) goja.Value {
		bucket := call.Argument(0).String()
		prefix := ""
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Argument(1)) {
			prefix = call.Argument(1).String()
		}
		files, err := host.StorageList(context.Background(), bucket, prefix)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		val, _ := gojaToValue(vm, files)
		return val
	})
	_ = sb.Set("storage", storage)
	_ = vm.Set("sovrabase", sb)

	_ = vm.Set("fetch", func(call goja.FunctionCall) goja.Value {
		url := call.Argument(0).String()
		opts := map[string]interface{}{}
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Argument(1)) {
			opts, _ = call.Argument(1).Export().(map[string]interface{})
		}
		method := "GET"
		if m, ok := opts["method"].(string); ok {
			method = m
		}
		headers := map[string]string{}
		if h, ok := opts["headers"].(map[string]interface{}); ok {
			for k, v := range h {
				headers[k] = fmt.Sprint(v)
			}
		}
		status, body, err := host.Fetch(context.Background(), method, url, headers, opts["body"])
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		var parsed interface{}
		_ = json.Unmarshal(body, &parsed)
		if parsed == nil {
			parsed = string(body)
		}
		result := map[string]interface{}{
			"status": status,
			"body":   parsed,
			"ok":     status >= 200 && status < 300,
		}
		val, _ := gojaToValue(vm, result)
		return val
	})
}

func exportJSBytes(val goja.Value) []byte {
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return nil
	}
	switch exp := val.Export().(type) {
	case string:
		return []byte(exp)
	case []byte:
		return exp
	default:
		return []byte(fmt.Sprint(exp))
	}
}

func parseHTTPResponse(vm *goja.Runtime, val goja.Value) *HTTPResponse {
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return &HTTPResponse{Status: 200, Body: nil}
	}
	exp := val.Export()
	switch v := exp.(type) {
	case map[string]interface{}:
		resp := &HTTPResponse{Status: 200, Headers: map[string]string{}}
		if s, ok := v["status"].(float64); ok {
			resp.Status = int(s)
		}
		if h, ok := v["headers"].(map[string]interface{}); ok {
			for k, hv := range h {
				resp.Headers[k] = fmt.Sprint(hv)
			}
		}
		resp.Body = v["body"]
		return resp
	default:
		return &HTTPResponse{Status: 200, Body: exp}
	}
}

func gojaToValue(vm *goja.Runtime, v interface{}) (goja.Value, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return goja.Undefined(), err
	}
	return vm.RunString("(" + string(b) + ")")
}

// NewRuntime creates a JS function runtime (legacy helper).
func NewRuntime() *jsRuntimeImpl {
	return &jsRuntimeImpl{logger: slog.Default().With("component", "functions-runtime")}
}
