package alerts

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"hydra-fullstack/internal/metrics"
)

// RegisterHandlers sets up all alert HTTP routes with metrics
func (am *AlertManager) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/alerts/status", am.withMetrics("alerts/status", am.handleStatus))
	mux.HandleFunc("/alerts/history", am.withMetrics("alerts/history", am.handleHistory))
	mux.HandleFunc("/alerts/acknowledge", am.withMetrics("alerts/acknowledge", am.handleAcknowledge))
	mux.HandleFunc("/alerts/active", am.withMetrics("alerts/active", am.handleActive))
	mux.HandleFunc("/health", am.withMetrics("health", am.handleHealth))
	mux.HandleFunc("/api/v1/alerts/fire", am.withMetrics("api/v1/alerts/fire", am.handleFireAlert))
}

// withMetrics wraps handlers with request metrics
func (am *AlertManager) withMetrics(endpoint string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		handler(wrapped, r)

		metrics.RecordRequest(r.Method, endpoint, wrapped.statusCode, time.Since(start))
	}
}

// responseWriter captures status code for metrics
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// handleHealth returns basic health status
func (am *AlertManager) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
		"service":   "hydra-alert-system",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStatus returns alert system configuration status
func (am *AlertManager) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := am.GetStatus()
	status["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleHistory returns paginated alert history
func (am *AlertManager) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	history := am.GetAlertHistory(limit, offset)

	response := map[string]interface{}{
		"alerts": history,
		"count":  len(history),
		"limit":  limit,
		"offset": offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleAcknowledge acknowledges an alert by ID
func (am *AlertManager) handleAcknowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AlertID string `json:"alert_id"`
		AckedBy string `json:"acked_by,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.AlertID == "" {
		http.Error(w, "alert_id is required", http.StatusBadRequest)
		return
	}

	if req.AckedBy == "" {
		req.AckedBy = "anonymous"
	}

	success := am.AcknowledgeAlert(req.AlertID, req.AckedBy)

	if !success {
		http.Error(w, "Alert not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"success":   true,
		"alert_id":  req.AlertID,
		"acked_by":  req.AckedBy,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleActive returns currently active (non-acknowledged) alerts
func (am *AlertManager) handleActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	active := am.GetActiveAlerts()

	response := map[string]interface{}{
		"alerts":    active,
		"count":     len(active),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleFireAlert allows manual alert firing via API
func (am *AlertManager) handleFireAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AlertType string  `json:"alert_type"`
		Severity  string  `json:"severity"`
		Category  string  `json:"category"`
		Message   string  `json:"message"`
		Value     float64 `json:"value"`
		Threshold float64 `json:"threshold"`
		Unit      string  `json:"unit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.AlertType == "" || req.Message == "" {
		http.Error(w, "alert_type and message are required", http.StatusBadRequest)
		return
	}

	severity := Severity(req.Severity)
	if severity == "" {
		severity = SeverityWarning
	}

	category := Category(req.Category)
	if category == "" {
		category = CategorySystem
	}

	alert := am.FireAlert(req.AlertType, severity, category, req.Message, req.Value, req.Threshold, req.Unit)

	response := map[string]interface{}{
		"success":   true,
		"alert_id":  alert.ID,
		"severity":  alert.Severity,
		"channels":  am.GetStatus()["routing_rules"],
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
