package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// sovrabaseHttpRequestsTotal counts all HTTP requests by method, path, and status code.
	sovrabaseHttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sovrabase_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// sovrabaseHttpRequestDurationSeconds observes request latency in seconds.
	sovrabaseHttpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sovrabase_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// sovrabaseActiveProjects reports the current number of active projects.
	sovrabaseActiveProjects = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sovrabase_active_projects",
			Help: "Number of active projects.",
		},
	)

	// sovrabaseDbOperationsTotal counts database operations by operation type and collection.
	sovrabaseDbOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sovrabase_db_operations_total",
			Help: "Total number of database operations.",
		},
		[]string{"operation", "collection"},
	)
)

func init() {
	prometheus.MustRegister(
		sovrabaseHttpRequestsTotal,
		sovrabaseHttpRequestDurationSeconds,
		sovrabaseActiveProjects,
		sovrabaseDbOperationsTotal,
	)
}

// MetricsMiddleware is an HTTP middleware that counts requests and observes duration.
// It wraps the next handler and records method, path, and status code.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture the status code.
		lrw := newLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(lrw.statusCode)
		path := r.URL.Path

		sovrabaseHttpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		sovrabaseHttpRequestDurationSeconds.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// HandleMetrics serves the Prometheus metrics endpoint.
func HandleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}

// loggingResponseWriter wraps http.ResponseWriter to capture the status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// SetActiveProjects sets the active projects gauge value.
func SetActiveProjects(count int) {
	sovrabaseActiveProjects.Set(float64(count))
}

// IncDbOperation increments the database operations counter.
func IncDbOperation(operation, collection string) {
	sovrabaseDbOperationsTotal.WithLabelValues(operation, collection).Inc()
}
