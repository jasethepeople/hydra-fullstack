package logging

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger wraps zerolog with alert correlation and context
type Logger struct {
	zerolog.Logger
	correlationID string
	service       string
}

// New creates a new structured logger
func New(service string, production bool) *Logger {
	var output io.Writer = os.Stdout

	if production {
		// JSON output for production (machine parseable)
		zerolog.TimeFieldFormat = time.RFC3339Nano
	} else {
		// Pretty console output for development
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	logger := zerolog.New(output).
		With().
		Timestamp().
		Str("service", service).
		Logger()

	// Set global level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if os.Getenv("DEBUG") == "true" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	return &Logger{
		Logger:  logger,
		service: service,
	}
}

// WithCorrelation adds a correlation ID for tracing requests
func (l *Logger) WithCorrelation(id string) *Logger {
	return &Logger{
		Logger:        l.Logger.With().Str("correlation_id", id).Logger(),
		correlationID: id,
		service:       l.service,
	}
}

// WithAlert adds alert context to logs
func (l *Logger) WithAlert(alertID, alertType, severity string) *Logger {
	return &Logger{
		Logger: l.Logger.With().
			Str("alert_id", alertID).
			Str("alert_type", alertType).
			Str("severity", severity).
			Logger(),
		correlationID: l.correlationID,
		service:       l.service,
	}
}

// AlertFired logs when an alert is triggered
func (l *Logger) AlertFired(alertID, alertType, severity, message string, value, threshold float64) {
	l.WithAlert(alertID, alertType, severity).
		Warn().
		Str("event", "alert_fired").
		Str("message", message).
		Float64("value", value).
		Float64("threshold", threshold).
		Msg("alert triggered")
}

// AlertDispatched logs notification delivery
func (l *Logger) AlertDispatched(alertID, channel string, success bool, latencyMs int64) {
	event := l.WithAlert(alertID, "", "").Info().
		Str("event", "alert_dispatched").
		Str("channel", channel).
		Bool("success", success).
		Int64("latency_ms", latencyMs)

	if !success {
		event = event.Str("action", "queued_for_retry")
	}

	event.Msg("notification dispatched")
}

// AlertAcknowledged logs acknowledgment
func (l *Logger) AlertAcknowledged(alertID, ackedBy string) {
	l.WithAlert(alertID, "", "").
		Info().
		Str("event", "alert_acknowledged").
		Str("acked_by", ackedBy).
		Msg("alert acknowledged by user")
}

// RenderStarted logs render job start
func (l *Logger) RenderStarted(jobID string, width, height int, complexity string) {
	l.Info().
		Str("event", "render_started").
		Str("job_id", jobID).
		Int("width", width).
		Int("height", height).
		Str("complexity", complexity).
		Msg("render job started")
}

// RenderCompleted logs render completion
func (l *Logger) RenderCompleted(jobID string, duration time.Duration, success bool) {
	event := l.Info().
		Str("event", "render_completed").
		Str("job_id", jobID).
		Dur("duration", duration).
		Bool("success", success)

	if !success {
		event = event.Str("action", "queued_for_retry")
	}

	event.Msg("render job completed")
}

// WorkerEvent logs worker pool events
func (l *Logger) WorkerEvent(event string, workerID int, queueDepth int64) {
	l.Debug().
		Str("event", event).
		Int("worker_id", workerID).
		Int64("queue_depth", queueDepth).
		Msg("worker pool event")
}

// CircuitBreakerState logs circuit breaker transitions
func (l *Logger) CircuitBreakerState(service string, state string, failures int) {
	l.Warn().
		Str("event", "circuit_breaker_state_change").
		Str("service", service).
		Str("state", state).
		Int("consecutive_failures", failures).
		Msg("circuit breaker state changed")
}

// NATSEvent logs NATS message events
func (l *Logger) NATSEvent(event, subject string, msgSize int) {
	l.Debug().
		Str("event", event).
		Str("nats_subject", subject).
		Int("msg_size_bytes", msgSize).
		Msg("nats event")
}

// MemorialEvent logs memorial site events
func (l *Logger) MemorialEvent(event, userID, action string) {
	l.Info().
		Str("event", event).
		Str("user_id", userID).
		Str("action", action).
		Msg("memorial site event")
}

// FatalError logs fatal errors with full context
func (l *Logger) FatalError(err error, context map[string]interface{}) {
	event := l.Error().Err(err).Str("event", "fatal_error")
	for k, v := range context {
		event = event.Interface(k, v)
	}
	event.Msg("fatal error occurred")
}

// Global convenience functions
func Info() *zerolog.Event  { return log.Info() }
func Warn() *zerolog.Event  { return log.Warn() }
func Error() *zerolog.Event { return log.Error() }
func Debug() *zerolog.Event { return log.Debug() }
func Fatal() *zerolog.Event { return log.Fatal() }
