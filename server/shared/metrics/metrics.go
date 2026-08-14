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

	// WorkerJobPhaseDuration tracks the latency of different phases in the transcode job.
	WorkerJobPhaseDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "motionmesh_worker_job_phase_duration_seconds",
			Help:    "Duration of individual job phases in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"phase"}, // e.g., "download", "probe", "transcode", "upload"
	)

	// StripeAPICallsTotal counts Stripe API calls.
	StripeAPICallsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_stripe_api_calls_total",
			Help: "Total number of Stripe API calls",
		},
	)

	// AIRequestsTotal counts AI provider requests.
	AIRequestsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_ai_requests_total",
			Help: "Total number of AI requests",
		},
	)

	// LastUsedEnqueueTotal counts how many last used requests were enqueued.
	LastUsedEnqueueTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_last_used_enqueue_total",
			Help: "Total number of last used updates enqueued",
		},
	)

	// LastUsedDroppedTotal counts how many last used requests were dropped.
	LastUsedDroppedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_last_used_dropped_total",
			Help: "Total number of last used updates dropped",
		},
	)

	// LastUsedWorkerLatency tracks the latency of the last used worker batch process.
	LastUsedWorkerLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "motionmesh_last_used_worker_latency_seconds",
			Help:    "Latency of the last used worker in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	// AuthLocalHit counts auth local cache hits.
	AuthLocalHit = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_auth_local_hit_total",
			Help: "Total number of auth local cache hits",
		},
	)

	// AuthRedisHit counts auth redis cache hits.
	AuthRedisHit = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_auth_redis_hit_total",
			Help: "Total number of auth redis cache hits",
		},
	)

	// AuthDBFallback counts auth db fallbacks.
	AuthDBFallback = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_auth_db_fallback_total",
			Help: "Total number of auth db fallbacks",
		},
	)

	// AuthFailure counts auth failures.
	AuthFailure = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "motionmesh_auth_failure_total",
			Help: "Total number of auth failures",
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
	Registry.MustRegister(WorkerJobPhaseDuration)
	Registry.MustRegister(StripeAPICallsTotal)
	Registry.MustRegister(AIRequestsTotal)
	Registry.MustRegister(LastUsedEnqueueTotal)
	Registry.MustRegister(LastUsedDroppedTotal)
	Registry.MustRegister(LastUsedWorkerLatency)
	Registry.MustRegister(AuthLocalHit)
	Registry.MustRegister(AuthRedisHit)
	Registry.MustRegister(AuthDBFallback)
	Registry.MustRegister(AuthFailure)
}

// Handler returns an http.Handler for exposing Prometheus metrics.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		Registry: Registry,
	})
}
