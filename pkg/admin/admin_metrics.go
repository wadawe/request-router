package admin

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	RequestsTotal          *prometheus.CounterVec
	RequestDurationSeconds *prometheus.HistogramVec
	// Add more metrics as needed
}

// Global variables
var (
	metricsHandler *Metrics
	metricLabels   = []string{"router", "path", "method", "target", "status"}
)

// Register prometheus metrics and handlers
func RegisterMetrics(mux *http.ServeMux) {
	metricsHandler = &Metrics{}

	// Initialise RequestsTotal metric
	metricsHandler.RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_requests_total",
			Help: "Total number of requests processed by the relay",
		},
		metricLabels,
	)
	prometheus.MustRegister(metricsHandler.RequestsTotal)

	// Initialise RequestDurationSeconds metric
	metricsHandler.RequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "relay_request_duration_seconds",
			Help:    "Duration of requests processed by the relay",
			Buckets: prometheus.DefBuckets,
		},
		metricLabels,
	)
	prometheus.MustRegister(metricsHandler.RequestDurationSeconds)

	// ...

	// Register the /metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())
}

// Get the Metrics handler instance
func GetMetricsHandler() *Metrics {
	return metricsHandler
}
