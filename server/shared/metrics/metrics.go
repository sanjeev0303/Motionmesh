package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Registry is the global Prometheus registry for the application.
	Registry = prometheus.NewRegistry()

	// APIRequestsTotal counts total API requests by endpoint and status code.
	APIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "motionmesh_api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "path", "status"},
	)

	// APIRequestDuration tracks the latency of API requests.
	APIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "motionmesh_api_request_duration_seconds",
			Help:    "API request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	// WorkerJobsActive tracks the number of currently running jobs in the worker pool.
	WorkerJobsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "motionmesh_worker_jobs_active",
			Help: "Number of active jobs in the worker pool",
		},
	)

	// WorkerJobsProcessedTotal counts total completed jobs.
	WorkerJobsProcessedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_worker_jobs_processed_total",
			Help: "Total number of completed jobs",
		},
	)

	// WorkerJobsFailedTotal counts total failed jobs.
	WorkerJobsFailedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_worker_jobs_failed_total",
			Help: "Total number of failed jobs",
		},
	)

	// CleanupJobsProcessedTotal counts total completed cleanup jobs.
	CleanupJobsProcessedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_cleanup_jobs_processed_total",
			Help: "Total number of completed cleanup jobs",
		},
	)

	// CleanupJobsFailedTotal counts total failed cleanup jobs.
	CleanupJobsFailedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_cleanup_jobs_failed_total",
			Help: "Total number of failed cleanup jobs",
		},
	)
)

func init() {
	// Register standard metrics
	Registry.MustRegister(APIRequestsTotal)
	Registry.MustRegister(APIRequestDuration)
	Registry.MustRegister(WorkerJobsActive)
	Registry.MustRegister(WorkerJobsProcessedTotal)
	Registry.MustRegister(WorkerJobsFailedTotal)
	Registry.MustRegister(CleanupJobsProcessedTotal)
	Registry.MustRegister(CleanupJobsFailedTotal)
}

// Handler returns an http.Handler for exposing Prometheus metrics.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		Registry: Registry,
	})
}
