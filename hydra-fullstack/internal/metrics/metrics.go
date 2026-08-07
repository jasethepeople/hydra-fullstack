package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

var (
	// Request metrics
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hydra_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "endpoint", "status"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "hydra_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "endpoint"})

	// Alert metrics
	AlertsFired = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hydra_alerts_fired_total",
		Help: "Total alerts fired by type and severity",
	}, []string{"type", "severity"})

	AlertsDispatched = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hydra_alerts_dispatched_total",
		Help: "Total alert notifications dispatched by channel",
	}, []string{"channel", "success"})

	AlertsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hydra_alerts_active",
		Help: "Number of currently active (non-acknowledged) alerts",
	})

	// Render metrics
	RendersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hydra_renders_total",
		Help: "Total render jobs by status",
	}, []string{"status", "complexity"})

	RenderDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "hydra_render_duration_seconds",
		Help:    "Render job duration in seconds",
		Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 25, 50, 100},
	}, []string{"complexity"})

	RenderQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hydra_render_queue_depth",
		Help: "Current render queue depth",
	})

	// System metrics
	MemoryUsagePercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hydra_memory_usage_percent",
		Help: "Current memory usage percentage",
	})

	LatencyP95 = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hydra_latency_p95_seconds",
		Help: "95th percentile latency in seconds",
	})

	ErrorRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hydra_error_rate_percent",
		Help: "Current error rate percentage",
	})

	// Worker metrics
	WorkersActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hydra_workers_active",
		Help: "Number of active workers",
	})

	WorkersBusy = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hydra_workers_busy",
		Help: "Number of busy workers",
	})

	// Circuit breaker metrics
	CircuitBreakerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hydra_circuit_breaker_state",
		Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
	}, []string{"service"})

	// Memorial metrics
	MemorialUploadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hydra_memorial_uploads_total",
		Help: "Total memorial uploads by type",
	}, []string{"media_type"})

	MemorialWebSocketConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hydra_memorial_websocket_connections",
		Help: "Current WebSocket connections",
	})

	MemorialCanvasPixels = promauto.NewCounter(prometheus.CounterOpts{
		Name: "hydra_memorial_canvas_pixels_total",
		Help: "Total canvas pixels placed",
	})
)

// RecordRequest records HTTP request metrics
func RecordRequest(method, endpoint string, status int, duration time.Duration) {
	RequestsTotal.WithLabelValues(method, endpoint, http.StatusText(status)).Inc()
	RequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordAlert records alert firing
func RecordAlert(alertType, severity string) {
	AlertsFired.WithLabelValues(alertType, severity).Inc()
	AlertsActive.Inc()
}

// RecordAlertDispatch records notification dispatch
func RecordAlertDispatch(channel string, success bool) {
	successStr := "false"
	if success {
		successStr = "true"
	}
	AlertsDispatched.WithLabelValues(channel, successStr).Inc()
}

// RecordAlertAcknowledged decrements active alerts
func RecordAlertAcknowledged() {
	AlertsActive.Dec()
}

// RecordRender records render job metrics
func RecordRender(status, complexity string, duration time.Duration) {
	RendersTotal.WithLabelValues(status, complexity).Inc()
	RenderDuration.WithLabelValues(complexity).Observe(duration.Seconds())
}

// UpdateQueueDepth updates queue depth gauge
func UpdateQueueDepth(depth int64) {
	RenderQueueDepth.Set(float64(depth))
}

// UpdateSystemMetrics updates system health gauges
func UpdateSystemMetrics(memoryPercent, latencyP95, errorRate float64) {
	MemoryUsagePercent.Set(memoryPercent)
	LatencyP95.Set(latencyP95)
	ErrorRate.Set(errorRate)
}

// UpdateWorkers updates worker pool gauges
func UpdateWorkers(active, busy int) {
	WorkersActive.Set(float64(active))
	WorkersBusy.Set(float64(busy))
}

// UpdateCircuitBreaker updates circuit breaker state
func UpdateCircuitBreaker(service string, state int) {
	CircuitBreakerState.WithLabelValues(service).Set(float64(state))
}

// RecordMemorialUpload records memorial upload
func RecordMemorialUpload(mediaType string) {
	MemorialUploadsTotal.WithLabelValues(mediaType).Inc()
}

// UpdateWebSocketConnections updates WebSocket gauge
func UpdateWebSocketConnections(count int) {
	MemorialWebSocketConnections.Set(float64(count))
}

// RecordCanvasPixel records canvas pixel placement
func RecordCanvasPixel() {
	MemorialCanvasPixels.Inc()
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// StartServer starts the metrics server on the given port
func StartServer(port, path string) {
	mux := http.NewServeMux()
	mux.Handle(path, Handler())

	addr := ":" + port
	log.Info().Str("addr", addr).Str("path", path).Msg("metrics server starting")

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Error().Err(err).Str("addr", addr).Msg("metrics server failed")
		}
	}()
}
