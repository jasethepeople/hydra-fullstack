package monitor

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"hydra-fullstack/internal/alerts"
	"hydra-fullstack/internal/config"
	"hydra-fullstack/internal/logging"
	"hydra-fullstack/internal/metrics"
	"hydra-fullstack/internal/natsbus"
	"hydra-fullstack/internal/storage"
)

// Enhanced HealthMonitor with configurable thresholds
type HealthMonitor struct {
	mu           sync.RWMutex
	metrics      Metrics
	alertManager *alerts.AlertManager
	cfg          *config.AlertConfig
	logger       *logging.Logger
	store        *storage.Store
	natsBus      *natsbus.NATSBus
	latencies    []float64
	maxSamples   int
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// Metrics holds current system metrics
type Metrics struct {
	MemoryUsedMB     float64
	MemoryTotalMB    float64
	MemoryPercent    float64
	LatencyP50       float64
	LatencyP95       float64
	LatencyP99       float64
	LatencyMax       float64
	ErrorRatePercent float64
	RequestCount     int64
	ErrorCount       int64
	Timestamp        time.Time
}

// NewHealthMonitor creates an enhanced health monitor
func NewHealthMonitor(am *alerts.AlertManager, cfg *config.AlertConfig, logger *logging.Logger,
	store *storage.Store, natsBus *natsbus.NATSBus) *HealthMonitor {
	return &HealthMonitor{
		alertManager: am,
		cfg:          cfg,
		logger:       logger,
		store:        store,
		natsBus:      natsBus,
		latencies:    make([]float64, 0, 1000),
		maxSamples:   1000,
		stopCh:       make(chan struct{}),
	}
}

// Start begins the monitoring loop
func (hm *HealthMonitor) Start() {
	hm.wg.Add(1)
	go hm.monitorLoop()
}

// Stop halts the monitoring loop
func (hm *HealthMonitor) Stop() {
	close(hm.stopCh)
	hm.wg.Wait()
}

// monitorLoop runs every 5 seconds checking metrics
func (hm *HealthMonitor) monitorLoop() {
	defer hm.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.checkMemory()
			hm.checkLatency()
			hm.checkErrorRate()
			hm.updateMetrics()
		case <-hm.stopCh:
			return
		}
	}
}

// checkMemory monitors memory usage and fires alerts
func (hm *HealthMonitor) checkMemory() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	usedMB := float64(m.Sys) / 1024 / 1024
	totalMB := float64(m.Sys+m.HeapIdle) / 1024 / 1024
	if totalMB == 0 {
		totalMB = 512
	}

	percent := (usedMB / totalMB) * 100

	hm.mu.Lock()
	hm.metrics.MemoryUsedMB = usedMB
	hm.metrics.MemoryTotalMB = totalMB
	hm.metrics.MemoryPercent = percent
	hm.metrics.Timestamp = time.Now().UTC()
	hm.mu.Unlock()

	if percent >= hm.cfg.MemoryCritical {
		alert := hm.alertManager.FireAlert(
			"High Memory Usage",
			alerts.SeverityCritical,
			alerts.CategorySystem,
			fmt.Sprintf("Memory at %.0fKB (%.0f%%)", usedMB*1024, percent),
			percent,
			hm.cfg.MemoryCritical,
			"%",
		)
		hm.logSystemEvent("memory_critical", alert.ID, percent)
	} else if percent >= hm.cfg.MemoryWarning {
		alert := hm.alertManager.FireAlert(
			"High Memory Usage",
			alerts.SeverityWarning,
			alerts.CategorySystem,
			fmt.Sprintf("Memory at %.0fKB (%.0f%%)", usedMB*1024, percent),
			percent,
			hm.cfg.MemoryWarning,
			"%",
		)
		hm.logSystemEvent("memory_warning", alert.ID, percent)
	}
}

// RecordLatency tracks a request latency
func (hm *HealthMonitor) RecordLatency(duration time.Duration) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	seconds := duration.Seconds()
	hm.latencies = append(hm.latencies, seconds)

	if len(hm.latencies) > hm.maxSamples {
		hm.latencies = hm.latencies[len(hm.latencies)-hm.maxSamples:]
	}

	hm.metrics.RequestCount++
}

// RecordError increments error count
func (hm *HealthMonitor) RecordError() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.metrics.ErrorCount++
}

// checkLatency calculates percentiles and fires alerts
func (hm *HealthMonitor) checkLatency() {
	hm.mu.Lock()
	latencies := make([]float64, len(hm.latencies))
	copy(latencies, hm.latencies)
	hm.mu.Unlock()

	if len(latencies) == 0 {
		return
	}

	p50, p95, p99, max := calculatePercentiles(latencies)

	hm.mu.Lock()
	hm.metrics.LatencyP50 = p50
	hm.metrics.LatencyP95 = p95
	hm.metrics.LatencyP99 = p99
	hm.metrics.LatencyMax = max
	hm.mu.Unlock()

	if p95 >= hm.cfg.LatencyCritical {
		alert := hm.alertManager.FireAlert(
			"High Render Latency",
			alerts.SeverityCritical,
			alerts.CategoryPerformance,
			fmt.Sprintf("p95 latency is %.2fs (threshold: %.0fs)", p95, hm.cfg.LatencyCritical),
			p95,
			hm.cfg.LatencyCritical,
			"s",
		)
		hm.logSystemEvent("latency_critical", alert.ID, p95)
	} else if p95 >= hm.cfg.LatencyWarning {
		alert := hm.alertManager.FireAlert(
			"High Render Latency",
			alerts.SeverityWarning,
			alerts.CategoryPerformance,
			fmt.Sprintf("p95 latency is %.2fs (threshold: %.0fs)", p95, hm.cfg.LatencyWarning),
			p95,
			hm.cfg.LatencyWarning,
			"s",
		)
		hm.logSystemEvent("latency_warning", alert.ID, p95)
	}
}

// checkErrorRate monitors error percentage
func (hm *HealthMonitor) checkErrorRate() {
	hm.mu.RLock()
	reqCount := hm.metrics.RequestCount
	errCount := hm.metrics.ErrorCount
	hm.mu.RUnlock()

	if reqCount == 0 {
		return
	}

	errorRate := (float64(errCount) / float64(reqCount)) * 100

	hm.mu.Lock()
	hm.metrics.ErrorRatePercent = errorRate
	hm.mu.Unlock()

	if errorRate >= hm.cfg.ErrorCritical {
		alert := hm.alertManager.FireAlert(
			"High Error Rate",
			alerts.SeverityCritical,
			alerts.CategoryError,
			fmt.Sprintf("Error rate at %.1f%% (threshold: %.0f%%)", errorRate, hm.cfg.ErrorCritical),
			errorRate,
			hm.cfg.ErrorCritical,
			"%",
		)
		hm.logSystemEvent("error_critical", alert.ID, errorRate)
	} else if errorRate >= hm.cfg.ErrorWarning {
		alert := hm.alertManager.FireAlert(
			"High Error Rate",
			alerts.SeverityWarning,
			alerts.CategoryError,
			fmt.Sprintf("Error rate at %.1f%% (threshold: %.0f%%)", errorRate, hm.cfg.ErrorWarning),
			errorRate,
			hm.cfg.ErrorWarning,
			"%",
		)
		hm.logSystemEvent("error_warning", alert.ID, errorRate)
	}
}

// updateMetrics updates Prometheus gauges
func (hm *HealthMonitor) updateMetrics() {
	hm.mu.RLock()
	m := hm.metrics
	hm.mu.RUnlock()

	metrics.UpdateSystemMetrics(m.MemoryPercent, m.LatencyP95, m.ErrorRatePercent)
}

// logSystemEvent persists system events to database
func (hm *HealthMonitor) logSystemEvent(eventType, alertID string, value float64) {
	if hm.store == nil {
		return
	}

	metadata := fmt.Sprintf(`{"alert_id":"%s","value":%.2f}`, alertID, value)
	severity := "warning"
	if eventType == "memory_critical" || eventType == "latency_critical" || eventType == "error_critical" {
		severity = "critical"
	}

	go hm.store.LogSystemEvent(eventType, severity, fmt.Sprintf("System threshold exceeded: %s", eventType), metadata)
}

// GetMetrics returns current metrics snapshot
func (hm *HealthMonitor) GetMetrics() Metrics {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.metrics
}

// calculatePercentiles computes p50, p95, p99, max
func calculatePercentiles(data []float64) (p50, p95, p99, max float64) {
	if len(data) == 0 {
		return 0, 0, 0, 0
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)

	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}

	max = sorted[len(sorted)-1]
	p50 = percentile(sorted, 0.50)
	p95 = percentile(sorted, 0.95)
	p99 = percentile(sorted, 0.99)

	return
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}

	index := p * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[lower]
	}

	fraction := index - float64(lower)
	return sorted[lower] + fraction*(sorted[upper]-sorted[lower])
}
