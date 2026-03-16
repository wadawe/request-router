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
	mh           *Metrics // Metrics handler instance
	metricLabels = []string{"router", "path", "method", "target", "status"}
)

// Register prometheus metrics and handlers
func RegisterMetrics(mux *http.ServeMux) {
	mh = &Metrics{}

	// Initialise RequestsTotal metric
	mh.RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_requests_total",
			Help: "Total number of requests processed by the relay",
		},
		metricLabels,
	)
	prometheus.MustRegister(mh.RequestsTotal)

	// Initialise RequestDurationSeconds metric
	mh.RequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "relay_request_duration_seconds",
			Help:    "Duration of requests processed by the relay",
			Buckets: prometheus.DefBuckets,
		},
		metricLabels,
	)
	prometheus.MustRegister(mh.RequestDurationSeconds)

	// ...

	// Register the /metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())
}

// Get the Metrics handler instance
func GetMetricsHandler() *Metrics {
	return mh
}
