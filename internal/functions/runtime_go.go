package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type goRuntimeImpl struct{}

func (rt *goRuntimeImpl) RunHTTP(ctx context.Context, fn *Function, req HTTPRequest, host HostAPI) *RunResult {
	return runFunctionIsolated(ctx, fn, WorkerModeHTTP, req, EventPayload{}, host)
}

func (rt *goRuntimeImpl) RunEvent(ctx context.Context, fn *Function, event EventPayload, host HostAPI) *RunResult {
	return runFunctionIsolated(ctx, fn, WorkerModeEvent, HTTPRequest{}, event, host)
}

func (rt *goRuntimeImpl) runLocalHTTP(ctx context.Context, fn *Function, req HTTPRequest, host HostAPI) *RunResult {
	if host == nil {
		host = DefaultHost
	}
	host = withContext(ctx, host)
	start := time.Now()
	result := &RunResult{}

	handler, err := rt.compileHandler(fn.Code, host)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	reqMap := structToMap(req)
	done := make(chan struct{})
	var runErr error
	var resp map[string]interface{}
	go func() {
		defer close(done)
		out := handler.Call([]reflect.Value{reflect.ValueOf(reqMap)})
		if len(out) > 0 {
			if m, ok := out[0].Interface().(map[string]interface{}); ok {
				resp = m
			}
		}
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

	if result.Error == nil && resp != nil {
		result.HTTPResponse = mapToHTTPResponse(resp)
	} else if result.Error == nil {
		result.HTTPResponse = &HTTPResponse{Status: 200, Body: map[string]string{"status": "ok"}}
	}
	result.Duration = time.Since(start)
	return result
}

func (rt *goRuntimeImpl) runLocalEvent(ctx context.Context, fn *Function, event EventPayload, host HostAPI) *RunResult {
	if host == nil {
		host = DefaultHost
	}
	host = withContext(ctx, host)
	start := time.Now()
	result := &RunResult{}

	i, err := rt.newInterpreter(host)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	wrapped := fmt.Sprintf(`package function
import "encoding/json"
var event map[string]interface{}
func init() {
	_ = json.Unmarshal([]byte(%q), &event)
}
%s`, mustJSON(event), fn.Code)

	done := make(chan struct{})
	var runErr error
	go func() {
		defer close(done)
		_, runErr = i.Eval(wrapped)
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

	result.Duration = time.Since(start)
	return result
}

func (rt *goRuntimeImpl) compileHandler(code string, host HostAPI) (reflect.Value, error) {
	i, err := rt.newInterpreter(host)
	if err != nil {
		return reflect.Value{}, err
	}
	wrapped := "package function\n" + code
	if _, err := i.Eval(wrapped); err != nil {
		return reflect.Value{}, fmt.Errorf("go compile: %w", err)
	}
	v, err := i.Eval("function.Handler")
	if err != nil {
		return reflect.Value{}, fmt.Errorf("go: Handler function not found")
	}
	return v, nil
}

func (rt *goRuntimeImpl) newInterpreter(host HostAPI) (*interp.Interpreter, error) {
	i := interp.New(interp.Options{})
	if err := i.Use(stdlib.Symbols); err != nil {
		return nil, err
	}
	symbols := map[string]map[string]reflect.Value{
		"function/host": {
			"Fetch": reflect.ValueOf(func(method, url string, body interface{}) (map[string]interface{}, error) {
				status, data, err := host.Fetch(context.Background(), method, url, nil, body)
				if err != nil {
					return nil, err
				}
				var parsed interface{}
				_ = json.Unmarshal(data, &parsed)
				return map[string]interface{}{
					"status": status,
					"body":   parsed,
					"ok":     status >= 200 && status < 300,
				}, nil
			}),
		},
	}
	if err := i.Use(symbols); err != nil {
		return nil, err
	}
	return i, nil
}

func structToMap(v interface{}) map[string]interface{} {
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}

func mapToHTTPResponse(m map[string]interface{}) *HTTPResponse {
	resp := &HTTPResponse{Status: 200, Headers: map[string]string{}}
	if s, ok := m["status"].(float64); ok {
		resp.Status = int(s)
	} else if s, ok := m["status"].(int); ok {
		resp.Status = s
	}
	if h, ok := m["headers"].(map[string]interface{}); ok {
		for k, v := range h {
			resp.Headers[k] = fmt.Sprint(v)
		}
	}
	resp.Body = m["body"]
	return resp
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// NormalizeLanguage returns the canonical language tag.
func NormalizeLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		return "javascript"
	}
	return lang
}
