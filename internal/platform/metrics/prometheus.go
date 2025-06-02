package metrics

import (
	"fmt"
	strconv2 "strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"github.com/wallarm/api-firewall/internal/config"
)

const logMetricsPrefix = "prometheus metrics"

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

type Options struct {
	EndpointName string
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type PrometheusMetrics struct {
	logger      *zerolog.Logger
	serviceOpts *Options
	enabled     bool
}

var _ Metrics = (*PrometheusMetrics)(nil)

func NewPrometheusMetrics(enabled bool) *PrometheusMetrics {
	return &PrometheusMetrics{enabled: enabled}
}

func (p *PrometheusMetrics) StartService(logger *zerolog.Logger, options *Options) error {

	p.logger = logger
	p.serviceOpts = options

	if p.logger == nil {
		return fmt.Errorf("%s: logger not initialized", logMetricsPrefix)
	}

	p.initializeMetrics()

	// Prometheus service handler
	fastPrometheusHandler := fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())
	metricsHandler := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case p.serviceOpts.EndpointName:
			fastPrometheusHandler(ctx)
			return
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}

	metricsAPI := fasthttp.Server{
		Handler:               metricsHandler,
		ReadTimeout:           p.serviceOpts.ReadTimeout,
		WriteTimeout:          p.serviceOpts.WriteTimeout,
		NoDefaultServerHeader: true,
		Logger:                &config.ZerologAdapter{Logger: *p.logger},
	}

	// Start the service listening for requests.
	p.logger.Info().Msgf("%s: API listening on %s%s", logMetricsPrefix, p.serviceOpts.Host, p.serviceOpts.EndpointName)

	return metricsAPI.ListenAndServe(p.serviceOpts.Host)
}

// initializeMetrics initialized metrics
func (p *PrometheusMetrics) initializeMetrics() {
	prometheus.MustRegister(TotalErrors, ErrorTypeCounter, HttpRequestsTotal, HttpRequestDuration)
}

func (p *PrometheusMetrics) IncErrorTypeCounter(err string, schemaID int) {
	if !p.enabled {
		return
	}

	TotalErrors.Add(1)
	ErrorTypeCounter.WithLabelValues(err, strconv2.Itoa(schemaID)).Inc()
}

func (p *PrometheusMetrics) IncHTTPRequestStat(start time.Time, schemaID int, statusCode int) {
	if !p.enabled {
		return
	}

	HttpRequestDuration.WithLabelValues(strconv2.Itoa(schemaID)).Observe(time.Since(start).Seconds())
	HttpRequestsTotal.WithLabelValues(strconv2.Itoa(schemaID), strconv2.Itoa(statusCode)).Inc()
}

func (p *PrometheusMetrics) IncHTTPRequestTotalCountOnly(schemaID int, statusCode int) {
	if !p.enabled {
		return
	}

	HttpRequestsTotal.WithLabelValues(strconv2.Itoa(schemaID), strconv2.Itoa(statusCode)).Inc()
}
