// Package telemetry provides metrics collection.
package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// Resolution metrics
	ResolutionTotal     *prometheus.CounterVec
	ResolutionDuration  *prometheus.HistogramVec
	ResolutionCacheHits prometheus.Counter
	ResolutionCacheMiss prometheus.Counter

	// Registry metrics
	RegistryLookups  *prometheus.CounterVec
	RegistryDuration *prometheus.HistogramVec
	RegistryErrors   *prometheus.CounterVec

	// Verification metrics
	VerificationTotal    *prometheus.CounterVec
	VerificationDuration *prometheus.HistogramVec

	// Cache metrics
	CacheSize      prometheus.Gauge
	CacheEvictions prometheus.Counter

	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPRequestSize     *prometheus.HistogramVec
	HTTPResponseSize    *prometheus.HistogramVec

	// Queue metrics
	QueuePending   prometheus.Gauge
	QueueProcessed prometheus.Counter
	QueueFailed    prometheus.Counter
}

// NewMetrics creates and registers all metrics
func NewMetrics(namespace, subsystem string) *Metrics {
	if namespace == "" {
		namespace = "ans"
	}
	if subsystem == "" {
		subsystem = "resolver"
	}

	m := &Metrics{
		// Resolution metrics
		ResolutionTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "resolution_total",
			Help:      "Total number of resolution requests",
		}, []string{"status", "protocol"}),

		ResolutionDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "resolution_duration_seconds",
			Help:      "Resolution request duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"status", "cache_hit"}),

		ResolutionCacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "resolution_cache_hits_total",
			Help:      "Total number of cache hits",
		}),

		ResolutionCacheMiss: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "resolution_cache_misses_total",
			Help:      "Total number of cache misses",
		}),

		// Registry metrics
		RegistryLookups: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "registry_lookups_total",
			Help:      "Total number of registry lookups",
		}, []string{"registry", "status"}),

		RegistryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "registry_lookup_duration_seconds",
			Help:      "Registry lookup duration in seconds",
			Buckets:   []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"registry"}),

		RegistryErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "registry_errors_total",
			Help:      "Total number of registry errors",
		}, []string{"registry", "error_type"}),

		// Verification metrics
		VerificationTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "verification_total",
			Help:      "Total number of verification checks",
		}, []string{"check", "result"}),

		VerificationDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "verification_duration_seconds",
			Help:      "Verification duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"check"}),

		// Cache metrics
		CacheSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "cache_size",
			Help:      "Current number of items in cache",
		}),

		CacheEvictions: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "cache_evictions_total",
			Help:      "Total number of cache evictions",
		}),

		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),

		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),

		HTTPRequestSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		}, []string{"method", "path"}),

		HTTPResponseSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		}, []string{"method", "path"}),

		// Queue metrics
		QueuePending: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "queue_pending",
			Help:      "Number of pending events in queue",
		}),

		QueueProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "queue_processed_total",
			Help:      "Total number of processed queue events",
		}),

		QueueFailed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "queue_failed_total",
			Help:      "Total number of failed queue events",
		}),
	}

	return m
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordResolution records a resolution request
func (m *Metrics) RecordResolution(status, protocol string, duration time.Duration, cacheHit bool) {
	m.ResolutionTotal.WithLabelValues(status, protocol).Inc()
	m.ResolutionDuration.WithLabelValues(status, strconv.FormatBool(cacheHit)).Observe(duration.Seconds())

	if cacheHit {
		m.ResolutionCacheHits.Inc()
	} else {
		m.ResolutionCacheMiss.Inc()
	}
}

// RecordRegistryLookup records a registry lookup
func (m *Metrics) RecordRegistryLookup(registry, status string, duration time.Duration) {
	m.RegistryLookups.WithLabelValues(registry, status).Inc()
	m.RegistryDuration.WithLabelValues(registry).Observe(duration.Seconds())
}

// RecordVerification records a verification check
func (m *Metrics) RecordVerification(check, result string, duration time.Duration) {
	m.VerificationTotal.WithLabelValues(check, result).Inc()
	m.VerificationDuration.WithLabelValues(check).Observe(duration.Seconds())
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(method, path string, status int, duration time.Duration, requestSize, responseSize int) {
	statusStr := strconv.Itoa(status)
	m.HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	m.HTTPRequestSize.WithLabelValues(method, path).Observe(float64(requestSize))
	m.HTTPResponseSize.WithLabelValues(method, path).Observe(float64(responseSize))
}

// RecordQueueEvent records a queue event processing result
func (m *Metrics) RecordQueueEvent(eventType string, success bool) {
	if success {
		m.QueueProcessed.Inc()
	} else {
		m.QueueFailed.Inc()
	}
}
