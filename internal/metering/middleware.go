package metering

import (
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

					// Wrap response writer to track download bandwidth
					mw := &meterResponseWriter{ResponseWriter: w}
					next.ServeHTTP(mw, r)

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

// meterResponseWriter wraps http.ResponseWriter to track bytes written.
type meterResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int
}

func (mw *meterResponseWriter) WriteHeader(code int) {
	mw.statusCode = code
	mw.ResponseWriter.WriteHeader(code)
}

func (mw *meterResponseWriter) Write(b []byte) (int, error) {
	if mw.statusCode == 0 {
		mw.statusCode = http.StatusOK
	}
	n, err := mw.ResponseWriter.Write(b)
	mw.written += n
	return n, err
}
