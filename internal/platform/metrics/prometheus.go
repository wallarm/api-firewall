package metrics

import (
	strconv2 "strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/savsgio/gotils/strconv"
)

// List of metrics
var (
	//todo: fix comments
	TotalErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "apifw_service_errors_total",
			Help: "Total number of errors occurred in the APIFW service.",
		})

	ErrorTypeCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "apifw_service_errors_by_type_total",
			Help: "Total number of errors by type and endpoint.",
		},
		[]string{"error_type", "method", "endpoint"},
	)

	// Counter: Total number of HTTP requests
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	// Histogram: HTTP request duration
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration in seconds",
			//Buckets: prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
			Buckets: []float64{.001, .005, .025, .05, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "endpoint"},
	)
)

// InitializeMetrics metrics
func InitializeMetrics() {
	prometheus.MustRegister(TotalErrors, ErrorTypeCounter, HttpRequestsTotal, HttpRequestDuration)
}

func IncErrorTypeCounter(err string, method, endpoint []byte) {
	TotalErrors.Add(1)
	ErrorTypeCounter.WithLabelValues(err, strconv.B2S(method), normalizeEndpoint(strconv.B2S(endpoint))).Inc()
}

func IncHTTPRequestStat(start time.Time, method, endpoint []byte, statusCode int) {
	HttpRequestDuration.WithLabelValues(strconv.B2S(method), normalizeEndpoint(strconv.B2S(endpoint))).Observe(time.Since(start).Seconds())
	HttpRequestsTotal.WithLabelValues(strconv.B2S(method), normalizeEndpoint(strconv.B2S(endpoint)), strconv2.Itoa(statusCode)).Inc()
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
