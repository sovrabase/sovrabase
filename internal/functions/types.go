package functions

import "time"

// HTTPRequest is passed to HTTP-triggered functions.
type HTTPRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

// HTTPResponse is returned by HTTP-triggered functions.
type HTTPResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

// EventPayload is passed to event-triggered functions.
type EventPayload struct {
	Type       string                 `json:"type"`
	Collection string                 `json:"collection,omitempty"`
	ID         string                 `json:"id,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	UserEmail  string                 `json:"user_email,omitempty"`
}

// RunResult holds the outcome of a function execution.
type RunResult struct {
	HTTPResponse *HTTPResponse
	Output       string
	Error        error
	Duration     time.Duration
	Metrics      ExecutionMetrics
}
