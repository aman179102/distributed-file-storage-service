package metrics

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsManager struct {
	registry *prometheus.Registry

	requestCount    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
	activeRequests  prometheus.Gauge
	errorCount      *prometheus.CounterVec

	storageOperations *prometheus.CounterVec
	storageBytes      *prometheus.GaugeVec
	storageDuration   *prometheus.HistogramVec

	activeConnections prometheus.Gauge
	goroutineCount    prometheus.Gauge
	openFileHandles   prometheus.Gauge

	objectCount   *prometheus.GaugeVec
	bucketCount   prometheus.Gauge
	chunkCount    prometheus.Gauge
	uploadCount   prometheus.Gauge
	replicationFactor prometheus.Gauge
	erasureCodingRatio prometheus.Gauge
}

func NewMetricsManager() *MetricsManager {
	mm := &MetricsManager{
		registry: prometheus.NewRegistry(),
		requestCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		requestSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_size_bytes",
				Help:    "HTTP request size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method"},
		),
		responseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method", "path"},
		),
		activeRequests: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_active",
				Help: "Number of active HTTP requests",
			},
		),
		errorCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_errors_total",
				Help: "Total number of HTTP errors by status code",
			},
			[]string{"status", "method"},
		),
		storageOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_operations_total",
				Help: "Total number of storage operations",
			},
			[]string{"operation", "status"},
		),
		storageBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_bytes_total",
				Help: "Total bytes stored by type",
			},
			[]string{"type"},
		),
		storageDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "storage_operation_duration_seconds",
				Help:    "Duration of storage operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		activeConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_active",
				Help: "Number of active database connections",
			},
		),
		goroutineCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "go_goroutines_total",
				Help: "Number of goroutines",
			},
		),
		openFileHandles: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "storage_open_files",
				Help: "Number of open file handles",
			},
		),
		objectCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_objects_total",
				Help: "Total number of stored objects",
			},
			[]string{"status"},
		),
		bucketCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "storage_buckets_total",
				Help: "Total number of buckets",
			},
		),
		chunkCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "storage_chunks_total",
				Help: "Total number of file chunks",
			},
		),
		uploadCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "storage_multipart_uploads_total",
				Help: "Total number of active multipart uploads",
			},
		),
		replicationFactor: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "storage_replication_factor",
				Help: "Current replication factor",
			},
		),
		erasureCodingRatio: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "storage_erasure_coding_ratio",
				Help: "Ratio of parity shards to total shards",
			},
		),
	}

	mm.registry.MustRegister(
		mm.requestCount, mm.requestDuration, mm.requestSize, mm.responseSize,
		mm.activeRequests, mm.errorCount,
		mm.storageOperations, mm.storageBytes, mm.storageDuration,
		mm.activeConnections, mm.goroutineCount, mm.openFileHandles,
		mm.objectCount, mm.bucketCount, mm.chunkCount, mm.uploadCount,
		mm.replicationFactor, mm.erasureCodingRatio,
	)

	return mm
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

func (mm *MetricsManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		mm.activeRequests.Inc()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)

		mm.requestCount.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		mm.requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
		mm.responseSize.WithLabelValues(r.Method, r.URL.Path).Observe(float64(rw.bytes))

		if rw.statusCode >= 400 {
			mm.errorCount.WithLabelValues(status, r.Method).Inc()
		}

		mm.activeRequests.Dec()
	})
}

func (mm *MetricsManager) RecordStorageOp(operation string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	mm.storageOperations.WithLabelValues(operation, status).Inc()
	mm.storageDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

func (mm *MetricsManager) SetStorageBytes(typeName string, bytes int64) {
	mm.storageBytes.WithLabelValues(typeName).Set(float64(bytes))
}

func (mm *MetricsManager) SetObjectCount(status string, count int64) {
	mm.objectCount.WithLabelValues(status).Set(float64(count))
}

func (mm *MetricsManager) SetBucketCount(count int64) {
	mm.bucketCount.Set(float64(count))
}

func (mm *MetricsManager) SetChunkCount(count int64) {
	mm.chunkCount.Set(float64(count))
}

func (mm *MetricsManager) SetActiveUploads(count int64) {
	mm.uploadCount.Set(float64(count))
}

func (mm *MetricsManager) SetReplicationFactor(factor int) {
	mm.replicationFactor.Set(float64(factor))
}

func (mm *MetricsManager) SetErasureCodingRatio(ratio float64) {
	mm.erasureCodingRatio.Set(float64(ratio))
}

func (mm *MetricsManager) Handler() http.Handler {
	return promhttp.HandlerFor(mm.registry, promhttp.HandlerOpts{
		ErrorLog:      slog.NewLogLogger(slog.NewJSONHandler(nil, nil), slog.WarnLevel),
		ErrorHandling: promhttp.ContinueOnError,
	})
}

func (mm *MetricsManager) UpdateSystemMetrics(activeConns, goroutines, openFiles int64) {
	mm.activeConnections.Set(float64(activeConns))
	mm.goroutineCount.Set(float64(goroutines))
	mm.openFileHandles.Set(float64(openFiles))
}
