package functions

import "context"

// FunctionRuntime executes user code for a given language.
type FunctionRuntime interface {
	RunHTTP(ctx context.Context, fn *Function, req HTTPRequest, host HostAPI) *RunResult
	RunEvent(ctx context.Context, fn *Function, event EventPayload, host HostAPI) *RunResult
}

// SelectRuntime returns the runtime for a function's language.
func SelectRuntime(lang string) FunctionRuntime {
	switch NormalizeLanguage(lang) {
	case "go":
		return goRuntime
	default:
		return jsRuntime
	}
}

var (
	goRuntime FunctionRuntime = &goRuntimeImpl{}
)
