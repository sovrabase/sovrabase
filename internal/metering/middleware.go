package metering

import (
	"io"
	"net/http"

	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// MeteringMiddleware returns an HTTP middleware that tracks API requests
// per project. It identifies the project from the X-Project-Key header.
// It uses IncMethod to atomically increment both the total and per-method counters.
func MeteringMiddleware(meterStore *MeterStore, projectManager *tenant.ProjectManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			projectKey := r.Header.Get("X-Project-Key")
			if projectKey == "" {
				projectKey = r.URL.Query().Get("project_key")
			}

			if projectKey != "" {
				proj, err := projectManager.GetProjectBySecret(projectKey)
				if err == nil && proj != nil {
					// Track the API request by method
					_ = meterStore.IncMethod(proj.ID, r.Method, 1)

					// Wrap request body to track upload bandwidth accurately.
					// r.ContentLength is unreliable (often -1 for chunked/multipart).
					// We count bytes as the handler reads the body.
					mrb := &meterRequestBody{ReadCloser: r.Body}
					r.Body = mrb

					// Wrap response writer to track download bandwidth
					mw := &MeterResponseWriter{ResponseWriter: w}
					next.ServeHTTP(mw, r)

					// Track upload bandwidth (actual bytes read from body)
					if mrb.read > 0 {
						_ = meterStore.Inc(proj.ID, MetricBandwidthUp, mrb.read)
					}

					// Track download bandwidth for responses with content
					if mw.written > 0 {
						_ = meterStore.Inc(proj.ID, MetricBandwidthDown, int64(mw.written))
					}
					return
				}
			}

			// No project key found or invalid — pass through without metering
			next.ServeHTTP(w, r)
		})
	}
}

// meterRequestBody wraps http.Request.Body to count bytes actually read.
type meterRequestBody struct {
	io.ReadCloser
	read int64
}

func (mr *meterRequestBody) Read(p []byte) (int, error) {
	n, err := mr.ReadCloser.Read(p)
	mr.read += int64(n)
	return n, err
}

// MeterResponseWriter wraps http.ResponseWriter to track bytes written.
type MeterResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int
}

func (mw *MeterResponseWriter) WriteHeader(code int) {
	mw.statusCode = code
	mw.ResponseWriter.WriteHeader(code)
}

func (mw *MeterResponseWriter) Write(b []byte) (int, error) {
	if mw.statusCode == 0 {
		mw.statusCode = http.StatusOK
	}
	n, err := mw.ResponseWriter.Write(b)
	mw.written += n
	return n, err
}

// Written returns the total number of bytes written to the response.
func (mw *MeterResponseWriter) Written() int {
	return mw.written
}
