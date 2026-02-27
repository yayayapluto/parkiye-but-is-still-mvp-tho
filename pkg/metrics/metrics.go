package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal counts total HTTP requests by method, path, and status code.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "parkieee",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration tracks request latency by method and path.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "parkieee",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "path"},
	)

	// HTTPActiveRequests tracks currently in-flight requests.
	HTTPActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "parkieee",
			Name:      "http_active_requests",
			Help:      "Number of HTTP requests currently being processed.",
		},
	)

	// DBQueryDuration tracks database query latency by operation.
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "parkieee",
			Name:      "db_query_duration_seconds",
			Help:      "Database query duration in seconds.",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		},
		[]string{"operation"},
	)
)
