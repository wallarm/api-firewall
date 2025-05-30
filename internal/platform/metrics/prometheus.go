package metrics

import (
	strconv2 "strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// List of metrics
var (
	// Counter: Total number of errors
	TotalErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "wallarm_apifw_service_errors_total",
			Help: "Total number of errors occurred in the APIFW service.",
		})

	// Counter: Errors by types
	ErrorTypeCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallarm_apifw_service_errors_by_type",
			Help: "Total number of errors by type and endpoint.",
		},
		[]string{"error_type", "schema_id"},
	)

	// Counter: Total number of HTTP requests
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallarm_apifw_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"schema_id", "status_code"},
	)

	// Histogram: HTTP request duration
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "wallarm_apifw_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.001, .005, .025, .05, .25, .5, 1, 2.5, 5},
		},
		[]string{"schema_id"},
	)
)

// InitializeMetrics metrics
func InitializeMetrics() {
	prometheus.MustRegister(TotalErrors, ErrorTypeCounter, HttpRequestsTotal, HttpRequestDuration)
}

func IncErrorTypeCounter(err string, schemaID int) {
	TotalErrors.Add(1)
	ErrorTypeCounter.WithLabelValues(err, strconv2.Itoa(schemaID)).Inc()
}

func IncHTTPRequestStat(start time.Time, schemaID int, statusCode int) {
	HttpRequestDuration.WithLabelValues(strconv2.Itoa(schemaID)).Observe(time.Since(start).Seconds())
	HttpRequestsTotal.WithLabelValues(strconv2.Itoa(schemaID), strconv2.Itoa(statusCode)).Inc()
}

// Normalize endpoints for metrics (replace dynamic parts)
func normalizeEndpoint(path string) string {
	// Replace numeric IDs with placeholder
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part != "" {
			// Check if part is numeric (ID)
			if _, err := strconv2.Atoi(part); err == nil {
				parts[i] = "{id}"
			}
		}
	}

	normalized := strings.Join(parts, "/")
	if normalized == "" {
		return "/"
	}
	return normalized
}
