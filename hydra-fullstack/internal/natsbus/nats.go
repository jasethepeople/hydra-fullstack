package natsbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"hydra-fullstack/internal/config"
	"hydra-fullstack/internal/logging"
)

// Event represents a system event for NATS
type Event struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	Timestamp   time.Time              `json:"timestamp"`
	Payload     map[string]interface{} `json:"payload"`
	Correlation string                 `json:"correlation_id,omitempty"`
}

// NATSBus handles event streaming via NATS/JetStream
type NATSBus struct {
	conn       *nats.Conn
	js         nats.JetStreamContext
	streamName string
	logger     *logging.Logger
	ctx        context.Context
}

// NewNATSBus creates a new NATS event bus
func NewNATSBus(cfg *config.NATSConfig, logger *logging.Logger) (*NATSBus, error) {
	if !cfg.Enabled {
		log.Info().Msg("nats disabled, using in-memory event bus")
		return &NATSBus{logger: logger}, nil
	}

	conn, err := nats.Connect(cfg.URL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Warn().Err(err).Msg("nats disconnected")
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info().Msg("nats reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	bus := &NATSBus{
		conn:       conn,
		js:         js,
		streamName: cfg.StreamName,
		logger:     logger,
		ctx:        context.Background(),
	}

	if err := bus.ensureStream(); err != nil {
		conn.Close()
		return nil, err
	}

	log.Info().Str("url", cfg.URL).Str("stream", cfg.StreamName).Msg("nats bus initialized")
	return bus, nil
}

// ensureStream creates the stream if it doesn't exist
func (nb *NATSBus) ensureStream() error {
	_, err := nb.js.StreamInfo(nb.streamName)
	if err == nil {
		return nil // Stream exists
	}

	_, err = nb.js.AddStream(&nats.StreamConfig{
		Name:     nb.streamName,
		Subjects: []string{nb.streamName + ".*"},
		MaxMsgs:  10000,
		MaxAge:   7 * 24 * time.Hour,
		Storage:  nats.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	log.Info().Str("stream", nb.streamName).Msg("nats stream created")
	return nil
}

// Publish sends an event to NATS
func (nb *NATSBus) Publish(eventType string, payload map[string]interface{}) error {
	if nb.conn == nil {
		// In-memory fallback
		log.Debug().Str("type", eventType).Interface("payload", payload).Msg("in-memory event")
		return nil
	}

	event := Event{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Type:      eventType,
		Source:    "hydra-server",
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	subject := fmt.Sprintf("%s.%s", nb.streamName, eventType)
	_, err = nb.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	nb.logger.NATSEvent("published", subject, len(data))
	return nil
}

// Subscribe sets up a subscription for event types
func (nb *NATSBus) Subscribe(eventType string, handler func(Event) error) error {
	if nb.conn == nil {
		log.Info().Str("type", eventType).Msg("nats subscription skipped (disabled)")
		return nil
	}

	subject := fmt.Sprintf("%s.%s", nb.streamName, eventType)

	_, err := nb.js.Subscribe(subject, func(msg *nats.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal nats message")
			msg.Nak()
			return
		}

		nb.logger.NATSEvent("received", subject, len(msg.Data))

		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_id", event.ID).Msg("event handler failed")
			msg.Nak()
			return
		}

		msg.Ack()
	}, nats.Durable(fmt.Sprintf("hydra-%s-consumer", eventType)),
		nats.ManualAck(),
		nats.MaxDeliver(3),
	)

	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Info().Str("subject", subject).Msg("nats subscription established")
	return nil
}

// PublishAlert sends an alert event
func (nb *NATSBus) PublishAlert(alertID, alertType, severity, message string, value, threshold float64) error {
	return nb.Publish("alert", map[string]interface{}{
		"alert_id":  alertID,
		"type":      alertType,
		"severity":  severity,
		"message":   message,
		"value":     value,
		"threshold": threshold,
	})
}

// PublishRender sends a render completion event
func (nb *NATSBus) PublishRender(jobID, status string, durationMs int, width, height int) error {
	return nb.Publish("render", map[string]interface{}{
		"job_id":      jobID,
		"status":      status,
		"duration_ms": durationMs,
		"width":       width,
		"height":      height,
	})
}

// PublishMemorial sends a memorial site event
func (nb *NATSBus) PublishMemorial(eventType, userID, action string, metadata map[string]interface{}) error {
	payload := map[string]interface{}{
		"event_type": eventType,
		"user_id":    userID,
		"action":     action,
	}
	for k, v := range metadata {
		payload[k] = v
	}
	return nb.Publish("memorial", payload)
}

// Close closes the NATS connection
func (nb *NATSBus) Close() {
	if nb.conn != nil {
		nb.conn.Close()
	}
}
