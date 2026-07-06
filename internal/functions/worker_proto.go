package functions

// WorkerMode identifies how the function is invoked inside the worker.
type WorkerMode string

const (
	WorkerModeHTTP  WorkerMode = "http"
	WorkerModeEvent WorkerMode = "event"
)

// WorkerJob is sent to the isolated worker subprocess via stdin.
type WorkerJob struct {
	Mode      WorkerMode   `json:"mode"`
	Language  string       `json:"language"`
	Code      string       `json:"code"`
	TimeoutMs int          `json:"timeout_ms"`
	HTTPReq   HTTPRequest  `json:"http_req,omitempty"`
	Event     EventPayload `json:"event,omitempty"`
}

// WorkerResponse is returned from the worker subprocess on stdout.
type WorkerResponse struct {
	HTTPResponse *HTTPResponse    `json:"http_response,omitempty"`
	Output       string           `json:"output,omitempty"`
	Error        string           `json:"error,omitempty"`
	DurationMs   int64            `json:"duration_ms"`
	Metrics      ExecutionMetrics `json:"metrics,omitempty"`
}

// RPCMessage carries host API calls between worker and parent.
type RPCMessage struct {
	Type   string `json:"type"` // call, resp, err
	ID     int    `json:"id"`
	Method string `json:"method,omitempty"`
	Params []byte `json:"params,omitempty"`
	Result []byte `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}
