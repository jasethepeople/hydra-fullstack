package dashboard

import (
	"encoding/json"
	"net/http"
	"os"

	"hydra-fullstack/internal/alerts"
	"hydra-fullstack/internal/monitor"
)

// DashboardHTML is read from file at runtime
var dashboardHTML []byte

func init() {
	var err error
	dashboardHTML, err = os.ReadFile("web/dashboard/index.html")
	if err != nil {
		dashboardHTML, err = os.ReadFile("../../web/dashboard/index.html")
	}
}

// RegisterDashboardHandlers sets up dashboard and metrics routes
func RegisterDashboardHandlers(mux *http.ServeMux, hm *monitor.HealthMonitor, am *alerts.AlertManager) {
	mux.HandleFunc("/dashboard/view", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if len(dashboardHTML) == 0 {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(minimalDashboard))
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write(dashboardHTML)
	})

	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		metrics := hm.GetMetrics()
		response := map[string]interface{}{
			"memory_used_mb":  metrics.MemoryUsedMB,
			"memory_total_mb": metrics.MemoryTotalMB,
			"memory_percent":  metrics.MemoryPercent,
			"latency_p50":     metrics.LatencyP50,
			"latency_p95":     metrics.LatencyP95,
			"latency_p99":     metrics.LatencyP99,
			"latency_max":     metrics.LatencyMax,
			"error_rate":      metrics.ErrorRatePercent,
			"error_count":     metrics.ErrorCount,
			"request_count":   metrics.RequestCount,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

const minimalDashboard = `<!DOCTYPE html>
<html><head><title>HYDRA Dashboard</title></head>
<body style="background:#0a0a0f;color:#fff;font-family:sans-serif;padding:2rem;">
<h1>HYDRA Alert Dashboard</h1>
<p>Dashboard HTML not found. Check web/dashboard/index.html exists.</p>
<ul>
<li><a href="/api/metrics">/api/metrics</a> - Live metrics</li>
<li><a href="/alerts/status">/alerts/status</a> - Alert status</li>
<li><a href="/alerts/history">/alerts/history</a> - Alert history</li>
<li><a href="/health">/health</a> - Health check</li>
</ul>
</body></html>`
