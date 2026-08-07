package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"

	"hydra-fullstack/internal/config"
	"hydra-fullstack/internal/logging"
	"hydra-fullstack/internal/metrics"
	"hydra-fullstack/internal/natsbus"
	"hydra-fullstack/internal/storage"
)

// Enhanced AlertManager with routing, persistence, and multiplexing
type AlertManager struct {
	mu              sync.RWMutex
	alerts          []Alert
	activeAlerts    map[string]Alert
	cfg             *config.AlertConfig
	logger          *logging.Logger
	store           *storage.Store
	natsBus         *natsbus.NATSBus
	httpClient      *retryablehttp.Client
	historyLimit    int
	routingRules    []RoutingRule
}

// RoutingRule defines where alerts go based on severity/type
type RoutingRule struct {
	Severity  Severity
	Category  Category
	Channels  []string
}

// NewAlertManager creates an enhanced alert manager
func NewAlertManager(cfg *config.AlertConfig, logger *logging.Logger, store *storage.Store, natsBus *natsbus.NATSBus) *AlertManager {
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 1 * time.Second
	client.RetryWaitMax = 5 * time.Second

	am := &AlertManager{
		alerts:       make([]Alert, 0),
		activeAlerts: make(map[string]Alert),
		cfg:          cfg,
		logger:       logger,
		store:        store,
		natsBus:      natsBus,
		httpClient:   client,
		historyLimit: 1000,
		routingRules: defaultRoutingRules(),
	}

	return am
}

func defaultRoutingRules() []RoutingRule {
	return []RoutingRule{
		{SeverityCritical, CategorySystem, []string{"discord", "pagerduty", "smtp"}},
		{SeverityCritical, CategoryPerformance, []string{"discord", "slack"}},
		{SeverityCritical, CategoryError, []string{"discord", "pagerduty"}},
		{SeverityWarning, CategorySystem, []string{"discord"}},
		{SeverityWarning, CategoryPerformance, []string{"discord", "slack"}},
		{SeverityInfo, CategorySystem, []string{"discord"}},
	}
}

// FireAlert creates and dispatches a new alert with full routing
func (am *AlertManager) FireAlert(alertType string, severity Severity, category Category,
	message string, value, threshold float64, unit string) *Alert {

	am.mu.Lock()
	defer am.mu.Unlock()

	alert := Alert{
		ID:        fmt.Sprintf("alt-%d-%s", time.Now().UnixNano(), alertType),
		Type:      alertType,
		Severity:  severity,
		Category:  category,
		Message:   message,
		Value:     value,
		Threshold: threshold,
		Unit:      unit,
		Timestamp: time.Now().UTC(),
	}

	// Add to in-memory history
	am.alerts = append([]Alert{alert}, am.alerts...)
	if len(am.alerts) > am.historyLimit {
		am.alerts = am.alerts[:am.historyLimit]
	}

	// Track active alert
	if severity != SeverityResolved {
		am.activeAlerts[alert.ID] = alert
		metrics.RecordAlert(alertType, string(severity))
	}

	// Persist to database
	if am.store != nil {
		go am.store.SaveAlert(alert.ID, alertType, string(severity), string(category),
			message, value, threshold, unit, alert.Timestamp)
	}

	// Log with correlation
	am.logger.AlertFired(alert.ID, alertType, string(severity), message, value, threshold)

	// Dispatch to NATS
	if am.natsBus != nil {
		go am.natsBus.PublishAlert(alert.ID, alertType, string(severity), message, value, threshold)
	}

	// Route to configured channels
	go am.routeAndDispatch(alert)

	return &alert
}

// routeAndDispatch sends alert to appropriate channels based on routing rules
func (am *AlertManager) routeAndDispatch(alert Alert) {
	channels := am.getChannelsForAlert(alert)

	for _, channel := range channels {
		start := time.Now()
		success := false

		switch channel {
		case "discord":
			if am.cfg.DiscordWebhook != "" {
				success = am.sendDiscord(alert)
			}
		case "slack":
			if am.cfg.SlackWebhook != "" {
				success = am.sendSlack(alert)
			}
		case "smtp":
			if am.cfg.SMTPHost != "" {
				success = am.sendEmail(alert)
			}
		case "pagerduty":
			if am.cfg.PagerDutyKey != "" {
				success = am.sendPagerDuty(alert)
			}
		}

		latency := time.Since(start).Milliseconds()
		am.logger.AlertDispatched(alert.ID, channel, success, latency)
		metrics.RecordAlertDispatch(channel, success)

		// Persist notification attempt
		if am.store != nil {
			go am.store.SaveNotification(alert.ID, channel, success, 0, "", latency)
		}
	}
}

// getChannelsForAlert determines which channels to use
func (am *AlertManager) getChannelsForAlert(alert Alert) []string {
	// Check routing rules first
	for _, rule := range am.routingRules {
		if rule.Severity == alert.Severity && rule.Category == alert.Category {
			return rule.Channels
		}
	}

	// Default: send to all configured channels
	var channels []string
	if am.cfg.DiscordWebhook != "" {
		channels = append(channels, "discord")
	}
	if am.cfg.SlackWebhook != "" {
		channels = append(channels, "slack")
	}
	if am.cfg.SMTPHost != "" && alert.Severity == SeverityCritical {
		channels = append(channels, "smtp")
	}
	if am.cfg.PagerDutyKey != "" && alert.Severity == SeverityCritical {
		channels = append(channels, "pagerduty")
	}
	return channels
}

// sendDiscord sends rich embed to Discord
func (am *AlertManager) sendDiscord(alert Alert) bool {
	color := 0x3498db
	switch alert.Severity {
	case SeverityWarning:
		color = 0xf39c12
	case SeverityCritical:
		color = 0xe74c3c
	case SeverityResolved:
		color = 0x2ecc71
	}

	emoji := "🚨"
	if alert.Severity == SeverityResolved {
		emoji = "✅"
	}

	embed := map[string]interface{}{
		"title":       fmt.Sprintf("%s %s", emoji, alert.Message),
		"description": alert.Type,
		"color":       color,
		"fields": []map[string]string{
			{"name": "Severity", "value": string(alert.Severity), "inline": "true"},
			{"name": "Category", "value": string(alert.Category), "inline": "true"},
			{"name": "Time", "value": alert.Timestamp.Format(time.RFC3339), "inline": "false"},
		},
		"footer": map[string]string{"text": "HYDRA Alert System"},
		"timestamp": alert.Timestamp.Format(time.RFC3339),
	}

	if alert.Value != 0 || alert.Threshold != 0 {
		embed["fields"] = append(embed["fields"].([]map[string]string), map[string]string{
			"name":   "Value / Threshold",
			"value":  fmt.Sprintf("%.2f%s / %.2f%s", alert.Value, alert.Unit, alert.Threshold, alert.Unit),
			"inline": "true",
		})
	}

	payload := map[string]interface{}{"embeds": []map[string]interface{}{embed}}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := retryablehttp.NewRequest("POST", am.cfg.DiscordWebhook, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Str("alert_id", alert.ID).Msg("discord dispatch failed")
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200 || resp.StatusCode == 204
}

// sendSlack sends message to Slack
func (am *AlertManager) sendSlack(alert Alert) bool {
	color := "#3498db"
	switch alert.Severity {
	case SeverityWarning:
		color = "#f39c12"
	case SeverityCritical:
		color = "#e74c3c"
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{{
			"color": color,
			"title": fmt.Sprintf("[%s] %s", alert.Severity, alert.Type),
			"text":  alert.Message,
			"fields": []map[string]string{
				{"title": "Category", "value": string(alert.Category), "short": "true"},
				{"title": "Time", "value": alert.Timestamp.Format(time.RFC3339), "short": "true"},
			},
		}},
	}

	jsonPayload, _ := json.Marshal(payload)
	req, _ := retryablehttp.NewRequest("POST", am.cfg.SlackWebhook, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// sendEmail sends SMTP alert (placeholder)
func (am *AlertManager) sendEmail(alert Alert) bool {
	// SMTP implementation would go here using net/smtp or gomail
	log.Info().Str("alert_id", alert.ID).Msg("email alert would be sent")
	return true
}

// sendPagerDuty sends PagerDuty event
func (am *AlertManager) sendPagerDuty(alert Alert) bool {
	severity := "warning"
	if alert.Severity == SeverityCritical {
		severity = "critical"
	}

	payload := map[string]interface{}{
		"routing_key":  am.cfg.PagerDutyKey,
		"event_action":   "trigger",
		"dedup_key":    alert.ID,
		"payload": map[string]interface{}{
			"summary":  fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Type, alert.Message),
			"severity": severity,
			"source":   "hydra-alert-system",
			"custom_details": map[string]interface{}{
				"value":     alert.Value,
				"threshold": alert.Threshold,
				"unit":      alert.Unit,
			},
		},
	}

	jsonPayload, _ := json.Marshal(payload)
	req, _ := retryablehttp.NewRequest("POST", "https://events.pagerduty.com/v2/enqueue", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 202
}

// GetActiveAlerts returns all non-acknowledged active alerts
func (am *AlertManager) GetActiveAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var active []Alert
	for _, alert := range am.activeAlerts {
		if !alert.Acknowledged {
			active = append(active, alert)
		}
	}

	// Sort by severity (Critical first) then timestamp
	for i := 0; i < len(active); i++ {
		for j := i + 1; j < len(active); j++ {
			if severityWeight(active[j].Severity) > severityWeight(active[i].Severity) {
				active[i], active[j] = active[j], active[i]
			}
		}
	}
	return active
}

// GetAlertHistory returns paginated alert history
func (am *AlertManager) GetAlertHistory(limit, offset int) []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if offset >= len(am.alerts) {
		return []Alert{}
	}
	end := offset + limit
	if end > len(am.alerts) {
		end = len(am.alerts)
	}
	return am.alerts[offset:end]
}

// GetTotalCount returns total alert count
func (am *AlertManager) GetTotalCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.alerts)
}

// AcknowledgeAlert marks an alert as acknowledged
func (am *AlertManager) AcknowledgeAlert(alertID, ackedBy string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert, exists := am.activeAlerts[alertID]; exists {
		alert.Acknowledged = true
		alert.AckedBy = ackedBy
		alert.AckedAt = time.Now().UTC()
		am.activeAlerts[alertID] = alert

		// Update in history
		for i := range am.alerts {
			if am.alerts[i].ID == alertID {
				am.alerts[i] = alert
				break
			}
		}

		// Persist acknowledgment
		if am.store != nil {
			go am.store.AcknowledgeAlert(alertID, ackedBy)
		}

		am.logger.AlertAcknowledged(alertID, ackedBy)
		metrics.RecordAlertAcknowledged()
		return true
	}
	return false
}

// GetStatus returns current alert system status
func (am *AlertManager) GetStatus() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return map[string]interface{}{
		"discord_configured": am.cfg.DiscordWebhook != "",
		"slack_configured":   am.cfg.SlackWebhook != "",
		"smtp_configured":    am.cfg.SMTPHost != "",
		"pagerduty_configured": am.cfg.PagerDutyKey != "",
		"active_alerts":      len(am.GetActiveAlerts()),
		"total_history":      len(am.alerts),
		"routing_rules":      len(am.routingRules),
	}
}

// ResolveAlert removes an alert from active alerts
func (am *AlertManager) ResolveAlert(alertID string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.activeAlerts, alertID)
}

func severityWeight(s Severity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}
